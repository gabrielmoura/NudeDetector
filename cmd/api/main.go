package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/api"
	"github.com/gabrielmoura/NudeDetector/internal/buildinfo"
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
	"github.com/gabrielmoura/NudeDetector/internal/runtimepath"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	ort "github.com/yalue/onnxruntime_go"
)

type runtime struct {
	handler        *api.Handler
	engine         inference.Engine
	worker         *inference.Worker
	detector       *detector.Detector
	log            *slog.Logger
	metadata       *model.ModelMetadata
	modelPath      string
	runtimeLibrary string
	httpAddr       string
	models         *aimodel.Registry
}

type runtimeOptions struct {
	HTTPAddr       string
	RuntimeLibrary string
	Models         string
}

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		observability.Setup().Error("command_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var options runtimeOptions
	root := &cobra.Command{
		Use:           "nude-detector",
		Short:         "Classifies pornography probability from images",
		Version:       buildinfo.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, arguments []string) error {
			runtime, err := newRuntime(options, false)
			if err != nil {
				return err
			}
			defer runtime.Close()
			return serve(runtime)
		},
	}
	root.PersistentFlags().StringVar(&options.HTTPAddr, "addr", "", "HTTP listen address (overrides HTTP_ADDR)")
	root.PersistentFlags().StringVar(&options.RuntimeLibrary, "runtime-library", "", "path to the ONNX Runtime shared library")
	root.AddCommand(
		newServeCommand(&options),
		newClassifyCommand(&options),
		newExportCommand(&options),
		newScanCommand(&options),
		newVersionCommand(),
	)
	return root
}

func newServeCommand(options *runtimeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Starts the HTTP API",
		RunE: func(command *cobra.Command, arguments []string) error {
			runtime, err := newRuntime(*options, false)
			if err != nil {
				return err
			}
			defer runtime.Close()
			return serve(runtime)
		},
	}
	command.Flags().StringVar(&options.Models, "models", "", "comma-separated model IDs to load (overrides ENABLED_MODELS)")
	return command
}

func newClassifyCommand(options *runtimeOptions) *cobra.Command {
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
			runtime, err := newRuntime(*options, false)
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

func newExportCommand(options *runtimeOptions) *cobra.Command {
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
			runtime, err := newRuntime(*options, true)
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

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints build and runtime version information",
		Run: func(command *cobra.Command, arguments []string) {
			info := buildinfo.Current()
			infoJSON, _ := json.Marshal(info)
			_, _ = command.OutOrStdout().Write(append(infoJSON, '\n'))
		},
	}
}

