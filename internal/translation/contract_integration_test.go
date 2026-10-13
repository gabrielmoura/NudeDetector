//go:build integration

package translation

import (
	"os"
	"strings"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestOPUSMTTranslatesEnglishToPortuguese(t *testing.T) {
	dir := os.Getenv("OPUS_DIR")
	if dir == "" || os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH") == "" {
		t.Skip("set OPUS_DIR and ONNXRUNTIME_SHARED_LIBRARY_PATH")
	}
	ort.SetSharedLibraryPath(os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"))
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()
	translator, err := NewOPUS(OPUSConfig{
		ModelID: "opus-mt-en-romance", EncoderPath: dir + "/encoder.onnx", DecoderPath: dir + "/decoder.onnx",
		TokenizerPath: dir + "/tokenizer.json", VocabularyPath: dir + "/vocab.json",
		SupportedSources: []string{"en"}, SupportedTargets: []string{"pt"}, MaxInputTokens: 64, MaxOutputTokens: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer translator.Close()
	result, err := translator.Translate(t.Context(), Request{Text: "Hello world", SourceLanguage: "en", TargetLanguage: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(result.TranslatedText, "Olá mundo") {
		t.Fatalf("translation = %q, want %q", result.TranslatedText, "Olá mundo")
	}
}
