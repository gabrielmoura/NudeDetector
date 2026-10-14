package api

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/observability"
	"github.com/gabrielmoura/NudeDetector/internal/ocr"
)

func (h *Handler) handleOCR(w http.ResponseWriter, r *http.Request) {
	if h.ocr == nil {
		writeErr(w, http.StatusServiceUnavailable, "ocr_unavailable", "OCR model is unavailable")
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "image must be multipart/form-data")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxImageBytes+1024*1024)
	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "invalid multipart image")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "missing image field")
		return
	}
	defer file.Close()
	if !requestedModelEnabled(h.models, r.FormValue("model"), "ocr") {
		writeErr(w, http.StatusBadRequest, "model_unavailable", "requested OCR model is not enabled")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, h.cfg.MaxImageBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_image", "read image")
		return
	}
	if int64(len(data)) > h.cfg.MaxImageBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds limit")
		return
	}
	const capability = "ocr"
	modelID := enabledModelID(h.models, capability)
	started := time.Now()
	observability.AIInFlight.WithLabelValues(capability, modelID).Inc()
	result, err := h.ocr.Recognize(r.Context(), data, ocr.Request{Language: r.FormValue("language"), ReturnBoxes: r.FormValue("return_boxes") == "true"})
	observability.AIInFlight.WithLabelValues(capability, modelID).Dec()
	if err != nil {
		observability.AIErrorsTotal.WithLabelValues(capability, "inference").Inc()
		writeErr(w, http.StatusUnprocessableEntity, "ocr_failed", err.Error())
		return
	}
	observability.AIRequestsTotal.WithLabelValues(capability, result.Model).Inc()
	observability.AIInferenceDuration.WithLabelValues(capability, result.Model).Observe(time.Since(started).Seconds())
	if r.FormValue("return_boxes") != "true" {
		result.Blocks = []ocr.Block{}
	}
	writeJSON(w, http.StatusOK, result)
}
