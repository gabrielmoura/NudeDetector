package asr

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
)

const whisperSampleRate = 16_000

// DecodeAudio accepts WAV directly and delegates other container formats to
// ffmpeg, always returning mono float32 PCM at Whisper's sample rate.
func DecodeAudio(ctx context.Context, data []byte, maxDurationSeconds int) (Audio, error) {
	if maxDurationSeconds < 1 {
		return Audio{}, errors.New("maximum audio duration must be greater than zero")
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		audio, err := DecodeWAV(data)
		if err != nil {
			return Audio{}, err
		}
		if durationSeconds(audio) > float64(maxDurationSeconds) {
			return Audio{}, fmt.Errorf("audio exceeds %d-second limit", maxDurationSeconds)
		}
		return audio, nil
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return Audio{}, errors.New("ffmpeg is required for non-WAV audio")
	}
	maxOutputBytes := int64(maxDurationSeconds) * whisperSampleRate * 4
	command := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-vn", "-ac", "1", "-ar", "16000", "-f", "f32le", "pipe:1")
	command.Stdin = bytes.NewReader(data)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return Audio{}, fmt.Errorf("create audio decoder output: %w", err)
	}
	var stderr limitedBuffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return Audio{}, fmt.Errorf("start ffmpeg: %w", err)
	}
	pcm, readErr := io.ReadAll(io.LimitReader(stdout, maxOutputBytes+1))
	waitErr := command.Wait()
	if readErr != nil {
		return Audio{}, fmt.Errorf("read decoded audio: %w", readErr)
	}
	if int64(len(pcm)) > maxOutputBytes {
		return Audio{}, fmt.Errorf("audio exceeds %d-second limit", maxDurationSeconds)
	}
	if waitErr != nil {
		return Audio{}, fmt.Errorf("decode audio with ffmpeg: %w: %s", waitErr, stderr.String())
	}
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		return Audio{}, errors.New("ffmpeg returned invalid PCM audio")
	}
	samples := make([]float32, len(pcm)/4)
	for i := range samples {
		samples[i] = mathFloat32(pcm[i*4:])
	}
	return Audio{Samples: samples, SampleRate: whisperSampleRate}, nil
}

func durationSeconds(audio Audio) float64 {
	if audio.SampleRate == 0 {
		return 0
	}
	return float64(len(audio.Samples)) / float64(audio.SampleRate)
}
func mathFloat32(data []byte) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(data)) }

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	const maximum = 8192
	remaining := maximum - b.Len()
	if remaining > 0 {
		if len(data) > remaining {
			_, _ = b.Buffer.Write(data[:remaining])
		} else {
			_, _ = b.Buffer.Write(data)
		}
	}
	return len(data), nil
}