func newRuntime(options runtimeOptions, loadDetector bool) (*runtime, error) {
	log := observability.Setup()
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if options.HTTPAddr != "" {
		cfg.HTTPAddr = options.HTTPAddr
	}
	if options.RuntimeLibrary != "" {
		cfg.ONNXRuntimeLibraryPath = options.RuntimeLibrary
	}
	if options.Models != "" {
		enabledModels, parseErr := aimodel.ParseEnabled(options.Models)
		if parseErr != nil {
			return nil, fmt.Errorf("parse --models: %w", parseErr)
		}
		cfg.EnabledModels = enabledModels
	}
	enabledModels := cfg.EnabledModels
	if loadDetector && cfg.DetectorEnabled && !containsModel(enabledModels, aimodel.NudeNetDetectorID) {
		enabledModels = append(enabledModels, aimodel.NudeNetDetectorID)
	}
	models, err := aimodel.New(cfg.ClassifierModelPath, cfg.DetectorModelPath, enabledModels)
	if err != nil {
		return nil, err
	}
	if err := validateImplementedModels(models); err != nil {
		return nil, err
	}
	executable, executableErr := os.Executable()
	if executableErr != nil {
		executable = ""
	}
	workingDirectory, workingDirectoryErr := os.Getwd()
	if workingDirectoryErr != nil {
		return nil, fmt.Errorf("resolve working directory: %w", workingDirectoryErr)
	}
	runtimeLibrary, err := runtimepath.Resolve(cfg.ONNXRuntimeLibraryPath, executable, workingDirectory)
	if err != nil {
		return nil, err
	}
	cfg.ONNXRuntimeLibraryPath = runtimeLibrary
	log.Info("runtime_initializing",
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.String("build_time", buildinfo.BuildTime),
		slog.String("onnx_runtime_library", runtimeLibrary),
	)
	ort.SetSharedLibraryPath(runtimeLibrary)
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("initialize ONNX Runtime: %w", err)
	}
	var metadata *model.ModelMetadata
	var engine inference.Engine
	var processor *imageprocessor.Processor
	var classifier moderation.Classifier
	var worker *inference.Worker
	if models.Enabled(aimodel.NSFWClassifierID) {
		if err := modelstore.Ensure(cfg.ClassifierModelPath, cfg.ClassifierModelURL, cfg.ClassifierModelSHA256); err != nil {
			ort.DestroyEnvironment()
			return nil, err
		}
		metadata, err = model.LoadMetadata(cfg.ClassifierMetadataPath)
		if err != nil {
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("load classifier metadata: %w", err)
		}
		engine, err = inference.NewONNX(cfg.ClassifierModelPath, cfg.ONNXIntraOpThreads, cfg.ONNXInterOpThreads)
		if err != nil {
			ort.DestroyEnvironment()
			return nil, err
		}
		processor, err = imageprocessor.New(metadata)
		if err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("initialize image processor: %w", err)
		}
		classifier, err = moderation.NewClassifier(metadata)
		if err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, err
		}
		if err := warmupClassifier(engine, classifier, metadata); err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, err
		}
		worker = inference.NewWorker(engine, cfg.InferenceConcurrency, cfg.MaxPendingInferences)
		if err := models.MarkLoaded(aimodel.NSFWClassifierID); err != nil {
			engine.Close()
			ort.DestroyEnvironment()
			return nil, err
		}
	}
	policy := moderation.NewPolicy(cfg.PornReviewThreshold, cfg.PornThreshold)
	fetcher := downloader.NewSafeFetcher(cfg.DialTimeout, cfg.TLSHandshakeTimeout, cfg.ResponseHeaderTimeout, cfg.DownloadTimeout, cfg.MaxRedirects, cfg.MaxImageBytes)
	var resultCache cache.Cache
	if cfg.CacheEnabled {
		resultCache = cache.NewMemory(cfg.CacheMaxEntries, cfg.CacheTTL)
	}
	var bodyDetector *detector.Detector
	if models.Enabled(aimodel.NudeNetDetectorID) {
		if !cfg.DetectorEnabled {
			if engine != nil {
				engine.Close()
			}
			ort.DestroyEnvironment()
			return nil, fmt.Errorf("nudenet-detector is selected but DETECTOR_ENABLED is false")
		}
		if err := modelstore.Ensure(cfg.DetectorModelPath, cfg.DetectorModelURL, cfg.DetectorModelSHA256); err != nil {
			if engine != nil {
				engine.Close()
			}
			ort.DestroyEnvironment()
			return nil, err
		}
		bodyDetector, err = detector.New(cfg.DetectorModelPath, float32(cfg.DetectorScoreMinimum), float32(cfg.DetectorIoUThreshold))
		if err != nil {
			if engine != nil {
				engine.Close()
			}
			ort.DestroyEnvironment()
			return nil, err
		}
		if err := models.MarkLoaded(aimodel.NudeNetDetectorID); err != nil {
			_ = bodyDetector.Close()
			if engine != nil {
				engine.Close()
			}
			ort.DestroyEnvironment()
			return nil, err
		}
	}
	observability.ModelLoaded = metadata != nil || bodyDetector != nil
	observability.ReadyFlag = true
	if metadata != nil {
		observability.ModelInfo.WithLabelValues(metadata.Name, metadata.Version).Set(1)
	}
	observability.Register(prometheus.DefaultRegisterer)
	return &runtime{
		handler:        api.New(cfg, log, metadata, engine, processor, classifier, policy, fetcher, resultCache, worker, bodyDetector, models),
		engine:         engine,
		worker:         worker,
		detector:       bodyDetector,
		log:            log,
		metadata:       metadata,
		modelPath:      cfg.ClassifierModelPath,
		runtimeLibrary: runtimeLibrary,
		httpAddr:       cfg.HTTPAddr,
		models:         models,
	}, nil
}

