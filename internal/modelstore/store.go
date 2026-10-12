package modelstore

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func Ensure(path, sourceURL, expectedSHA256 string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat model: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}

	response, err := http.Get(sourceURL)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download model: unexpected status %s", response.Status)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".model-*")
	if err != nil {
		return fmt.Errorf("create temporary model: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temporary, hash), response.Body); err != nil {
		temporary.Close()
		return fmt.Errorf("write model: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close model: %w", err)
	}
	if actual := fmt.Sprintf("%x", hash.Sum(nil)); actual != expectedSHA256 {
		return fmt.Errorf("downloaded model checksum mismatch: got %s", actual)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return fmt.Errorf("set model permissions: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("install model: %w", err)
	}
	return nil
}
