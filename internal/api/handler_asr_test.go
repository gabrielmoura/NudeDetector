package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/config"
)

func TestTranscriptionRejectsUnavailableService(t *testing.T) {
	t.Parallel()

	handler := &Handler{cfg: &config.Config{MaxAudioBytes: 1024}}
	request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	response := httptest.NewRecorder()

	handler.handleTranscription(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("handleTranscription() status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
