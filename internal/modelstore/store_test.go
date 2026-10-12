package modelstore

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDownloadsAndChecksumsModel(t *testing.T) {
	payload := []byte("test ONNX model bytes")
	sum := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "models", "model.onnx")
	if err := Ensure(path, server.URL, fmt.Sprintf("%x", sum)); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed model: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("installed model = %q, want %q", got, payload)
	}
}

func TestEnsureRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("unexpected"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "model.onnx")
	if err := Ensure(path, server.URL, "0000"); err == nil {
		t.Fatal("Ensure() error = nil, want checksum mismatch")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("model installed after checksum mismatch: %v", err)
	}
}
