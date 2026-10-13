// Package ocr implements local PP-OCR recognition.
package ocr

import "context"

type Box struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Block struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Box        Box     `json:"box"`
	Order      int     `json:"order"`
}

type Request struct {
	Language    string
	ReturnBoxes bool
}

type Result struct {
	Text             string         `json:"text"`
	Model            string         `json:"model"`
	Language         string         `json:"language,omitempty"`
	ProcessingTimeMS int64          `json:"processing_time_ms"`
	Blocks           []Block        `json:"blocks"`
	Metadata         map[string]any `json:"metadata"`
}

type Recognizer interface {
	Recognize(ctx context.Context, image []byte, request Request) (Result, error)
	Close() error
}
