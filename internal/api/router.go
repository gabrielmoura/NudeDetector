package api

import (
	"net/http"
)

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/moderation/pornography", h.handleModeration)
	mux.HandleFunc("POST /v1/moderation/export", h.handleExport)
	mux.HandleFunc("POST /v1/audio/transcriptions", h.handleTranscription)
	mux.HandleFunc("GET /v1/models", h.modelsList)
	mux.HandleFunc("GET /v1/models/{id}", h.modelInfo)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("GET /metrics", h.metrics)
	return mux
}
