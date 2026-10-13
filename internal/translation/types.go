// Package translation implements local machine translation services.
package translation

import "context"

// Request describes one translation operation.
type Request struct {
	Text           string
	SourceLanguage string
	TargetLanguage string
}

// Result is the serializable result of a translation operation.
type Result struct {
	SourceText       string         `json:"source_text"`
	TranslatedText   string         `json:"translated_text"`
	SourceLanguage   string         `json:"source_language"`
	TargetLanguage   string         `json:"target_language"`
	Model            string         `json:"model"`
	ProcessingTimeMS int64          `json:"processing_time_ms"`
	Metadata         map[string]any `json:"metadata"`
}

// Translator is shared by HTTP and CLI callers.
type Translator interface {
	Translate(context.Context, Request) (Result, error)
	Close() error
}
