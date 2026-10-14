//go:build integration

package translation

import (
	"os"
	"testing"

	sp "github.com/tggo/goSentencePiece"
	ort "github.com/yalue/onnxruntime_go"
)

func TestM2M100Contract(t *testing.T) {
	dir := os.Getenv("M2M100_DIR")
	if dir == "" || os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH") == "" {
		t.Skip("set M2M100_DIR and ONNXRUNTIME_SHARED_LIBRARY_PATH")
	}
	ort.SetSharedLibraryPath(os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"))
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()
	for _, path := range []string{dir + "/encoder.onnx", dir + "/decoder.onnx"} {
		inputs, outputs, err := ort.GetInputOutputInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s inputs=%+v outputs=%+v", path, inputs, outputs)
	}
}

func TestM2M100FirstGenerationStep(t *testing.T) {
	dir := os.Getenv("M2M100_DIR")
	if dir == "" || os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH") == "" {
		t.Skip("set M2M100_DIR and ONNXRUNTIME_SHARED_LIBRARY_PATH")
	}
	ort.SetSharedLibraryPath(os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"))
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()
	tokenizer, err := sp.NewTokenizerFromJSON(dir + "/tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := tokenizer.Encode("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	inputIDs := []int64{128022}
	for _, id := range encoded {
		inputIDs = append(inputIDs, int64(id))
	}
	inputIDs = append(inputIDs, 2)
	input, _ := ort.NewTensor(ort.Shape{1, int64(len(inputIDs))}, inputIDs)
	defer input.Destroy()
	maskData := make([]int64, len(inputIDs))
	for i := range maskData {
		maskData[i] = 1
	}
	mask, _ := ort.NewTensor(ort.Shape{1, int64(len(maskData))}, maskData)
	defer mask.Destroy()
	encoder, err := ort.NewDynamicAdvancedSession(dir+"/encoder.onnx", []string{"input_ids", "attention_mask"}, []string{"last_hidden_state"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Destroy()
	encOut := []ort.Value{nil}
	if err := encoder.Run([]ort.Value{input, mask}, encOut); err != nil {
		t.Fatal(err)
	}
	defer encOut[0].Destroy()
	inputs, outputs, err := ort.GetInputOutputInfo(dir + "/decoder.onnx")
	if err != nil {
		t.Fatal(err)
	}
	inNames, outNames := make([]string, len(inputs)), make([]string, len(outputs))
	for i := range inputs {
		inNames[i] = inputs[i].Name
	}
	for i := range outputs {
		outNames[i] = outputs[i].Name
	}
	decoder, err := ort.NewDynamicAdvancedSession(dir+"/decoder.onnx", inNames, outNames, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Destroy()
	decoderIDs, _ := ort.NewTensor(ort.Shape{1, 2}, []int64{2, 128075})
	defer decoderIDs.Destroy()
	values := []ort.Value{mask, decoderIDs, encOut[0]}
	caches := make([]*ort.Tensor[float32], 0, 48)
	for range 48 {
		cache, cacheErr := ort.NewTensor(ort.Shape{1, 16, 0, 64}, []float32{})
		if cacheErr != nil {
			t.Fatal(cacheErr)
		}
		caches = append(caches, cache)
		values = append(values, cache)
	}
	defer func() {
		for _, cache := range caches {
			_ = cache.Destroy()
		}
	}()
	branch, _ := ort.NewTensor(ort.Shape{1}, []bool{false})
	defer branch.Destroy()
	values = append(values, branch)
	result := make([]ort.Value, len(outNames))
	if err := decoder.Run(values, result); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, value := range result {
			if value != nil {
				_ = value.Destroy()
			}
		}
	}()
	logits := result[0].(*ort.Tensor[float32]).GetData()
	start := len(logits) - 128112
	best := start
	for i := start + 1; i < len(logits); i++ {
		if logits[i] > logits[best] {
			best = i
		}
	}
	nextToken := best - start
	decoded, err := tokenizer.Decode([]int{128075, nextToken})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("input=%v next_token=%d decoded=%q", inputIDs, nextToken, decoded)
}
