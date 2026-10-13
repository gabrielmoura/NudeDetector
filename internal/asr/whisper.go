package asr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

type WhisperConfig struct {
	ModelID     string
	EncoderPath string
	DecoderPath string
	TokensPath  string
	Threads     int
}

type Whisper struct {
	modelID    string
	recognizer *sherpa.OfflineRecognizer
	sem        chan struct{}
	closeOnce  sync.Once
}

func NewWhisper(config WhisperConfig) (*Whisper, error) {
	if config.ModelID == "" {
		return nil, errors.New("ASR model ID is required")
	}
	for name, path := range map[string]string{
		"encoder": config.EncoderPath,
		"decoder": config.DecoderPath,
		"tokens":  config.TokensPath,
	} {
		if path == "" {
			return nil, fmt.Errorf("Whisper %s path is required", name)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("stat Whisper %s: %w", name, err)
		}
	}
	if config.Threads < 1 {
		return nil, errors.New("ASR thread count must be greater than zero")
	}

	recognizer := sherpa.NewOfflineRecognizer(&sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{SampleRate: 16000, FeatureDim: 80},
		ModelConfig: sherpa.OfflineModelConfig{
			Whisper: sherpa.OfflineWhisperModelConfig{
				Encoder:                 config.EncoderPath,
				Decoder:                 config.DecoderPath,
				Task:                    "transcribe",
				TailPaddings:            -1,
				EnableTokenTimestamps:   1,
				EnableSegmentTimestamps: 1,
			},
			Tokens:     config.TokensPath,
			NumThreads: config.Threads,
			Provider:   "cpu",
			ModelType:  "whisper",
		},
		DecodingMethod: "greedy_search",
	})
	if recognizer == nil {
		return nil, errors.New("create Whisper recognizer")
	}
	return &Whisper{
		modelID:    config.ModelID,
		recognizer: recognizer,
		sem:        make(chan struct{}, 1),
	}, nil
}

func (w *Whisper) Transcribe(ctx context.Context, audio Audio, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if audio.SampleRate < 1 || len(audio.Samples) == 0 {
		return Result{}, errors.New("audio must contain samples and a positive sample rate")
	}
	select {
	case w.sem <- struct{}{}:
		defer func() { <-w.sem }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}

	start := time.Now()
	stream := sherpa.NewOfflineStream(w.recognizer)
	if stream == nil {
		return Result{}, errors.New("create Whisper stream")
	}
	defer sherpa.DeleteOfflineStream(stream)
	if language := normalizeLanguage(request.Language); language != "" {
		stream.SetOption("language", language)
	}
	stream.AcceptWaveform(audio.SampleRate, audio.Samples)
	w.recognizer.Decode(stream)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	recognized := stream.GetResult()
	metadata := map[string]any{
		"engine":                    "sherpa-onnx",
		"segment_timestamps":        false,
		"token_timestamps_returned": len(recognized.Timestamps) > 0,
	}
	if request.Timestamps && len(recognized.Timestamps) > 0 {
		metadata["token_timestamps_seconds"] = recognized.Timestamps
	}
	language := recognized.Lang
	if language == "" {
		language = normalizeLanguage(request.Language)
	}
	return Result{
		Text:             recognized.Text,
		Language:         language,
		Model:            w.modelID,
		DurationSeconds:  float64(len(audio.Samples)) / float64(audio.SampleRate),
		ProcessingTimeMS: time.Since(start).Milliseconds(),
		Segments:         []Segment{},
		Metadata:         metadata,
	}, nil
}

func (w *Whisper) Close() error {
	w.closeOnce.Do(func() {
		if w.recognizer != nil {
			sherpa.DeleteOfflineRecognizer(w.recognizer)
			w.recognizer = nil
		}
	})
	return nil
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if index := strings.IndexByte(language, '-'); index > 0 {
		return language[:index]
	}
	return language
}
