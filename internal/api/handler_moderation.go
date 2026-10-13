package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

var errDetectorUnavailable = errors.New("visual details detector is unavailable")
var errClassifierUnavailable = errors.New("NSFW classifier is unavailable")

func (h *Handler) Classify(ctx context.Context, data []byte) (*moderation.ModerationResult, error) {
	if h.engine == nil || h.worker == nil || h.proc == nil || h.classifier == nil {
		return nil, errClassifierUnavailable
	}
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
	observability.InferenceInFlight.Inc()
	observability.InferenceQueueDepth.Set(float64(h.worker.QueueDepth()))
	logits, err := h.worker.Infer(ctx, tensor)
	observability.InferenceInFlight.Dec()
	observability.InferenceQueueDepth.Set(float64(h.worker.QueueDepth()))
	if err != nil {
		return nil, err
	}
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

	if _, found := r.URL.Query()["details"]; found {
		writeErr(w, http.StatusBadRequest, "invalid_request", "details must be provided in the request body")
		return
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		details, err := parseDetails(r.FormValue("details"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid_request", "details must be boolean")
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
		response, err := h.classifyResponse(r.Context(), data, details)
		if err != nil {
			writeModerationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response)
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
		response, err := h.classifyResponse(r.Context(), data, req.Details)
		if err != nil {
			writeModerationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response)
		observability.RequestsTotal.WithLabelValues("POST", "/v1/moderation/pornography", strconv.Itoa(http.StatusOK)).Inc()
		return
	}

	writeErr(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "unsupported")
}

func (h *Handler) classifyResponse(
	ctx context.Context,
	data []byte,
	details bool,
) (ModerationResponse, error) {
	result, err := h.Classify(ctx, data)
	if err != nil {
		return ModerationResponse{}, err
	}
	if !details {
		return newModerationResponse(result, false, nil), nil
	}
	if h.detector == nil {
		return ModerationResponse{}, errDetectorUnavailable
	}
	detections, err := h.detector.Detect(ctx, data)
	if err != nil {
		return ModerationResponse{}, fmt.Errorf("detect visual details: %w", err)
	}
	return newModerationResponse(result, true, detections), nil
}

func writeModerationError(w http.ResponseWriter, err error) {
	if errors.Is(err, errClassifierUnavailable) {
		writeErr(w, http.StatusServiceUnavailable, "classifier_unavailable", err.Error())
		return
	}
	if errors.Is(err, errDetectorUnavailable) {
		writeErr(w, http.StatusServiceUnavailable, "detector_unavailable", err.Error())
		return
	}
	writeErr(w, http.StatusInternalServerError, "inference_failed", err.Error())
}

func parseDetails(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
