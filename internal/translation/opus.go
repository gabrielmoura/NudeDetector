package translation

import (
	"context"
	"encoding/json"
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
	eosTokenID            = int64(0)
	padTokenID            = int64(65000)
	decoderVocabulary     = 65001
	decoderLayerCount     = 6
	decoderAttentionHeads = 8
	decoderHeadSize       = 64
)

// OPUSConfig defines the pinned local artifacts for an OPUS-MT ONNX model.
type OPUSConfig struct {
	ModelID          string
	EncoderPath      string
	DecoderPath      string
	TokenizerPath    string
	VocabularyPath   string
	SupportedSources []string
	SupportedTargets []string
	TargetPrefix     bool
	MaxInputTokens   int
	MaxOutputTokens  int
}

// OPUS runs Marian encoder/decoder ONNX graphs locally. Session use is
// serialized because the native session's concurrent execution guarantee is
// model-provider dependent and CPU memory use is intentionally bounded.
type OPUS struct {
	modelID          string
	encoder          *ort.DynamicAdvancedSession
	decoder          *ort.DynamicAdvancedSession
	tokenizer        *sp.Tokenizer
	targetTokenIDs   map[string]int64
	supportedSources map[string]struct{}
	supportedTargets map[string]struct{}
	maxInputTokens   int
	maxOutputTokens  int
	targetPrefix     bool
	mu               sync.Mutex
}

func NewOPUS(config OPUSConfig) (*OPUS, error) {
	if config.ModelID == "" {
		return nil, errors.New("translation model ID is required")
	}
	if config.MaxInputTokens < 1 || config.MaxOutputTokens < 1 {
		return nil, errors.New("translation token limits must be greater than zero")
	}
	for name, path := range map[string]string{"encoder": config.EncoderPath, "decoder": config.DecoderPath, "tokenizer": config.TokenizerPath, "vocabulary": config.VocabularyPath} {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("stat translation %s: %w", name, err)
		}
	}
	tokenizer, err := sp.NewTokenizerFromJSON(config.TokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("load translation tokenizer: %w", err)
	}
	vocabulary, err := loadVocabulary(config.VocabularyPath)
	if err != nil {
		return nil, err
	}
	encoder, err := ort.NewDynamicAdvancedSession(config.EncoderPath, []string{"input_ids", "attention_mask"}, []string{"last_hidden_state"}, nil)
	if err != nil {
		return nil, fmt.Errorf("create translation encoder session: %w", err)
	}
	decoderInputs, decoderOutputs, err := ort.GetInputOutputInfo(config.DecoderPath)
	if err != nil {
		_ = encoder.Destroy()
		return nil, fmt.Errorf("inspect translation decoder: %w", err)
	}
	inNames, outNames := make([]string, len(decoderInputs)), make([]string, len(decoderOutputs))
	for i := range decoderInputs {
		inNames[i] = decoderInputs[i].Name
	}
	for i := range decoderOutputs {
		outNames[i] = decoderOutputs[i].Name
	}
	decoder, err := ort.NewDynamicAdvancedSession(config.DecoderPath, inNames, outNames, nil)
	if err != nil {
		_ = encoder.Destroy()
		return nil, fmt.Errorf("create translation decoder session: %w", err)
	}
	return &OPUS{
		modelID: config.ModelID, encoder: encoder, decoder: decoder, tokenizer: tokenizer,
		targetTokenIDs: vocabulary, supportedSources: languageSet(config.SupportedSources), supportedTargets: languageSet(config.SupportedTargets),
		maxInputTokens: config.MaxInputTokens, maxOutputTokens: config.MaxOutputTokens,
		targetPrefix: config.TargetPrefix,
	}, nil
}

func (o *OPUS) Translate(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return Result{}, errors.New("translation text is required")
	}
	source, target := normalizeLanguage(request.SourceLanguage), normalizeLanguage(request.TargetLanguage)
	if source == "" || target == "" {
		return Result{}, errors.New("source and target languages are required")
	}
	if _, ok := o.supportedSources[source]; !ok {
		return Result{}, fmt.Errorf("model %q does not support source language %q", o.modelID, source)
	}
	if _, ok := o.supportedTargets[target]; !ok {
		return Result{}, fmt.Errorf("model %q does not support target language %q", o.modelID, target)
	}
	targetTokenID, hasTargetToken := o.targetTokenIDs[">>"+target+"<<"]
	if o.targetPrefix && !hasTargetToken {
		return Result{}, fmt.Errorf("model %q has no target token for %q", o.modelID, target)
	}
	encoded, err := o.tokenizer.Encode(text)
	if err != nil {
		return Result{}, fmt.Errorf("tokenize translation input: %w", err)
	}
	prefixTokens := 1
	if !o.targetPrefix {
		prefixTokens = 0
	}
	if len(encoded)+prefixTokens+1 > o.maxInputTokens {
		return Result{}, fmt.Errorf("translation input exceeds %d tokens", o.maxInputTokens)
	}
	inputIDs := make([]int64, 0, len(encoded)+prefixTokens+1)
	if o.targetPrefix {
		inputIDs = append(inputIDs, targetTokenID)
	}
	for _, id := range encoded {
		inputIDs = append(inputIDs, int64(id))
	}
	inputIDs = append(inputIDs, eosTokenID)

	o.mu.Lock()
	defer o.mu.Unlock()
	started := time.Now()
	generated, err := o.generate(ctx, inputIDs)
	if err != nil {
		return Result{}, err
	}
	translated, err := o.tokenizer.Decode(generated)
	if err != nil {
		return Result{}, fmt.Errorf("decode translation output: %w", err)
	}
	return Result{SourceText: request.Text, TranslatedText: strings.TrimSpace(translated), SourceLanguage: source, TargetLanguage: target, Model: o.modelID, ProcessingTimeMS: time.Since(started).Milliseconds(), Metadata: map[string]any{"engine": "onnxruntime", "architecture": "marian", "cache": false}}, nil
}

