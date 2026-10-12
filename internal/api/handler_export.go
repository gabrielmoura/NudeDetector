package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gabrielmoura/NudeDetector/internal/exporter"
	"github.com/gabrielmoura/NudeDetector/internal/media"
)

type ExportOptions struct {
	DrawBoxes   bool
	Blur        bool
	BlurRadius  float64
	FrameTimeMS int64
}

func (h *Handler) Export(
	ctx context.Context,
	data []byte,
	contentType string,
	options ExportOptions,
) ([]byte, int, error) {
	if h.detector == nil {
		return nil, 0, errors.New("export detector is disabled")
	}
	frame, err := media.ImageFrame(ctx, data, contentType, options.FrameTimeMS)
	if err != nil {
		return nil, 0, err
	}
	detections, err := h.detector.Detect(ctx, frame)
	if err != nil {
		return nil, 0, err
	}
	result, err := exporter.Render(frame, detections, exporter.Options{
		DrawBoxes:  options.DrawBoxes,
		Blur:       options.Blur,
		BlurRadius: options.BlurRadius,
	})
	if err != nil {
		return nil, 0, err
	}
	return result, len(detections), nil
}

func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
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
		writeErr(w, http.StatusBadRequest, "invalid_media", err.Error())
		return
	}
	if int64(len(data)) > h.cfg.MaxImageBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "media_too_large", "media exceeds limit")
		return
	}
	frameTimeMS, err := optionalInt64(r.FormValue("frame_time_ms"), 0)
	if err != nil || frameTimeMS < 0 {
		writeErr(w, http.StatusBadRequest, "invalid_request", "frame_time_ms must be a non-negative integer")
		return
	}
	drawBoxes, err := optionalBool(r.FormValue("draw_boxes"), true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "draw_boxes must be boolean")
		return
	}
	blur, err := optionalBool(r.FormValue("blur"), false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "blur must be boolean")
		return
	}
	blurRadius, err := optionalFloat64(r.FormValue("blur_radius"), 25)
	if err != nil || blurRadius < 1 || blurRadius > 100 {
		writeErr(w, http.StatusBadRequest, "invalid_request", "blur_radius must be between 1 and 100")
		return
	}
	result, count, err := h.Export(r.Context(), data, http.DetectContentType(data), ExportOptions{
		DrawBoxes:   drawBoxes,
		Blur:        blur,
		BlurRadius:  blurRadius,
		FrameTimeMS: frameTimeMS,
	})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "export_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Disposition", "inline; filename=moderated.jpg")
	w.Header().Set("X-NudeDetector-Detections", strconv.Itoa(count))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

func optionalBool(value string, fallback bool) (bool, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseBool(value)
}

func optionalInt64(value string, fallback int64) (int64, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func optionalFloat64(value string, fallback float64) (float64, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(value, 64)
}
