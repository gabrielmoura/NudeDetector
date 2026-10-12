package imageprocessor

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/disintegration/imaging"
)

type Processor struct {
	width  int
	height int
}

func New(width, height int) *Processor {
	if width <= 0 {
		width = 224
	}
	if height <= 0 {
		height = 224
	}
	return &Processor{width: width, height: height}
}

func (p *Processor) Process(data []byte) ([]float32, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image data")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	resized := imaging.Resize(img, p.width, p.height, imaging.Lanczos)
	bounds := resized.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	tensor := make([]float32, 1*3*h*w)
	idx := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := resized.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			tensor[idx] = float32(r>>8) / 255.0
			tensor[idx+h*w] = float32(g>>8) / 255.0
			tensor[idx+2*h*w] = float32(b>>8) / 255.0
			idx++
		}
	}
	return tensor, nil
}

func DecodeConfig(data []byte) (int, int, error) {
	c, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return c.Width, c.Height, nil
}
