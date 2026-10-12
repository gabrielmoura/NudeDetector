package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr string

	// Classifier
	ClassifierModelPath    string
	ClassifierMetadataPath string
	PornReviewThreshold    float64
	PornThreshold          float64
	ClassifyHentaiAsPorn   bool

	// Image limits
	MaxImageBytes  int64
	MaxImagePixels int64
	MaxWidth       int
	MaxHeight      int

	// Download
	DownloadTimeout       int64 // ms
	DialTimeout           int64 // ms
	TLSHandshakeTimeout   int64 // ms
	ResponseHeaderTimeout int64 // ms
	MaxRedirects          int

	// Inference
	InferenceConcurrency   int
	MaxPendingInferences   int
	ONNXIntraOpThreads     int
	ONNXInterOpThreads     int
	ONNXRuntimeLibraryPath string

	// Cache
	CacheEnabled    bool
	CacheMaxEntries int
	CacheTTL        int64 // ms

	// Explicit detector (optional)
	ExplicitDetectorEnabled      bool
	ExplicitDetectorModelPath    string
	ExplicitDetectorConcurrency  int
	ExplicitDetectorMinPornScore float64
	ExplicitDetectorMaxPornScore float64

	// Observability
	PPROFEnabled bool
}

func Load() (*Config, error) {
	c := &Config{
		HTTPAddr: getEnv("HTTP_ADDR", ":8080"),

		ClassifierModelPath:    getEnv("CLASSIFIER_MODEL_PATH", "./models/model.onnx"),
		ClassifierMetadataPath: getEnv("CLASSIFIER_METADATA_PATH", "./models/model.json"),
		PornReviewThreshold:    getEnvFloat("PORN_REVIEW_THRESHOLD", 0.40),
		PornThreshold:          getEnvFloat("PORN_THRESHOLD", 0.80),
		ClassifyHentaiAsPorn:   getEnvBool("CLASSIFY_HENTAI_AS_PORNOGRAPHY", false),

		MaxImageBytes:  getEnvInt64("MAX_IMAGE_BYTES", 10*1024*1024), // 10MB
		MaxImagePixels: getEnvInt64("MAX_IMAGE_PIXELS", 50_000_000),
		MaxWidth:       getEnvInt("MAX_WIDTH", 8192),
		MaxHeight:      getEnvInt("MAX_HEIGHT", 8192),

		DownloadTimeout:       getEnvInt64("DOWNLOAD_TIMEOUT", 30_000),
		DialTimeout:           getEnvInt64("DIAL_TIMEOUT", 5_000),
		TLSHandshakeTimeout:   getEnvInt64("TLS_HANDSHAKE_TIMEOUT", 10_000),
		ResponseHeaderTimeout: getEnvInt64("RESPONSE_HEADER_TIMEOUT", 30_000),
		MaxRedirects:          getEnvInt("MAX_REDIRECTS", 5),

		InferenceConcurrency: getEnvInt("INFERENCE_CONCURRENCY", 4),
		MaxPendingInferences: getEnvInt("MAX_PENDING_INFERENCES", 100),
		ONNXIntraOpThreads:   getEnvInt("ONNX_INTRA_OP_THREADS", 0),
		ONNXInterOpThreads:   getEnvInt("ONNX_INTER_OP_THREADS", 0),
		ONNXRuntimeLibraryPath: getEnv(
			"ONNXRUNTIME_SHARED_LIBRARY_PATH",
			"onnxruntime.so",
		),

		CacheEnabled:    getEnvBool("CACHE_ENABLED", true),
		CacheMaxEntries: getEnvInt("CACHE_MAX_ENTRIES", 1000),
		CacheTTL:        getEnvInt64("CACHE_TTL", 3600_000),

		ExplicitDetectorEnabled:      getEnvBool("EXPLICIT_DETECTOR_ENABLED", false),
		ExplicitDetectorModelPath:    getEnv("EXPLICIT_DETECTOR_MODEL_PATH", "./models/explicit.onnx"),
		ExplicitDetectorConcurrency:  getEnvInt("EXPLICIT_DETECTOR_CONCURRENCY", 2),
		ExplicitDetectorMinPornScore: getEnvFloat("EXPLICIT_DETECTOR_MIN_PORN_SCORE", 0.25),
		ExplicitDetectorMaxPornScore: getEnvFloat("EXPLICIT_DETECTOR_MAX_PORN_SCORE", 0.80),
		PPROFEnabled:                 getEnvBool("PPROF_ENABLED", false),
	}

	if c.PornReviewThreshold < 0 || c.PornReviewThreshold > 1 {
		return nil, fmt.Errorf("invalid PORN_REVIEW_THRESHOLD")
	}
	if c.PornThreshold < 0 || c.PornThreshold > 1 {
		return nil, fmt.Errorf("invalid PORN_THRESHOLD")
	}
	if c.InferenceConcurrency < 1 {
		c.InferenceConcurrency = 1
	}
	if c.MaxPendingInferences < 1 {
		c.MaxPendingInferences = 1
	}

	return c, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
