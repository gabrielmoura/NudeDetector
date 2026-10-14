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
	ModelID      string
	EncoderPath  string
	DecoderPath  string
	TokensPath   string
	Threads      int
	ChunkSeconds int
}

type Whisper struct {
	modelID      string
	recognizer   *sherpa.OfflineRecognizer
	sem          chan struct{}
	chunkSamples int
	closeOnce    sync.Once
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
	if config.Threads < 1 || config.ChunkSeconds < 1 {
		return nil, errors.New("ASR thread count and chunk duration must be greater than zero")
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
		modelID:      config.ModelID,
		recognizer:   recognizer,
		sem:          make(chan struct{}, 1),
		chunkSamples: 16_000 * config.ChunkSeconds,
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
	texts := make([]string, 0, (len(audio.Samples)+w.chunkSamples-1)/w.chunkSamples)
	var language string
	var tokenTimestamps []float32
	for startIndex := 0; startIndex < len(audio.Samples); startIndex += w.chunkSamples {
		endIndex := min(startIndex+w.chunkSamples, len(audio.Samples))
		recognized, err := w.transcribeChunk(audio.SampleRate, audio.Samples[startIndex:endIndex], request.Language)
		if err != nil {
			return Result{}, err
		}
		if recognized.Lang != "" {
			language = recognized.Lang
		}
		texts = append(texts, recognized.Text)
		if request.Timestamps && len(recognized.Timestamps) > 0 {
			offset := float32(startIndex) / float32(audio.SampleRate)
			for _, timestamp := range recognized.Timestamps {
				tokenTimestamps = append(tokenTimestamps, timestamp+offset)
			}
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
	}
	metadata := map[string]any{
		"engine":                    "sherpa-onnx",
		"segment_timestamps":        false,
		"chunk_count":               len(texts),
		"token_timestamps_returned": len(tokenTimestamps) > 0,
	}
	if request.Timestamps && len(tokenTimestamps) > 0 {
		metadata["token_timestamps_seconds"] = tokenTimestamps
	}
	if language == "" {
		language = normalizeLanguage(request.Language)
	}
	return Result{Text: reconcileChunkText(texts), Language: language, Model: w.modelID, DurationSeconds: float64(len(audio.Samples)) / float64(audio.SampleRate), ProcessingTimeMS: time.Since(start).Milliseconds(), Segments: []Segment{}, Metadata: metadata}, nil
}

func (w *Whisper) transcribeChunk(sampleRate int, samples []float32, language string) (*sherpa.OfflineRecognizerResult, error) {
	stream := sherpa.NewOfflineStream(w.recognizer)
	if stream == nil {
		return nil, errors.New("create Whisper stream")
	}
	defer sherpa.DeleteOfflineStream(stream)
	if language := normalizeLanguage(language); language != "" {
		stream.SetOption("language", language)
	}
	stream.AcceptWaveform(sampleRate, samples)
	w.recognizer.Decode(stream)
	return stream.GetResult(), nil
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

func reconcileChunkText(chunks []string) string {
	result := ""
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		if result == "" {
			result = chunk
			continue
		}
		resultWords := strings.Fields(result)
		chunkWords := strings.Fields(chunk)
		maximum := min(12, len(resultWords), len(chunkWords))
		overlap := 0
		for size := maximum; size > 0; size-- {
			if strings.EqualFold(strings.Join(resultWords[len(resultWords)-size:], " "), strings.Join(chunkWords[:size], " ")) {
				overlap = size
				break
			}
		}
		result = strings.TrimSpace(result + " " + strings.Join(chunkWords[overlap:], " "))
	}
	return result
}
