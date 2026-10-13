package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
)

func TestModelsEndpointsExposeCatalogAndUnknownModel(t *testing.T) {
	t.Parallel()

	registry, err := aimodel.New("missing-classifier.onnx", "missing-detector.onnx", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := &Handler{models: registry}

	listRequest := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	listResponse := httptest.NewRecorder()
	handler.Router().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /v1/models status = %d, want %d", listResponse.Code, http.StatusOK)
	}

	unknownRequest := httptest.NewRequest(http.MethodGet, "/v1/models/unknown", nil)
	unknownResponse := httptest.NewRecorder()
	handler.Router().ServeHTTP(unknownResponse, unknownRequest)
	if unknownResponse.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/models/unknown status = %d, want %d", unknownResponse.Code, http.StatusNotFound)
	}
}
