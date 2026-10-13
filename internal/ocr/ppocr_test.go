package ocr

import (
	"image"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestDecodeCTCRemovesBlanksAndDuplicates(t *testing.T) {
	t.Parallel()

	text, confidence, err := decodeCTC(
		[]float32{
			0.9, 0.1, 0.0,
			0.1, 0.8, 0.1,
			0.2, 0.7, 0.1,
			0.1, 0.1, 0.8,
			0.8, 0.1, 0.1,
			0.1, 0.1, 0.9,
		},
		ort.Shape{1, 6, 3},
		[]string{"A", "B"},
	)
	if err != nil {
		t.Fatalf("decodeCTC() error = %v", err)
	}
	if text != "ABB" || confidence < 0.8 {
		t.Fatalf("decodeCTC() = %q, %f", text, confidence)
	}
}

func TestDetectorBoxesReturnsScaledBoxes(t *testing.T) {
	t.Parallel()

	values := make([]float32, 16)
	for index := range values {
		values[index] = 0.9
	}
	boxes := detectorBoxes(values, ort.Shape{1, 1, 4, 4}, imageRect(32, 32), 0.125, 0.125)
	if len(boxes) != 1 || boxes[0].Width != 32 || boxes[0].Height != 32 {
		t.Fatalf("detectorBoxes() = %#v", boxes)
	}
}

func imageRect(width, height int) image.Rectangle { return image.Rect(0, 0, width, height) }
