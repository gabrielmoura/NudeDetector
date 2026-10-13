package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/translation"
)

func (h *Handler) handleTranslation(w http.ResponseWriter, r *http.Request) {
	if h.translator == nil {
		writeErr(w, http.StatusServiceUnavailable, "translation_unavailable", "translation model is unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.cfg.TranslationMaxCharacters)*4+1024)
	var request struct {
		Text           string `json:"text"`
		SourceLanguage string `json:"source_language"`
		TargetLanguage string `json:"target_language"`
		Model          string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if request.Model != "" {
		model, found := h.models.Get(request.Model)
		if !found || !h.models.Enabled(request.Model) || !supportsTranslation(model.Capabilities) {
			writeErr(w, http.StatusBadRequest, "model_unavailable", "requested translation model is not enabled")
			return
		}
	}
	if len([]rune(request.Text)) > h.cfg.TranslationMaxCharacters {
		writeErr(w, http.StatusRequestEntityTooLarge, "text_too_large", "translation text exceeds the configured limit")
		return
	}
	if strings.TrimSpace(request.Text) == "" || request.SourceLanguage == "" || request.TargetLanguage == "" {
		writeErr(w, http.StatusBadRequest, "invalid_request", "text, source_language, and target_language are required")
		return
	}
	result, err := h.translator.Translate(r.Context(), translation.Request{Text: request.Text, SourceLanguage: request.SourceLanguage, TargetLanguage: request.TargetLanguage})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "translation_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func supportsTranslation(capabilities []aimodel.Capability) bool {
	for _, capability := range capabilities {
		if capability == aimodel.CapabilityTranslation {
			return true
		}
	}
	return false
}
