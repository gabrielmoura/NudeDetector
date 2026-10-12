package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr string

	// Classifier
	ClassifierModelPath    string
	ClassifierMetadataPath string
	ClassifierModelURL     string
	ClassifierModelSHA256  string
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

	// NudeNet detector used by the export endpoint.
	DetectorEnabled      bool
	DetectorModelPath    string
	DetectorModelURL     string
	DetectorModelSHA256  string
	DetectorScoreMinimum float64
	DetectorIoUThreshold float64

	// Observability
	PPROFEnabled bool
}

func Load() (*Config, error) {
	if err := validateEnvironmentSyntax(); err != nil {
		return nil, err
	}

	c := &Config{
		HTTPAddr: getEnv("HTTP_ADDR", ":8080"),

		ClassifierModelPath:    getEnv("CLASSIFIER_MODEL_PATH", "./models/model.onnx"),
		ClassifierMetadataPath: getEnv("CLASSIFIER_METADATA_PATH", "./models/model.json"),
		ClassifierModelURL:     getEnv("CLASSIFIER_MODEL_URL", "https://huggingface.co/onnx-community/nsfw-classifier-ONNX/resolve/main/onnx/model.onnx"),
		ClassifierModelSHA256:  getEnv("CLASSIFIER_MODEL_SHA256", "d6e841f61f193d9f435b464748c772b4096edd941c9f12b3a05bcb655fb267eb"),
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
			"",
		),

		CacheEnabled:    getEnvBool("CACHE_ENABLED", true),
		CacheMaxEntries: getEnvInt("CACHE_MAX_ENTRIES", 1000),
		CacheTTL:        getEnvInt64("CACHE_TTL", 3600_000),

		DetectorEnabled:      getEnvBool("DETECTOR_ENABLED", true),
		DetectorModelPath:    getEnv("DETECTOR_MODEL_PATH", "./models/320n.onnx"),
		DetectorModelURL:     getEnv("DETECTOR_MODEL_URL", "https://raw.githubusercontent.com/notAI-tech/NudeNet/v3/nudenet/320n.onnx"),
		DetectorModelSHA256:  getEnv("DETECTOR_MODEL_SHA256", "c15d8273adad2d0a92f014cc69ab2d6c311a06777a55545f2c4eb46f51911f0f"),
		DetectorScoreMinimum: getEnvFloat("DETECTOR_SCORE_MINIMUM", 0.25),
		DetectorIoUThreshold: getEnvFloat("DETECTOR_IOU_THRESHOLD", 0.45),
		PPROFEnabled:         getEnvBool("PPROF_ENABLED", false),
	}

	if !validUnitInterval(c.PornReviewThreshold) {
		return nil, fmt.Errorf("invalid PORN_REVIEW_THRESHOLD")
	}
	if !validUnitInterval(c.PornThreshold) {
		return nil, fmt.Errorf("invalid PORN_THRESHOLD")
	}
	if c.PornReviewThreshold > c.PornThreshold {
		return nil, fmt.Errorf("PORN_REVIEW_THRESHOLD must be less than or equal to PORN_THRESHOLD")
	}
	if c.MaxImageBytes <= 0 {
		return nil, fmt.Errorf("MAX_IMAGE_BYTES must be greater than zero")
	}
	if c.MaxImagePixels <= 0 {
		return nil, fmt.Errorf("MAX_IMAGE_PIXELS must be greater than zero")
	}
	if c.MaxWidth <= 0 || c.MaxHeight <= 0 {
		return nil, fmt.Errorf("MAX_WIDTH and MAX_HEIGHT must be greater than zero")
	}
	if c.DownloadTimeout <= 0 || c.DialTimeout <= 0 || c.TLSHandshakeTimeout <= 0 || c.ResponseHeaderTimeout <= 0 {
		return nil, fmt.Errorf("download timeouts must be greater than zero")
	}
	if c.MaxRedirects < 0 {
		return nil, fmt.Errorf("MAX_REDIRECTS must not be negative")
	}
	if c.InferenceConcurrency < 1 {
		return nil, fmt.Errorf("INFERENCE_CONCURRENCY must be greater than zero")
	}
	if c.MaxPendingInferences < 1 {
		return nil, fmt.Errorf("MAX_PENDING_INFERENCES must be greater than zero")
	}
	if c.ONNXIntraOpThreads < 0 || c.ONNXInterOpThreads < 0 {
		return nil, fmt.Errorf("ONNX thread counts must not be negative")
	}
	if c.CacheMaxEntries <= 0 || c.CacheTTL <= 0 {
		return nil, fmt.Errorf("CACHE_MAX_ENTRIES and CACHE_TTL must be greater than zero")
	}
	if !validUnitInterval(c.DetectorScoreMinimum) {
		return nil, fmt.Errorf("invalid DETECTOR_SCORE_MINIMUM")
	}
	if !validUnitInterval(c.DetectorIoUThreshold) {
		return nil, fmt.Errorf("invalid DETECTOR_IOU_THRESHOLD")
	}

	return c, nil
}

func validUnitInterval(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validateEnvironmentSyntax() error {
	integerKeys := []string{
		"MAX_IMAGE_BYTES", "MAX_IMAGE_PIXELS", "MAX_WIDTH", "MAX_HEIGHT",
		"DOWNLOAD_TIMEOUT", "DIAL_TIMEOUT", "TLS_HANDSHAKE_TIMEOUT",
		"RESPONSE_HEADER_TIMEOUT", "MAX_REDIRECTS", "INFERENCE_CONCURRENCY",
		"MAX_PENDING_INFERENCES", "ONNX_INTRA_OP_THREADS", "ONNX_INTER_OP_THREADS",
		"CACHE_MAX_ENTRIES", "CACHE_TTL",
	}
	for _, key := range integerKeys {
		if value := os.Getenv(key); value != "" {
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				return fmt.Errorf("invalid %s: must be an integer", key)
			}
		}
	}

	floatKeys := []string{
		"PORN_REVIEW_THRESHOLD", "PORN_THRESHOLD", "DETECTOR_SCORE_MINIMUM",
		"DETECTOR_IOU_THRESHOLD",
	}
	for _, key := range floatKeys {
		if value := os.Getenv(key); value != "" {
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return fmt.Errorf("invalid %s: must be a number", key)
			}
		}
	}

	booleanKeys := []string{
		"CLASSIFY_HENTAI_AS_PORNOGRAPHY", "CACHE_ENABLED", "DETECTOR_ENABLED", "PPROF_ENABLED",
	}
	for _, key := range booleanKeys {
		if value := os.Getenv(key); value != "" {
			if _, err := strconv.ParseBool(value); err != nil {
				return fmt.Errorf("invalid %s: must be boolean", key)
			}
		}
	}
	return nil
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
