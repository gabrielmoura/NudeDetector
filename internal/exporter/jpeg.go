package exporter

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/gabrielmoura/NudeDetector/internal/detector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Options struct {
	DrawBoxes  bool
	Blur       bool
	BlurRadius float64
}

func Render(data []byte, detections []detector.Detection, options Options) ([]byte, error) {
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode export image: %w", err)
	}
	canvas := imaging.Clone(source)
	for _, detection := range detections {
		if options.Blur {
			blur(canvas, detection.Box, options.BlurRadius)
		}
		if options.DrawBoxes {
			drawBox(canvas, detection)
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("encode JPEG export: %w", err)
	}
	return output.Bytes(), nil
}

func blur(canvas *image.NRGBA, box detector.Box, radius float64) {
	if radius < 1 {
		radius = 1
	}
	region := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H).Intersect(canvas.Bounds())
	if region.Empty() {
		return
	}
	copyRegion := imaging.Crop(canvas, region)
	blurred := imaging.Blur(copyRegion, radius)
	draw.Draw(canvas, region, blurred, blurred.Bounds().Min, draw.Src)
}

func drawBox(canvas *image.NRGBA, detection detector.Detection) {
	box := detection.Box
	region := image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H).Intersect(canvas.Bounds())
	if region.Empty() {
		return
	}
	stroke := color.RGBA{R: 255, G: 64, B: 64, A: 255}
	for offset := range 3 {
		left := region.Min.X + offset
		top := region.Min.Y + offset
		right := region.Max.X - 1 - offset
		bottom := region.Max.Y - 1 - offset
		if left >= right || top >= bottom {
			break
		}
		for x := left; x <= right; x++ {
			canvas.Set(x, top, stroke)
			canvas.Set(x, bottom, stroke)
		}
		for y := top; y <= bottom; y++ {
			canvas.Set(left, y, stroke)
			canvas.Set(right, y, stroke)
		}
	}
	label := detection.Label + " " + strconv.Itoa(int(detection.Score*100+0.5)) + "%"
	drawer := font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(stroke),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(region.Min.X, max(basicfont.Face7x13.Metrics().Ascent.Ceil(), region.Min.Y-2)),
	}
	drawer.DrawString(label)
}
