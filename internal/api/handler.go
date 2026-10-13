package api

import (
	"log/slog"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/asr"
	"github.com/gabrielmoura/NudeDetector/internal/cache"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/detector"
	"github.com/gabrielmoura/NudeDetector/internal/downloader"
	"github.com/gabrielmoura/NudeDetector/internal/imageprocessor"
	"github.com/gabrielmoura/NudeDetector/internal/inference"
	"github.com/gabrielmoura/NudeDetector/internal/model"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
	"github.com/gabrielmoura/NudeDetector/internal/ocr"
	"github.com/gabrielmoura/NudeDetector/internal/translation"
	"golang.org/x/sync/singleflight"
)

type Handler struct {
	cfg        *config.Config
	log        *slog.Logger
	meta       *model.ModelMetadata
	engine     inference.Engine
	proc       *imageprocessor.Processor
	classifier moderation.Classifier
	policy     moderation.Policy
	fetcher    *downloader.SafeFetcher
	cache      cache.Cache
	sg         *singleflight.Group
	worker     *inference.Worker
	detector   *detector.Detector
	models     *aimodel.Registry
	asr        asr.Transcriber
	ocr        ocr.Recognizer
	translator translation.Translator
}

func New(
	cfg *config.Config,
	log *slog.Logger,
	meta *model.ModelMetadata,
	engine inference.Engine,
	proc *imageprocessor.Processor,
	clf moderation.Classifier,
	pol moderation.Policy,
	fetcher *downloader.SafeFetcher,
	c cache.Cache,
	worker *inference.Worker,
	bodyDetector *detector.Detector,
	models *aimodel.Registry,
	transcriber asr.Transcriber,
	ocrRecognizer ocr.Recognizer,
	translator translation.Translator,
) *Handler {
	return &Handler{
		cfg:        cfg,
		log:        log,
		meta:       meta,
		engine:     engine,
		proc:       proc,
		classifier: clf,
		policy:     pol,
		fetcher:    fetcher,
		cache:      c,
		sg:         &singleflight.Group{},
		worker:     worker,
		detector:   bodyDetector,
		models:     models,
		asr:        transcriber,
		ocr:        ocrRecognizer,
		translator: translator,
	}
}
