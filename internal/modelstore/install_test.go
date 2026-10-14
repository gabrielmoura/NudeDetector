package modelstore

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRejectsUnsafeURL(t *testing.T) {
	err := Install(context.Background(), []Artifact{{Path: t.TempDir() + "/model.onnx", URL: "http://example.test/model.onnx", SHA256: strings.Repeat("0", 64), MaxBytes: 1}})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Install() error = %v, want HTTPS rejection", err)
	}
}

func TestVerifyAndRemoveArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.onnx")
	payload := []byte("model")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	artifacts := []Artifact{{Path: path, SHA256: fmt.Sprintf("%x", sum)}}
	if err := Verify(artifacts); err != nil {
		t.Fatal(err)
	}
	if err := Remove(artifacts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("artifact still exists: %v", err)
	}
}

func TestInstallRejectsIncompleteArtifact(t *testing.T) {
	err := Install(context.Background(), []Artifact{{}})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("Install() error = %v, want validation error", err)
	}
}
