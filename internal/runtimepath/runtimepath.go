package runtimepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gabrielmoura/NudeDetector/internal/buildinfo"
)

const defaultLibraryName = "libonnxruntime.so.1.22.0"

func Resolve(configured string, executable string, workingDirectory string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect configured ONNX Runtime %q: %w", configured, err)
		}
		return "", fmt.Errorf("configured ONNX Runtime %q does not exist; set ONNXRUNTIME_SHARED_LIBRARY_PATH to a valid library", configured)
	}

	candidates := []string{
		filepath.Join(workingDirectory, ".cache", "onnxruntime", buildinfoRuntimeVersion(), "lib", defaultLibraryName),
		filepath.Join("/app", "lib", defaultLibraryName),
	}
	if executable != "" {
		executableDirectory := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(executableDirectory, "..", ".cache", "onnxruntime", buildinfoRuntimeVersion(), "lib", defaultLibraryName),
			filepath.Join(executableDirectory, "lib", defaultLibraryName),
		)
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"ONNX Runtime shared library not found; run 'make onnxruntime' or set ONNXRUNTIME_SHARED_LIBRARY_PATH (searched: %v)",
		candidates,
	)
}

func buildinfoRuntimeVersion() string {
	return buildinfo.ONNXRuntimeVersion
}
