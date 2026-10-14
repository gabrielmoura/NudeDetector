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

func TestInstallArtifactsUsesPinnedM2MManifest(t *testing.T) {
	artifacts, err := installArtifacts("m2m100-418m", &config.Config{TranslationModelsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("artifact count = %d, want 3", len(artifacts))
	}
	for _, artifact := range artifacts {
		if artifact.SHA256 == "" || artifact.MaxBytes < 1 {
			t.Fatalf("unsafe artifact: %#v", artifact)
		}
	}
	if artifacts[1].MaxBytes < 860_415_003 {
		t.Fatalf("decoder limit = %d, want at least 860415003", artifacts[1].MaxBytes)
	}
}

func TestInstallArtifactsUsesPinnedWhisperAndOCRManifests(t *testing.T) {
	cfg := &config.Config{ASRModelsDir: t.TempDir(), OCRModelsDir: t.TempDir()}
	for _, modelID := range []string{"whisper-tiny", "whisper-base", "whisper-small", "pp-ocrv6-tiny", "pp-ocrv6-small"} {
		artifacts, err := installArtifacts(modelID, cfg)
		if err != nil {
			t.Fatalf("installArtifacts(%q): %v", modelID, err)
		}
		if len(artifacts) != 3 {
			t.Fatalf("installArtifacts(%q) count = %d, want 3", modelID, len(artifacts))
		}
		for _, artifact := range artifacts {
			if artifact.SHA256 == "" || artifact.MaxBytes < 1 {
				t.Fatalf("unsafe artifact for %s: %#v", modelID, artifact)
			}
		}
	}
}

func TestWhisperSmallManifestAllowsItsPinnedDecoder(t *testing.T) {
	artifacts, err := installArtifacts("whisper-small", &config.Config{ASRModelsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if artifacts[1].MaxBytes < 262_226_114 {
		t.Fatalf("decoder limit = %d, want at least 262226114", artifacts[1].MaxBytes)
	}
}
