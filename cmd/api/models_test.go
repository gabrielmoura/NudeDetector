package main

import (
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/config"
)

func TestInstallArtifactsUsesPinnedOPUSManifest(t *testing.T) {
	artifacts, err := installArtifacts("opus-mt-en-romance", &config.Config{TranslationModelsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 4 {
		t.Fatalf("artifact count = %d, want 4", len(artifacts))
	}
	for _, artifact := range artifacts {
		if artifact.SHA256 == "" || artifact.MaxBytes < 1 {
			t.Fatalf("unsafe artifact: %#v", artifact)
		}
	}
}
