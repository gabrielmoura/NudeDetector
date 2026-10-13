package api

import (
	"io"
	"net/http"
	"strings"

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
	data, err := io.ReadAll(io.LimitReader(file, h.cfg.MaxImageBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_image", "read image")
		return
	}
	if int64(len(data)) > h.cfg.MaxImageBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds limit")
		return
	}
	result, err := h.ocr.Recognize(r.Context(), data, ocr.Request{Language: r.FormValue("language"), ReturnBoxes: r.FormValue("return_boxes") == "true"})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "ocr_failed", err.Error())
		return
	}
	if r.FormValue("return_boxes") != "true" {
		result.Blocks = []ocr.Block{}
	}
	writeJSON(w, http.StatusOK, result)
}
