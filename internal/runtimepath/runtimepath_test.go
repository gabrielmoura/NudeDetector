package runtimepath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsRepositoryRuntimeRelativeToExecutable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	executable := filepath.Join(root, "bin", "nude-detector")
	library := filepath.Join(root, ".cache", "onnxruntime", "1.22.0", "lib", defaultLibraryName)
	if err := os.MkdirAll(filepath.Dir(library), 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	if err := os.WriteFile(library, []byte("runtime"), 0o644); err != nil {
		t.Fatalf("write runtime fixture: %v", err)
	}

	got, err := Resolve("", executable, filepath.Join(root, "other"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != library {
		t.Fatalf("Resolve() = %q, want %q", got, library)
	}
}

func TestResolveReportsInvalidConfiguredRuntime(t *testing.T) {
	t.Parallel()

	if _, err := Resolve("/missing/libonnxruntime.so", "", t.TempDir()); err == nil {
		t.Fatal("Resolve() error = nil, want missing configured library")
	}
}
