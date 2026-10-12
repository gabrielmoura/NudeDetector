package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/api"
	"github.com/gabrielmoura/NudeDetector/internal/cache"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/detector"
	"github.com/gabrielmoura/NudeDetector/internal/downloader"
	"github.com/gabrielmoura/NudeDetector/internal/imageprocessor"
	"github.com/gabrielmoura/NudeDetector/internal/inference"
	"github.com/gabrielmoura/NudeDetector/internal/model"
	"github.com/gabrielmoura/NudeDetector/internal/modelstore"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
	"github.com/gabrielmoura/NudeDetector/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	ort "github.com/yalue/onnxruntime_go"
)

type runtime struct {
	handler  *api.Handler
	engine   inference.Engine
	detector *detector.Detector
	httpAddr string
}

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "nude-detector",
		Short: "Classifies pornography probability from images",
		RunE: func(command *cobra.Command, arguments []string) error {
			runtime, err := newRuntime()
			if err != nil {
				return err
			}
			defer runtime.Close()
			return serve(runtime.handler.Router(), runtime.httpAddr)
		},
	}
	root.AddCommand(newServeCommand(), newClassifyCommand(), newExportCommand())
	return root
}

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Starts the HTTP API",
		RunE: func(command *cobra.Command, arguments []string) error {
			runtime, err := newRuntime()
			if err != nil {
				return err
			}
			defer runtime.Close()
			return serve(runtime.handler.Router(), runtime.httpAddr)
		},
	}
}

func newClassifyCommand() *cobra.Command {
	var inputPath string
	command := &cobra.Command{
		Use:   "classify",
		Short: "Classifies a local image and writes JSON to stdout",
		RunE: func(command *cobra.Command, arguments []string) error {
			if inputPath == "" {
				return fmt.Errorf("--input is required")
			}
			data, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}
			runtime, err := newRuntime()
			if err != nil {
				return err
			}
			defer runtime.Close()
			result, err := runtime.handler.Classify(command.Context(), data)
			if err != nil {
				return err
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(result)
		},
	}
	command.Flags().StringVarP(&inputPath, "input", "i", "", "path to a local image")
	return command
}

func newExportCommand() *cobra.Command {
	var inputPath, outputPath string
	var drawBoxes, blur bool
	var blurRadius float64
	var frameTimeMS int64
	command := &cobra.Command{
		Use:   "export",
		Short: "Exports a JPEG with NudeNet boxes and optional blur",
		RunE: func(command *cobra.Command, arguments []string) error {
			if inputPath == "" || outputPath == "" {
				return fmt.Errorf("--input and --output are required")
			}
			data, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}
			runtime, err := newRuntime()
			if err != nil {
				return err
			}
			defer runtime.Close()
			result, _, err := runtime.handler.Export(
				command.Context(),
				data,
				detectContentType(inputPath, data),
				api.ExportOptions{
					DrawBoxes:   drawBoxes,
					Blur:        blur,
					BlurRadius:  blurRadius,
					FrameTimeMS: frameTimeMS,
				},
			)
			if err != nil {
				return err
			}
			return os.WriteFile(outputPath, result, 0o644)
		},
	}
	command.Flags().StringVarP(&inputPath, "input", "i", "", "path to an image, GIF, or video")
	command.Flags().StringVarP(&outputPath, "output", "o", "", "destination JPEG path")
	command.Flags().BoolVar(&drawBoxes, "draw-boxes", true, "draw labels and detection boxes")
	command.Flags().BoolVar(&blur, "blur", false, "blur detected regions")
	command.Flags().Float64Var(&blurRadius, "blur-radius", 25, "blur radius between 1 and 100")
	command.Flags().Int64Var(&frameTimeMS, "frame-time-ms", 0, "GIF or video frame time in milliseconds")
	return command
}

func newRuntime() (*runtime, error) {
	log := observability.Setup()
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	ort.SetSharedLibraryPath(cfg.ONNXRuntimeLibraryPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("initialize ONNX Runtime: %w", err)
	}
	if err := modelstore.Ensure(cfg.ClassifierModelPath, cfg.ClassifierModelURL, cfg.ClassifierModelSHA256); err != nil {
		ort.DestroyEnvironment()
		return nil, err
	}
	metadata, err := model.LoadMetadata(cfg.ClassifierMetadataPath)
	if err != nil {
		ort.DestroyEnvironment()
		return nil, fmt.Errorf("load classifier metadata: %w", err)
	}
	engine, err := inference.NewONNX(cfg.ClassifierModelPath, cfg.ONNXIntraOpThreads, cfg.ONNXInterOpThreads)
	if err != nil {
		ort.DestroyEnvironment()
		return nil, err
	}
	processor := imageprocessor.New(metadata.Width, metadata.Height)
	classifier, err := moderation.NewClassifier(metadata)
	if err != nil {
		engine.Close()
		ort.DestroyEnvironment()
		return nil, err
	}
	policy := moderation.NewPolicy(cfg.PornReviewThreshold, cfg.PornThreshold)
	fetcher := downloader.NewSafeFetcher(cfg.DialTimeout, cfg.TLSHandshakeTimeout, cfg.ResponseHeaderTimeout, cfg.DownloadTimeout, cfg.MaxRedirects, cfg.MaxImageBytes)
	var resultCache cache.Cache
	if cfg.CacheEnabled {
		resultCache = cache.NewMemory(cfg.CacheMaxEntries, cfg.CacheTTL)
	}
	worker := inference.NewWorker(engine, cfg.InferenceConcurrency, cfg.MaxPendingInferences)
	var bodyDetector *detector.Detector
	if cfg.DetectorEnabled {
		if err := modelstore.Ensure(cfg.DetectorModelPath, cfg.DetectorModelURL, cfg.DetectorModelSHA256); err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, err
		}
		bodyDetector, err = detector.New(cfg.DetectorModelPath, float32(cfg.DetectorScoreMinimum), float32(cfg.DetectorIoUThreshold))
		if err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, err
		}
	}
	observability.ModelLoaded = true
	observability.ReadyFlag = true
	observability.ModelInfo.WithLabelValues(metadata.Name, metadata.Version).Set(1)
	registry := prometheus.NewRegistry()
	observability.Register(registry)
	return &runtime{
		handler:  api.New(cfg, log, metadata, engine, processor, classifier, policy, fetcher, resultCache, worker, bodyDetector),
		engine:   engine,
		detector: bodyDetector,
		httpAddr: cfg.HTTPAddr,
	}, nil
}

func (r *runtime) Close() {
	if r.detector != nil {
		_ = r.detector.Close()
	}
	_ = r.engine.Close()
	_ = ort.DestroyEnvironment()
}

func serve(handler http.Handler, addr string) error {
	server := &http.Server{Addr: addr, Handler: handler, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		context, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(context)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func detectContentType(path string, data []byte) string {
	if extension := filepath.Ext(path); extension == ".gif" {
		return "image/gif"
	}
	return http.DetectContentType(data)
}
