package modelstore

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Artifact is one immutable model file installed by an explicit CLI action.
type Artifact struct {
	Path     string
	URL      string
	SHA256   string
	MaxBytes int64
}

// Install downloads and atomically installs immutable artifacts. It is never
// called from an inference request path.
func Install(ctx context.Context, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		if err := installOne(ctx, artifact); err != nil {
			return err
		}
	}
	return nil
}

func installOne(ctx context.Context, artifact Artifact) error {
	if artifact.Path == "" || artifact.URL == "" || artifact.SHA256 == "" || artifact.MaxBytes < 1 {
		return fmt.Errorf("model artifact path, URL, checksum, and size limit are required")
	}
	parsed, err := url.Parse(artifact.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("model artifact URL must be HTTPS")
	}
	if existing, err := checksumFile(artifact.Path); err == nil && strings.EqualFold(existing, artifact.SHA256) {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("verify existing artifact: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(artifact.Path), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	client := &http.Client{CheckRedirect: func(request *http.Request, _ []*http.Request) error {
		if request.URL.Scheme != "https" {
			return fmt.Errorf("redirected model URL must be HTTPS")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return fmt.Errorf("create model download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download model artifact: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download model artifact: unexpected status %s", response.Status)
	}
	if response.ContentLength > artifact.MaxBytes {
		return fmt.Errorf("model artifact exceeds %d-byte limit", artifact.MaxBytes)
	}
	temporary, err := os.CreateTemp(filepath.Dir(artifact.Path), ".model-*")
	if err != nil {
		return fmt.Errorf("create model temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(response.Body, artifact.MaxBytes+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("write model artifact: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close model artifact: %w", closeErr)
	}
	if written > artifact.MaxBytes {
		return fmt.Errorf("model artifact exceeds %d-byte limit", artifact.MaxBytes)
	}
	if actual := fmt.Sprintf("%x", hash.Sum(nil)); !strings.EqualFold(actual, artifact.SHA256) {
		return fmt.Errorf("model artifact checksum mismatch: got %s", actual)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return fmt.Errorf("set model artifact permissions: %w", err)
	}
	if err := os.Rename(temporaryPath, artifact.Path); err != nil {
		return fmt.Errorf("install model artifact: %w", err)
	}
	return nil
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
