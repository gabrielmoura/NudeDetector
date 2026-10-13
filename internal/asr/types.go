// Package asr implements local automatic speech recognition services.
package asr

import "context"

type Audio struct {
	Samples    []float32
	SampleRate int
}

type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type Request struct {
	Language   string
	Timestamps bool
}

type Result struct {
	Text             string         `json:"text"`
	Language         string         `json:"language,omitempty"`
	Model            string         `json:"model"`
	DurationSeconds  float64        `json:"duration_seconds"`
	ProcessingTimeMS int64          `json:"processing_time_ms"`
	Segments         []Segment      `json:"segments"`
	Metadata         map[string]any `json:"metadata"`
}

type Transcriber interface {
	Transcribe(ctx context.Context, audio Audio, request Request) (Result, error)
	Close() error
}