func validateImplementedModels(models *aimodel.Registry) error {
	for _, id := range models.EnabledIDs() {
		switch id {
		case aimodel.NSFWClassifierID, aimodel.NudeNetDetectorID:
		default:
			return fmt.Errorf("model %q is registered but its service is not implemented yet", id)
		}
	}
	return nil
}

func containsModel(models []string, wanted string) bool {
	for _, id := range models {
		if id == wanted {
			return true
		}
	}
	return false
}

func warmupClassifier(
	engine inference.Engine,
	classifier moderation.Classifier,
	metadata *model.ModelMetadata,
) error {
	shape := engine.InputShape()
	if len(shape) != 4 || shape[0] != 1 || shape[1] != 3 ||
		shape[2] != int64(metadata.Height) || shape[3] != int64(metadata.Width) {
		return fmt.Errorf("classifier input shape %v is incompatible with metadata %dx%d NCHW", shape, metadata.Width, metadata.Height)
	}

	size := int(shape[0] * shape[1] * shape[2] * shape[3])
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	logits, err := engine.Infer(ctx, make([]float32, size))
	if err != nil {
		return fmt.Errorf("classifier warm-up: %w", err)
	}
	if len(logits) != len(metadata.Labels) {
		return fmt.Errorf("classifier warm-up returned %d classes, metadata defines %d labels", len(logits), len(metadata.Labels))
	}
	if _, err := classifier.Classify(inference.StableSoftmax(logits)); err != nil {
		return fmt.Errorf("validate classifier warm-up output: %w", err)
	}
	return nil
}

func (r *runtime) Close() {
	if r.worker != nil {
		r.worker.Close()
	}
	if r.detector != nil {
		_ = r.detector.Close()
	}
	if r.engine != nil {
		_ = r.engine.Close()
	}
	_ = ort.DestroyEnvironment()
}

func serve(runtime *runtime) error {
	listener, err := net.Listen("tcp", runtime.httpAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", runtime.httpAddr, err)
	}
	defer listener.Close()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	logAttributes := []slog.Attr{
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.String("build_time", buildinfo.BuildTime),
		slog.String("listen_address", listener.Addr().String()),
		slog.String("host", host),
		slog.String("port", port),
		slog.String("onnx_runtime_library", runtime.runtimeLibrary),
		slog.Bool("detector_enabled", runtime.detector != nil),
		slog.Any("enabled_models", runtime.models.EnabledIDs()),
	}
	if runtime.metadata != nil {
		logAttributes = append(logAttributes,
			slog.String("model_name", runtime.metadata.Name),
			slog.String("model_version", runtime.metadata.Version),
			slog.String("model_path", runtime.modelPath),
			slog.Any("model_labels", runtime.metadata.Labels),
		)
	}
	runtime.log.LogAttrs(context.Background(), slog.LevelInfo, "server_started", logAttributes...)
	server := &http.Server{Addr: runtime.httpAddr, Handler: runtime.handler.Router(), ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runtime.log.Info("server_stopping")
		_ = server.Shutdown(shutdownContext)
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	runtime.log.Info("server_stopped")
	return nil
}

func detectContentType(path string, data []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".gif":
		return "image/gif"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	}
	return http.DetectContentType(data)
}
