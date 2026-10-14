package translation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	sp "github.com/tggo/goSentencePiece"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	m2mEOSID      = int64(2)
	m2mVocabulary = 128112
	m2mLayers     = 12
	m2mHeads      = 16
	m2mHeadSize   = 64
)

var m2mLanguageTokens = map[string]int64{
	"en": 128022,
	"es": 128023,
	"fr": 128028,
	"it": 128045,
	"pt": 128075,
}

// M2MConfig identifies an installed M2M100 ONNX export.
type M2MConfig struct {
	ModelID         string
	EncoderPath     string
	DecoderPath     string
	TokenizerPath   string
	MaxInputTokens  int
	MaxOutputTokens int
}

// M2M100 translates supported language pairs locally through the M2M100
// encoder/decoder ONNX export. Inference is serialized to bound CPU and RAM.
type M2M100 struct {
	modelID         string
	encoder         *ort.DynamicAdvancedSession
	decoder         *ort.DynamicAdvancedSession
	tokenizer       *sp.Tokenizer
	maxInputTokens  int
	maxOutputTokens int
	mu              sync.Mutex
}

func NewM2M100(config M2MConfig) (*M2M100, error) {
	if config.ModelID == "" {
		return nil, errors.New("translation model ID is required")
	}
	if config.MaxInputTokens < 1 || config.MaxOutputTokens < 1 {
		return nil, errors.New("translation token limits must be greater than zero")
	}
	for name, path := range map[string]string{
		"encoder":   config.EncoderPath,
		"decoder":   config.DecoderPath,
		"tokenizer": config.TokenizerPath,
	} {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("stat M2M100 %s: %w", name, err)
		}
	}
	tokenizer, err := sp.NewTokenizerFromJSON(config.TokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("load M2M100 tokenizer: %w", err)
	}
	encoder, err := ort.NewDynamicAdvancedSession(
		config.EncoderPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create M2M100 encoder session: %w", err)
	}
	decoderInputs, decoderOutputs, err := ort.GetInputOutputInfo(config.DecoderPath)
	if err != nil {
		_ = encoder.Destroy()
		return nil, fmt.Errorf("inspect M2M100 decoder: %w", err)
	}
	inNames := make([]string, len(decoderInputs))
	outNames := make([]string, len(decoderOutputs))
	for i := range decoderInputs {
		inNames[i] = decoderInputs[i].Name
	}
	for i := range decoderOutputs {
		outNames[i] = decoderOutputs[i].Name
	}
	decoder, err := ort.NewDynamicAdvancedSession(config.DecoderPath, inNames, outNames, nil)
	if err != nil {
		_ = encoder.Destroy()
		return nil, fmt.Errorf("create M2M100 decoder session: %w", err)
	}
	return &M2M100{
		modelID:         config.ModelID,
		encoder:         encoder,
		decoder:         decoder,
		tokenizer:       tokenizer,
		maxInputTokens:  config.MaxInputTokens,
		maxOutputTokens: config.MaxOutputTokens,
	}, nil
}

func (m *M2M100) Translate(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return Result{}, errors.New("translation text is required")
	}
	source := normalizeLanguage(request.SourceLanguage)
	target := normalizeLanguage(request.TargetLanguage)
	sourceToken, sourceSupported := m2mLanguageTokens[source]
	targetToken, targetSupported := m2mLanguageTokens[target]
	if !sourceSupported || !targetSupported {
		return Result{}, fmt.Errorf("M2M100 does not support %q to %q", source, target)
	}
	encoded, err := m.tokenizer.Encode(text)
	if err != nil {
		return Result{}, fmt.Errorf("tokenize M2M100 input: %w", err)
	}
	if len(encoded)+2 > m.maxInputTokens {
		return Result{}, fmt.Errorf("translation input exceeds %d tokens", m.maxInputTokens)
	}
	inputIDs := make([]int64, 0, len(encoded)+2)
	inputIDs = append(inputIDs, sourceToken)
	for _, id := range encoded {
		inputIDs = append(inputIDs, int64(id))
	}
	inputIDs = append(inputIDs, m2mEOSID)
	m.mu.Lock()
	defer m.mu.Unlock()
	started := time.Now()
	generated, err := m.generate(ctx, inputIDs, targetToken)
	if err != nil {
		return Result{}, err
	}
	translated, err := m.tokenizer.Decode(generated)
	if err != nil {
		return Result{}, fmt.Errorf("decode M2M100 output: %w", err)
	}
	return Result{
		SourceText:       request.Text,
		TranslatedText:   strings.TrimSpace(translated),
		SourceLanguage:   source,
		TargetLanguage:   target,
		Model:            m.modelID,
		ProcessingTimeMS: time.Since(started).Milliseconds(),
		Metadata: map[string]any{
			"engine":       "onnxruntime",
			"architecture": "m2m100",
			"cache":        false,
		},
	}, nil
}

