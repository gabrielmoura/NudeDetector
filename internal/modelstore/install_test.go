package modelstore

import (
	"context"
	"strings"
	"testing"
)

func TestInstallRejectsUnsafeURL(t *testing.T) {
	err := Install(context.Background(), []Artifact{{Path: t.TempDir() + "/model.onnx", URL: "http://example.test/model.onnx", SHA256: strings.Repeat("0", 64), MaxBytes: 1}})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("Install() error = %v, want HTTPS rejection", err)
	}
}

func TestInstallRejectsIncompleteArtifact(t *testing.T) {
	err := Install(context.Background(), []Artifact{{}})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("Install() error = %v, want validation error", err)
	}
}
