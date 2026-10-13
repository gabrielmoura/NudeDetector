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
		Text           string   `json:"text"`
		Texts          []string `json:"texts"`
		SourceLanguage string   `json:"source_language"`
		TargetLanguage string   `json:"target_language"`
		Model          string   `json:"model"`
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
	if (request.Text == "" && len(request.Texts) == 0) || (request.Text != "" && len(request.Texts) != 0) || request.SourceLanguage == "" || request.TargetLanguage == "" {
		writeErr(w, http.StatusBadRequest, "invalid_request", "exactly one of text or texts plus source_language and target_language is required")
		return
	}
	texts := request.Texts
	if request.Text != "" {
		texts = []string{request.Text}
	}
	if len(texts) > h.cfg.TranslationMaxBatch {
		writeErr(w, http.StatusRequestEntityTooLarge, "batch_too_large", "translation batch exceeds the configured limit")
		return
	}
	results := make([]translation.Result, 0, len(texts))
	for _, text := range texts {
		if len([]rune(text)) > h.cfg.TranslationMaxCharacters {
			writeErr(w, http.StatusRequestEntityTooLarge, "text_too_large", "translation text exceeds the configured limit")
			return
		}
		if strings.TrimSpace(text) == "" {
			writeErr(w, http.StatusBadRequest, "invalid_request", "translation text must not be empty")
			return
		}
		result, err := h.translator.Translate(r.Context(), translation.Request{Text: text, SourceLanguage: request.SourceLanguage, TargetLanguage: request.TargetLanguage})
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "translation_failed", err.Error())
			return
		}
		results = append(results, result)
	}
	if request.Text != "" {
		writeJSON(w, http.StatusOK, results[0])
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"translations": results})
}

func supportsTranslation(capabilities []aimodel.Capability) bool {
	for _, capability := range capabilities {
		if capability == aimodel.CapabilityTranslation {
			return true
		}
	}
	return false
}
