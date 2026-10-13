package aimodel

import (
	"strings"
	"testing"
)

func TestRegistryDefaultsToOnlyNSFWClassifier(t *testing.T) {
	t.Parallel()

	registry, err := New("missing-classifier.onnx", "missing-detector.onnx", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !registry.Enabled(NSFWClassifierID) {
		t.Fatal("NSFW classifier is not enabled by default")
	}
	if registry.Enabled(NudeNetDetectorID) {
		t.Fatal("NudeNet detector is enabled by default")
	}
	model, ok := registry.Get("whisper-base")
	if !ok || model.State != StateNotInstalled || model.Enabled {
		t.Fatalf("whisper-base = %#v, found = %t", model, ok)
	}
}

func TestRegistryRejectsUnknownEnabledModel(t *testing.T) {
	t.Parallel()

	_, err := New("classifier.onnx", "detector.onnx", []string{"unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("New() error = %v, want unknown model error", err)
	}
}

func TestRegistryRejectsDuplicateEnabledModel(t *testing.T) {
	t.Parallel()

	_, err := New("classifier.onnx", "detector.onnx", []string{NSFWClassifierID, NSFWClassifierID})
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("New() error = %v, want duplicate model error", err)
	}
}

func TestParseEnabledRejectsEmptyAndDuplicateIDs(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"nsfw-classifier,,nudenet-detector", "nsfw-classifier,nsfw-classifier"} {
		if _, err := ParseEnabled(input); err == nil {
			t.Fatalf("ParseEnabled(%q) error = nil", input)
		}
	}
}
