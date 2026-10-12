package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/api"
	"github.com/gabrielmoura/NudeDetector/internal/cache"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/downloader"
	"github.com/gabrielmoura/NudeDetector/internal/imageprocessor"
	"github.com/gabrielmoura/NudeDetector/internal/inference"
	"github.com/gabrielmoura/NudeDetector/internal/model"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
	"github.com/gabrielmoura/NudeDetector/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	ort "github.com/yalue/onnxruntime_go"
)

func main() {
	log := observability.Setup()
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ort.SetSharedLibraryPath(cfg.ONNXRuntimeLibraryPath)
	if err := ort.InitializeEnvironment(); err != nil {
		log.Error(
			"initialize onnx runtime",
			"path", cfg.ONNXRuntimeLibraryPath,
			"err", err,
		)
		os.Exit(1)
	}
	defer ort.DestroyEnvironment()

	meta, err := model.LoadMetadata(cfg.ClassifierMetadataPath)
	if err != nil {
		meta = &model.ModelMetadata{Name: "nsfw-classifier", Version: "1.0.0", Labels: []string{"drawings", "hentai", "neutral", "porn", "sexy"}, Width: 224, Height: 224}
		log.Warn("metadata missing, using defaults", "err", err)
	}
	if meta.Width == 0 && meta.Height == 0 {
		meta.Width, meta.Height = 224, 224
	}

	engine, err := inference.NewONNX(cfg.ClassifierModelPath, cfg.ONNXIntraOpThreads, cfg.ONNXInterOpThreads)
	if err != nil {
		log.Error("onnx load", "err", err)
		os.Exit(1)
	}

	proc := imageprocessor.New(meta.Width, meta.Height)
	clf, _ := moderation.NewClassifier(meta)
	pol := moderation.NewPolicy(cfg.PornReviewThreshold, cfg.PornThreshold)
	fetcher := downloader.NewSafeFetcher(cfg.DialTimeout, cfg.TLSHandshakeTimeout, cfg.ResponseHeaderTimeout, cfg.DownloadTimeout, cfg.MaxRedirects, cfg.MaxImageBytes)
	var c cache.Cache
	if cfg.CacheEnabled {
		c = cache.NewMemory(cfg.CacheMaxEntries, cfg.CacheTTL)
	}
	worker := inference.NewWorker(engine, cfg.InferenceConcurrency, cfg.MaxPendingInferences)

	observability.ModelLoaded = true
	observability.ReadyFlag = true
	observability.ModelInfo.WithLabelValues(meta.Name, meta.Version).Set(1)
	reg := prometheus.NewRegistry()
	observability.Register(reg)

	h := api.New(cfg, log, meta, engine, proc, clf, pol, fetcher, c, worker)
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: h.Router(), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}

	go func() {
		log.Info("starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	engine.Close()
}
