package moderation

import (
	"math"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/model"
)

func TestClassifierMapsPornByLabelRatherThanIndex(t *testing.T) {
	t.Parallel()

	classifier, err := NewClassifier(&model.ModelMetadata{
		Labels:    []string{"sexy", "porn", "neutral", "drawings", "hentai"},
		PornLabel: "porn",
	})
	if err != nil {
		t.Fatalf("NewClassifier() error = %v", err)
	}

	scores, err := classifier.Classify([]float32{0.05, 0.80, 0.03, 0.02, 0.10})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if math.Abs(scores.Porn-0.80) > 0.000001 ||
		math.Abs(scores.Sexy-0.05) > 0.000001 ||
		math.Abs(scores.Hentai-0.10) > 0.000001 {
		t.Fatalf("unexpected score mapping: %#v", scores)
	}
}

func TestClassifierRejectsMismatchedOutput(t *testing.T) {
	t.Parallel()

	classifier, err := NewClassifier(&model.ModelMetadata{
		Labels:    []string{"drawings", "hentai", "neutral", "porn", "sexy"},
		PornLabel: "porn",
	})
	if err != nil {
		t.Fatalf("NewClassifier() error = %v", err)
	}
	if _, err := classifier.Classify([]float32{0.1}); err == nil {
		t.Fatal("Classify() error = nil, want output/label mismatch")
	}
}
