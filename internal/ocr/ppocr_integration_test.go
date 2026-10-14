//go:build integration

package ocr

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func TestPPTinyRecognizesRenderedEnglishText(t *testing.T) {
	modelDir := os.Getenv("OCR_MODEL_DIR")
	runtimeLibrary := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH")
	if modelDir == "" || runtimeLibrary == "" {
		t.Skip("set OCR_MODEL_DIR and ONNXRUNTIME_SHARED_LIBRARY_PATH for OCR integration")
	}
	modelID := os.Getenv("OCR_MODEL_ID")
	if modelID == "" {
		modelID = "pp-ocrv6-tiny"
	}
	ort.SetSharedLibraryPath(runtimeLibrary)
	if err := ort.InitializeEnvironment(); err != nil {
		t.Fatalf("InitializeEnvironment() error = %v", err)
	}
	defer ort.DestroyEnvironment()

	recognizer, err := NewPP(PPConfig{
		ModelID:        modelID,
		DetectorPath:   filepath.Join(modelDir, "det.onnx"),
		RecognizerPath: filepath.Join(modelDir, "rec.onnx"),
		RecognizerYAML: filepath.Join(modelDir, "rec.yml"),
		MaxSide:        960,
	})
	if err != nil {
		t.Fatalf("NewPP() error = %v", err)
	}
	defer recognizer.Close()

	result, err := recognizer.Recognize(t.Context(), renderedTextPNG(t, "HELLO OCR"), Request{Language: "en", ReturnBoxes: true})
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if !strings.Contains(strings.ToUpper(result.Text), "HELLO") {
		t.Fatalf("Recognize() text = %q, blocks = %#v", result.Text, result.Blocks)
	}
	if result.Model != modelID {
		t.Fatalf("Recognize() model = %q, want %q", result.Model, modelID)
	}
}

func renderedTextPNG(t *testing.T, text string) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 900, 220))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	fontValue, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	face, err := opentype.NewFace(fontValue, &opentype.FaceOptions{Size: 92, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		t.Fatalf("NewFace() error = %v", err)
	}
	defer face.Close()
	drawer := &font.Drawer{Dst: canvas, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(36, 145)}
	drawer.DrawString(text)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	return encoded.Bytes()
}
