package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

var ErrUnsupportedMedia = errors.New("unsupported media type")

func ImageFrame(ctx context.Context, data []byte, contentType string, frameTimeMS int64) ([]byte, error) {
	if contentType == "image/gif" || hasVideoPrefix(contentType) {
		return extractWithFFmpeg(ctx, data, frameTimeMS)
	}
	return data, nil
}

func hasVideoPrefix(contentType string) bool {
	return len(contentType) >= len("video/") && contentType[:len("video/")] == "video/"
}

func extractWithFFmpeg(ctx context.Context, data []byte, frameTimeMS int64) ([]byte, error) {
	if frameTimeMS < 0 {
		return nil, errors.New("frame_time_ms must not be negative")
	}
	directory, err := os.MkdirTemp("", "nude-detector-media-")
	if err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	defer os.RemoveAll(directory)
	inputPath := filepath.Join(directory, "input")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write media input: %w", err)
	}
	arguments := []string{"-hide_banner", "-loglevel", "error", "-ss", strconv.FormatFloat(float64(frameTimeMS)/1000, 'f', 3, 64), "-i", inputPath, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1"}
	command := exec.CommandContext(ctx, "ffmpeg", arguments...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	frame, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("extract media frame: %w: %s", err, stderr.String())
	}
	if len(frame) == 0 {
		return nil, errors.New("ffmpeg returned an empty frame")
	}
	return frame, nil
}
