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
	"github.com/gabrielmoura/NudeDetector/internal/model"
)

type Processor struct {
	width       int
	height      int
	rescale     float32
	normalize   bool
	imageMean   [3]float32
	imageStdDev [3]float32
}

func New(metadata *model.ModelMetadata) (*Processor, error) {
	if metadata == nil {
		return nil, errors.New("model metadata is required")
	}
	if metadata.Width <= 0 || metadata.Height <= 0 {
		return nil, errors.New("model image dimensions must be positive")
	}

	processor := &Processor{
		width:   metadata.Width,
		height:  metadata.Height,
		rescale: 1.0 / 255.0,
	}
	if metadata.Preprocess == nil {
		return processor, nil
	}
	if metadata.Preprocess.DoRescale {
		if metadata.Preprocess.Rescale <= 0 {
			return nil, errors.New("model rescale factor must be positive")
		}
		processor.rescale = metadata.Preprocess.Rescale
	}
	if !metadata.Preprocess.DoNormalize {
		return processor, nil
	}
	if len(metadata.Preprocess.ImageMean) != 3 || len(metadata.Preprocess.ImageStd) != 3 {
		return nil, errors.New("model normalization must define three mean and std values")
	}
	for channel := range 3 {
		if metadata.Preprocess.ImageStd[channel] == 0 {
			return nil, errors.New("model normalization std must not contain zero")
		}
		processor.imageMean[channel] = metadata.Preprocess.ImageMean[channel]
		processor.imageStdDev[channel] = metadata.Preprocess.ImageStd[channel]
	}
	processor.normalize = true
	return processor, nil
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
			red := float32(r>>8) * p.rescale
			green := float32(g>>8) * p.rescale
			blue := float32(b>>8) * p.rescale
			if p.normalize {
				red = (red - p.imageMean[0]) / p.imageStdDev[0]
				green = (green - p.imageMean[1]) / p.imageStdDev[1]
				blue = (blue - p.imageMean[2]) / p.imageStdDev[2]
			}
			tensor[idx] = red
			tensor[idx+h*w] = green
			tensor[idx+2*h*w] = blue
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
