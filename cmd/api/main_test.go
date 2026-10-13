package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/inference"
	"github.com/gabrielmoura/NudeDetector/internal/model"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
)

type testEngine struct {
	shape  []int64
	logits inference.Logits
}

func (e testEngine) Infer(context.Context, []float32) (inference.Logits, error) { return e.logits, nil }
func (e testEngine) Close() error                                               { return nil }
func (e testEngine) InputShape() []int64                                        { return e.shape }

func TestRootCommandExposesAPIAndLocalCommands(t *testing.T) {
	t.Parallel()

	root := newRootCommand()
	for _, name := range []string{"serve", "classify", "export", "scan", "asr", "ocr"} {
		if command, _, err := root.Find([]string{name}); err != nil || command == root {
			t.Fatalf("command %q is not registered: %v", name, err)
		}
	}

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root help: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("root help did not produce output")
	}
}

func TestScanMediaKindAndOutputPath(t *testing.T) {
	t.Parallel()

	if got := scanMediaKind("sample.JPEG"); got != "image" {
		t.Fatalf("scanMediaKind(image) = %q, want image", got)
	}
	if got := scanMediaKind("clip.MP4"); got != "media" {
		t.Fatalf("scanMediaKind(media) = %q, want media", got)
	}
	if got := scanMediaKind("notes.txt"); got != "" {
		t.Fatalf("scanMediaKind(unsupported) = %q, want empty", got)
	}

	want := filepath.Join("out", "nested", "clip-moderated.jpg")
	if got := scanOutputPath("input", "out", filepath.Join("input", "nested", "clip.mp4")); got != want {
		t.Fatalf("scanOutputPath() = %q, want %q", got, want)
	}
	if !pathWithin("/data/input", "/data/input/output") {
		t.Fatal("pathWithin() = false, want nested path to be detected")
	}
	if pathWithin("/data/input", "/data/other") {
		t.Fatal("pathWithin() = true, want sibling path to be outside")
	}
}

func TestWarmupClassifierValidatesShapeAndClassCount(t *testing.T) {
	t.Parallel()

	metadata := &model.ModelMetadata{
		Width:     224,
		Height:    224,
		Labels:    []string{"drawings", "hentai", "neutral", "porn", "sexy"},
		PornLabel: "porn",
	}
	classifier, err := moderation.NewClassifier(metadata)
	if err != nil {
		t.Fatalf("NewClassifier() error = %v", err)
	}
	engine := testEngine{
		shape:  []int64{1, 3, 224, 224},
		logits: inference.Logits{1, 2, 3, 4, 5},
	}
	if err := warmupClassifier(engine, classifier, metadata); err != nil {
		t.Fatalf("warmupClassifier() error = %v", err)
	}

	engine.logits = inference.Logits{1}
	if err := warmupClassifier(engine, classifier, metadata); err == nil {
		t.Fatal("warmupClassifier() error = nil, want class count mismatch")
	}
}

func TestVersionCommandWritesJSON(t *testing.T) {
	t.Parallel()

	root := newRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"version"`)) ||
		!bytes.Contains(output.Bytes(), []byte(`"commit"`)) {
		t.Fatalf("version output = %q, want JSON build fields", output.String())
	}
}
