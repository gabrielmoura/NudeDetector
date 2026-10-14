package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadRejectsInvalidEnvironmentSyntax(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "integer", key: "MAX_IMAGE_BYTES", value: "large", want: "must be an integer"},
		{name: "float", key: "PORN_THRESHOLD", value: "high", want: "must be a number"},
		{name: "boolean", key: "CACHE_ENABLED", value: "sometimes", want: "must be boolean"},
		{name: "model auto install", key: "MODEL_AUTO_INSTALL", value: "sometimes", want: "must be boolean"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfigEnvironment(t)
			t.Setenv(test.key, test.value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want message containing %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsInconsistentThresholds(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("PORN_REVIEW_THRESHOLD", "0.9")
	t.Setenv("PORN_THRESHOLD", "0.8")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "less than or equal") {
		t.Fatalf("Load() error = %v, want threshold ordering error", err)
	}
}

func TestLoadRejectsNonFiniteThreshold(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("PORN_THRESHOLD", "NaN")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORN_THRESHOLD") {
		t.Fatalf("Load() error = %v, want non-finite threshold error", err)
	}
}

func TestLoadRejectsUnsafeLimits(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  string
	}{
		{key: "MAX_IMAGE_BYTES", value: "0", want: "MAX_IMAGE_BYTES"},
		{key: "MAX_REDIRECTS", value: "-1", want: "MAX_REDIRECTS"},
		{key: "INFERENCE_CONCURRENCY", value: "0", want: "INFERENCE_CONCURRENCY"},
		{key: "CACHE_TTL", value: "0", want: "CACHE_MAX_ENTRIES and CACHE_TTL"},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			clearConfigEnvironment(t)
			t.Setenv(test.key, test.value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want message containing %q", err, test.want)
			}
		})
	}
}

func TestLoadUsesSafeDefaults(t *testing.T) {
	clearConfigEnvironment(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if c.MaxImageBytes <= 0 || c.MaxImagePixels <= 0 || c.InferenceConcurrency <= 0 {
		t.Fatalf("Load() returned unsafe defaults: %#v", c)
	}
	if got := strings.Join(c.EnabledModels, ","); got != "nsfw-classifier" {
		t.Fatalf("EnabledModels = %q, want nsfw-classifier", got)
	}
	if c.ModelAutoInstall {
		t.Fatal("ModelAutoInstall = true, want false by default")
	}
}

func TestLoadEnablesStartupModelInstallOnlyWhenRequested(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("MODEL_AUTO_INSTALL", "true")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !c.ModelAutoInstall {
		t.Fatal("ModelAutoInstall = false, want true")
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	keys := []string{
		"PORN_REVIEW_THRESHOLD", "PORN_THRESHOLD", "MAX_IMAGE_BYTES", "MAX_IMAGE_PIXELS",
		"MAX_WIDTH", "MAX_HEIGHT", "MAX_AUDIO_BYTES", "ASR_MODELS_DIR", "ASR_THREADS", "ASR_CHUNK_SECONDS", "DOWNLOAD_TIMEOUT", "DIAL_TIMEOUT",
		"TLS_HANDSHAKE_TIMEOUT", "RESPONSE_HEADER_TIMEOUT", "MAX_REDIRECTS",
		"INFERENCE_CONCURRENCY", "MAX_PENDING_INFERENCES", "ONNX_INTRA_OP_THREADS",
		"ONNX_INTER_OP_THREADS", "CACHE_MAX_ENTRIES", "CACHE_TTL",
		"CLASSIFY_HENTAI_AS_PORNOGRAPHY", "CACHE_ENABLED", "DETECTOR_ENABLED", "MODEL_AUTO_INSTALL", "PPROF_ENABLED",
	}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			t.Setenv(key, value)
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}
		_ = os.Unsetenv(key)
	}
}