func (m *M2M100) generate(ctx context.Context, inputIDs []int64, targetToken int64) ([]int, error) {
	input, err := ort.NewTensor(ort.Shape{1, int64(len(inputIDs))}, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create M2M100 input tensor: %w", err)
	}
	defer input.Destroy()
	maskData := make([]int64, len(inputIDs))
	for i := range maskData {
		maskData[i] = 1
	}
	mask, err := ort.NewTensor(ort.Shape{1, int64(len(maskData))}, maskData)
	if err != nil {
		return nil, fmt.Errorf("create M2M100 attention tensor: %w", err)
	}
	defer mask.Destroy()
	encoderOutputs := []ort.Value{nil}
	if err := m.encoder.Run([]ort.Value{input, mask}, encoderOutputs); err != nil {
		return nil, fmt.Errorf("run M2M100 encoder: %w", err)
	}
	defer encoderOutputs[0].Destroy()
	hidden, ok := encoderOutputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("M2M100 encoder returned an unexpected tensor")
	}
	generated := []int{int(targetToken)}
	for range m.maxOutputTokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		decoderIDs := make([]int64, 1, len(generated)+1)
		decoderIDs[0] = m2mEOSID
		for _, token := range generated {
			decoderIDs = append(decoderIDs, int64(token))
		}
		next, err := m.decodeStep(mask, hidden, decoderIDs)
		if err != nil {
			return nil, err
		}
		if next == int(m2mEOSID) {
			return generated, nil
		}
		generated = append(generated, next)
	}
	return nil, fmt.Errorf("M2M100 generation exceeded %d tokens", m.maxOutputTokens)
}

func (m *M2M100) decodeStep(mask ort.Value, hidden ort.Value, decoderIDs []int64) (int, error) {
	decoderInput, err := ort.NewTensor(ort.Shape{1, int64(len(decoderIDs))}, decoderIDs)
	if err != nil {
		return 0, fmt.Errorf("create M2M100 decoder tensor: %w", err)
	}
	defer decoderInput.Destroy()
	inputs := []ort.Value{mask, decoderInput, hidden}
	caches := make([]*ort.Tensor[float32], 0, m2mLayers*4)
	for range m2mLayers * 4 {
		cache, cacheErr := ort.NewTensor(ort.Shape{1, m2mHeads, 0, m2mHeadSize}, []float32{})
		if cacheErr != nil {
			for _, value := range caches {
				_ = value.Destroy()
			}
			return 0, fmt.Errorf("create M2M100 cache tensor: %w", cacheErr)
		}
		caches = append(caches, cache)
		inputs = append(inputs, cache)
	}
	defer func() {
		for _, cache := range caches {
			_ = cache.Destroy()
		}
	}()
	branch, err := ort.NewTensor(ort.Shape{1}, []bool{false})
	if err != nil {
		return 0, fmt.Errorf("create M2M100 cache selector: %w", err)
	}
	defer branch.Destroy()
	inputs = append(inputs, branch)
	outputs := make([]ort.Value, 1+m2mLayers*4)
	if err := m.decoder.Run(inputs, outputs); err != nil {
		return 0, fmt.Errorf("run M2M100 decoder: %w", err)
	}
	defer func() {
		for _, value := range outputs {
			if value != nil {
				_ = value.Destroy()
			}
		}
	}()
	logits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return 0, errors.New("M2M100 decoder returned unexpected logits")
	}
	next, err := argmaxM2M(logits.GetData())
	if err != nil {
		return 0, err
	}
	return next, nil
}

func (m *M2M100) Close() error {
	if m.decoder != nil {
		_ = m.decoder.Destroy()
	}
	if m.encoder != nil {
		return m.encoder.Destroy()
	}
	return nil
}

func argmaxM2M(logits []float32) (int, error) {
	if len(logits) < m2mVocabulary {
		return 0, fmt.Errorf("M2M100 logits have %d values, want at least %d", len(logits), m2mVocabulary)
	}
	start := len(logits) - m2mVocabulary
	best := start
	for i := start + 1; i < len(logits); i++ {
		if logits[i] > logits[best] {
			best = i
		}
	}
	return best - start, nil
}
