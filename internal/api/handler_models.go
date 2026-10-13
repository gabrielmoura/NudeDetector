package api

import "net/http"

func (h *Handler) modelsList(w http.ResponseWriter, _ *http.Request) {
	if h.models == nil {
		writeErr(w, http.StatusServiceUnavailable, "models_unavailable", "model registry is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, h.models.List())
}

func (h *Handler) modelInfo(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeErr(w, http.StatusServiceUnavailable, "models_unavailable", "model registry is unavailable")
		return
	}
	model, ok := h.models.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "model_not_found", "model is not registered")
		return
	}
	writeJSON(w, http.StatusOK, model)
}
