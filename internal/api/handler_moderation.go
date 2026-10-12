package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/cache"
	"github.com/gabrielmoura/NudeDetector/internal/imageprocessor"
	"github.com/gabrielmoura/NudeDetector/internal/inference"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
	"github.com/gabrielmoura/NudeDetector/internal/observability"
	"github.com/gabrielmoura/NudeDetector/internal/util"
)

func (h *Handler) Classify(ctx context.Context, data []byte) (*moderation.ModerationResult, error) {
	hash := util.SHA256(data)
	if h.cache != nil {
		if e, ok := h.cache.Get(hash + "|v1|" + h.meta.Name + "|" + h.meta.Version); ok {
			s := moderation.ClassificationScores{Drawings: e.Drawings, Hentai: e.Hentai, Neutral: e.Neutral, Porn: e.Porn, Sexy: e.Sexy}
			cls := h.policy.Evaluate(s)
			observability.CacheHits.Inc()
			observability.ClassificationTotal.WithLabelValues(cls).Inc()
			return &moderation.ModerationResult{
				PornographyScore: s.Porn, PornographyPercent: s.Porn * 100,
				Classification: cls, Scores: s,
				Thresholds: moderation.Thresholds{Review: h.cfg.PornReviewThreshold, Pornographic: h.cfg.PornThreshold},
				Model:      moderation.ModelInfo{Name: h.meta.Name, Version: h.meta.Version},
				Processing: moderation.ProcessingInfo{Cached: true},
			}, nil
		}
		observability.CacheMisses.Inc()
	}
	_, _, err := imageprocessor.DecodeConfig(data)
	if err != nil {
		return nil, err
	}
	tensor, err := h.proc.Process(data)
	if err != nil {
		return nil, err
	}
	logits, err := h.worker.Infer(ctx, tensor)
	if err != nil {
		return nil, err
	}
	observability.InferenceInFlight.Dec()
	probs := inference.StableSoftmax(logits)
	sc, err := h.classifier.Classify(probs)
	if err != nil {
		return nil, err
	}
	cls := h.policy.Evaluate(*sc)
	observability.ClassificationTotal.WithLabelValues(cls).Inc()
	if h.cache != nil {
		h.cache.Set(hash+"|v1|"+h.meta.Name+"|"+h.meta.Version, &cache.Entry{Drawings: sc.Drawings, Hentai: sc.Hentai, Neutral: sc.Neutral, Porn: sc.Porn, Sexy: sc.Sexy})
	}
	return &moderation.ModerationResult{
		PornographyScore: sc.Porn, PornographyPercent: sc.Porn * 100,
		Classification: cls, Scores: *sc,
		Thresholds: moderation.Thresholds{Review: h.cfg.PornReviewThreshold, Pornographic: h.cfg.PornThreshold},
		Model:      moderation.ModelInfo{Name: h.meta.Name, Version: h.meta.Version},
		Processing: moderation.ProcessingInfo{Cached: false},
	}, nil
}

func (h *Handler) handleModeration(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	observability.RequestsInFlight.Inc()
	defer observability.RequestsInFlight.Dec()
	defer func() {
		observability.RequestDuration.WithLabelValues("/v1/moderation/pornography").Observe(
			time.Since(start).Seconds(),
		)
	}()

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_request", err.Error())
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
			writeErr(w, http.StatusBadRequest, "invalid_image", err.Error())
			return
		}
		if int64(len(data)) > h.cfg.MaxImageBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds limit")
			return
		}
		wc, hc, err := imageprocessor.DecodeConfig(data)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_image", err.Error())
			return
		}
		if int64(wc*hc) > h.cfg.MaxImagePixels || wc > h.cfg.MaxWidth || hc > h.cfg.MaxHeight {
			writeErr(w, http.StatusUnprocessableEntity, "image_too_many_pixels", "image too large")
			return
		}
		res, err := h.Classify(r.Context(), data)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "inference_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
		observability.RequestsTotal.WithLabelValues("POST", "/v1/moderation/pornography", strconv.Itoa(http.StatusOK)).Inc()
		return
	}

	if strings.Contains(ct, "application/json") || ct == "" {
		var req URLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if req.URL == "" {
			writeErr(w, http.StatusBadRequest, "invalid_url", "url required")
			return
		}
		data, err := h.fetcher.Fetch(r.Context(), req.URL)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "download_failed", err.Error())
			return
		}
		if int64(len(data)) > h.cfg.MaxImageBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds limit")
			return
		}
		wc, hc, err := imageprocessor.DecodeConfig(data)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_image", err.Error())
			return
		}
		if int64(wc*hc) > h.cfg.MaxImagePixels || wc > h.cfg.MaxWidth || hc > h.cfg.MaxHeight {
			writeErr(w, http.StatusUnprocessableEntity, "image_too_many_pixels", "image too large")
			return
		}
		res, err := h.Classify(r.Context(), data)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "inference_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
		observability.RequestsTotal.WithLabelValues("POST", "/v1/moderation/pornography", strconv.Itoa(http.StatusOK)).Inc()
		return
	}

	writeErr(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "unsupported")
}
