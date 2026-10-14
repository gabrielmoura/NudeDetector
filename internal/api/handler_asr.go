package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/asr"
	"github.com/gabrielmoura/NudeDetector/internal/observability"
)

var errASRUnavailable = errors.New("ASR model is unavailable")

func (h *Handler) handleTranscription(w http.ResponseWriter, r *http.Request) {
	if h.asr == nil {
		writeErr(w, http.StatusServiceUnavailable, "asr_unavailable", errASRUnavailable.Error())
		return
	}
	if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "audio must be multipart/form-data")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxAudioBytes+1024*1024)
	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "invalid multipart audio")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "missing file field")
		return
	}
	defer file.Close()
	if !requestedModelEnabled(h.models, r.FormValue("model"), "asr") {
		writeErr(w, http.StatusBadRequest, "model_unavailable", "requested ASR model is not enabled")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, h.cfg.MaxAudioBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_audio", "read audio")
		return
	}
	if int64(len(data)) > h.cfg.MaxAudioBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "audio_too_large", "audio exceeds limit")
		return
	}
	audio, err := asr.DecodeAudio(r.Context(), data, h.cfg.ASRMaxDurationSeconds)
	if err != nil {
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported_audio", err.Error())
		return
	}
	timestamps, err := optionalBool(r.FormValue("timestamps"), false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "timestamps must be boolean")
		return
	}
	start := time.Now()
	const capability = "asr"
	modelID := enabledModelID(h.models, capability)
	observability.AIInFlight.WithLabelValues(capability, modelID).Inc()
	result, err := h.asr.Transcribe(r.Context(), audio, asr.Request{
		Language:   r.FormValue("language"),
		Timestamps: timestamps,
	})
	observability.AIInFlight.WithLabelValues(capability, modelID).Dec()
	if err != nil {
		observability.AIErrorsTotal.WithLabelValues(capability, "inference").Inc()
		if errors.Is(err, r.Context().Err()) {
			writeErr(w, http.StatusRequestTimeout, "transcription_cancelled", "transcription cancelled")
			return
		}
		writeErr(w, http.StatusInternalServerError, "transcription_failed", err.Error())
		return
	}
	observability.AIRequestsTotal.WithLabelValues(capability, result.Model).Inc()
	observability.AIInferenceDuration.WithLabelValues(capability, result.Model).Observe(time.Since(start).Seconds())
	if result.ProcessingTimeMS == 0 {
		result.ProcessingTimeMS = time.Since(start).Milliseconds()
	}
	format := r.FormValue("format")
	if format == "" || format == "json" {
		writeJSON(w, http.StatusOK, result)
		return
	}
	if format != "text" {
		writeErr(w, http.StatusBadRequest, "invalid_request", "format must be text or json")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(result.Text))
}