func (o *OPUS) generate(ctx context.Context, inputIDs []int64) ([]int, error) {
	input, err := ort.NewTensor(ort.Shape{1, int64(len(inputIDs))}, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create translation input tensor: %w", err)
	}
	defer input.Destroy()
	maskData := make([]int64, len(inputIDs))
	for i := range maskData {
		maskData[i] = 1
	}
	mask, err := ort.NewTensor(ort.Shape{1, int64(len(maskData))}, maskData)
	if err != nil {
		return nil, fmt.Errorf("create translation attention tensor: %w", err)
	}
	defer mask.Destroy()
	encoderOutputs := []ort.Value{nil}
	if err := o.encoder.Run([]ort.Value{input, mask}, encoderOutputs); err != nil {
		return nil, fmt.Errorf("run translation encoder: %w", err)
	}
	defer encoderOutputs[0].Destroy()
	hidden, ok := encoderOutputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("translation encoder returned an unexpected tensor")
	}
	generated := make([]int, 0, o.maxOutputTokens)
	for range o.maxOutputTokens {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		decoderIDs := make([]int64, 1, len(generated)+1)
		decoderIDs[0] = padTokenID
		for _, token := range generated {
			decoderIDs = append(decoderIDs, int64(token))
		}
		next, err := o.decodeStep(mask, hidden, decoderIDs)
		if err != nil {
			return nil, err
		}
		if next == int(eosTokenID) {
			return generated, nil
		}
		generated = append(generated, next)
	}
	return nil, fmt.Errorf("translation generation exceeded %d tokens", o.maxOutputTokens)
}

func (o *OPUS) decodeStep(mask ort.Value, hidden ort.Value, decoderIDs []int64) (int, error) {
	decoderInput, err := ort.NewTensor(ort.Shape{1, int64(len(decoderIDs))}, decoderIDs)
	if err != nil {
		return 0, fmt.Errorf("create translation decoder tensor: %w", err)
	}
	defer decoderInput.Destroy()
	inputs := []ort.Value{mask, decoderInput, hidden}
	caches := make([]*ort.Tensor[float32], 0, decoderLayerCount*4)
	for range decoderLayerCount * 4 {
		cache, cacheErr := ort.NewTensor(ort.Shape{1, decoderAttentionHeads, 0, decoderHeadSize}, []float32{})
		if cacheErr != nil {
			for _, value := range caches {
				_ = value.Destroy()
			}
			return 0, fmt.Errorf("create translation cache tensor: %w", cacheErr)
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
		return 0, fmt.Errorf("create translation cache selector: %w", err)
	}
	defer branch.Destroy()
	inputs = append(inputs, branch)
	outputs := make([]ort.Value, 25)
	if err := o.decoder.Run(inputs, outputs); err != nil {
		return 0, fmt.Errorf("run translation decoder: %w", err)
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
		return 0, errors.New("translation decoder returned an unexpected logits tensor")
	}
	return argmaxLastToken(logits.GetData()), nil
}

func (o *OPUS) Close() error {
	if o.decoder != nil {
		_ = o.decoder.Destroy()
	}
	if o.encoder != nil {
		return o.encoder.Destroy()
	}
	return nil
}

func loadVocabulary(path string) (map[string]int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read translation vocabulary: %w", err)
	}
	var vocabulary map[string]int64
	if err := json.Unmarshal(data, &vocabulary); err != nil {
		return nil, fmt.Errorf("parse translation vocabulary: %w", err)
	}
	if vocabulary["</s>"] != eosTokenID || vocabulary["<pad>"] != padTokenID {
		return nil, errors.New("translation vocabulary has unexpected special token IDs")
	}
	return vocabulary, nil
}

func argmaxLastToken(logits []float32) int {
	start := len(logits) - decoderVocabulary
	best := start
	for i := start + 1; i < len(logits); i++ {
		if logits[i] > logits[best] {
			best = i
		}
	}
	return best - start
}

func normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if index := strings.IndexByte(language, '-'); index >= 0 {
		language = language[:index]
	}
	return language
}
func languageSet(languages []string) map[string]struct{} {
	result := make(map[string]struct{}, len(languages))
	for _, language := range languages {
		if normalized := normalizeLanguage(language); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}
