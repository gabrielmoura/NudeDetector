package imageprocessor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/model"
)

func TestProcessAppliesModelRescaleAndNormalization(t *testing.T) {
	t.Parallel()

	processor, err := New(&model.ModelMetadata{
		Width:  1,
		Height: 1,
		Preprocess: &model.PreprocessConfig{
			DoRescale:   true,
			Rescale:     1.0 / 255.0,
			DoNormalize: true,
			ImageMean:   []float32{0.5, 0.25, 0.0},
			ImageStd:    []float32{0.5, 0.25, 1.0},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	imageValue := image.NewRGBA(image.Rect(0, 0, 1, 1))
	imageValue.SetRGBA(0, 0, color.RGBA{R: 255, G: 64, B: 128, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageValue); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	got, err := processor.Process(encoded.Bytes())
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	want := []float32{1, (float32(64)/255 - 0.25) / 0.25, float32(128) / 255}
	for index := range want {
		if math.Abs(float64(got[index]-want[index])) > 0.0001 {
			t.Fatalf("tensor[%d] = %f, want %f", index, got[index], want[index])
		}
	}
}
