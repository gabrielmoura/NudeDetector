package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/translation"
)

type translationStub struct{}

func (translationStub) Translate(_ context.Context, request translation.Request) (translation.Result, error) {
	return translation.Result{SourceText: request.Text, TranslatedText: "Olá", SourceLanguage: "en", TargetLanguage: "pt", Model: "opus-mt-en-romance", Metadata: map[string]any{}}, nil
}
func (translationStub) Close() error { return nil }

func TestTranslationEndpointUsesSharedTranslator(t *testing.T) {
	registry, err := aimodel.New("classifier.onnx", "detector.onnx", []string{"opus-mt-en-romance"})
	if err != nil {
		t.Fatal(err)
	}
	handler := &Handler{cfg: &config.Config{TranslationMaxCharacters: 100}, models: registry, translator: translationStub{}}
	request := httptest.NewRequest(http.MethodPost, "/v1/translations", strings.NewReader(`{"text":"Hello","source_language":"en","target_language":"pt","model":"opus-mt-en-romance"}`))
	response := httptest.NewRecorder()
	handler.handleTranslation(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"translated_text":"Olá"`) {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestTranslationEndpointRejectsUnavailableService(t *testing.T) {
	handler := &Handler{cfg: &config.Config{TranslationMaxCharacters: 100}}
	request := httptest.NewRequest(http.MethodPost, "/v1/translations", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.handleTranslation(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
