package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
	"go.yaml.in/yaml/v2"
	_ "golang.org/x/image/webp"
)

type PPConfig struct {
	ModelID        string
	DetectorPath   string
	RecognizerPath string
	RecognizerYAML string
	MaxSide        int
}

type PP struct {
	modelID                           string
	detector                          *ort.DynamicAdvancedSession
	recognizer                        *ort.DynamicAdvancedSession
	detectorInput, detectorOutput     string
	recognizerInput, recognizerOutput string
	characters                        []string
	maxSide                           int
	sem                               chan struct{}
	closeOnce                         sync.Once
}

type inferenceConfig struct {
	PostProcess struct {
		CharacterDict []string `yaml:"character_dict"`
	} `yaml:"PostProcess"`
}

func NewPP(config PPConfig) (*PP, error) {
	if config.ModelID == "" {
		return nil, errors.New("OCR model ID is required")
	}
	if config.MaxSide < 32 {
		return nil, errors.New("OCR maximum image side must be at least 32")
	}
	for name, path := range map[string]string{
		"detector": config.DetectorPath, "recognizer": config.RecognizerPath, "recognizer configuration": config.RecognizerYAML,
	} {
		if path == "" {
			return nil, fmt.Errorf("OCR %s path is required", name)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("stat OCR %s: %w", name, err)
		}
	}
	characters, err := readCharacters(config.RecognizerYAML)
	if err != nil {
		return nil, err
	}
	detector, detectorInput, detectorOutput, err := newSession(config.DetectorPath)
	if err != nil {
		return nil, fmt.Errorf("create OCR detector session: %w", err)
	}
	recognizer, recognizerInput, recognizerOutput, err := newSession(config.RecognizerPath)
	if err != nil {
		_ = detector.Destroy()
		return nil, fmt.Errorf("create OCR recognizer session: %w", err)
	}
	return &PP{
		modelID: config.ModelID, detector: detector, recognizer: recognizer,
		detectorInput: detectorInput, detectorOutput: detectorOutput,
		recognizerInput: recognizerInput, recognizerOutput: recognizerOutput,
		characters: characters, maxSide: config.MaxSide, sem: make(chan struct{}, 1),
	}, nil
}

func newSession(path string) (*ort.DynamicAdvancedSession, string, string, error) {
	inputs, outputs, err := ort.GetInputOutputInfo(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("inspect ONNX model: %w", err)
	}
	if len(inputs) != 1 || len(outputs) != 1 {
		return nil, "", "", fmt.Errorf("expected one input and one output, got %d and %d", len(inputs), len(outputs))
	}
	session, err := ort.NewDynamicAdvancedSession(path, []string{inputs[0].Name}, []string{outputs[0].Name}, nil)
	if err != nil {
		return nil, "", "", err
	}
	return session, inputs[0].Name, outputs[0].Name, nil
}

func readCharacters(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OCR recognizer configuration: %w", err)
	}
	var config inferenceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse OCR recognizer configuration: %w", err)
	}
	if len(config.PostProcess.CharacterDict) == 0 {
		return nil, errors.New("OCR recognizer configuration has no character dictionary")
	}
	// PP-OCRv6 multilingual recognition reserves a CTC class for a literal
	// space in addition to the YAML character dictionary.
	config.PostProcess.CharacterDict = append(config.PostProcess.CharacterDict, " ")
	return config.PostProcess.CharacterDict, nil
}

func (p *PP) Recognize(ctx context.Context, data []byte, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if len(data) == 0 {
		return Result{}, errors.New("empty image")
	}
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	imageValue, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("decode OCR image: %w", err)
	}
	start := time.Now()
	boxes, err := p.detect(imageValue)
	if err != nil {
		return Result{}, err
	}
	blocks := make([]Block, 0, len(boxes))
	for _, box := range boxes {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		text, confidence, err := p.recognize(imageValue, box)
		if err != nil {
			return Result{}, err
		}
		if text == "" {
			continue
		}
		blocks = append(blocks, Block{Text: text, Confidence: confidence, Box: box})
	}
	sortBlocks(blocks)
	texts := make([]string, 0, len(blocks))
	for index := range blocks {
		blocks[index].Order = index
		texts = append(texts, blocks[index].Text)
	}
	return Result{
		Text: strings.Join(texts, "\n"), Model: p.modelID, Language: request.Language,
		ProcessingTimeMS: time.Since(start).Milliseconds(), Blocks: blocks,
		Metadata: map[string]any{"engine": "onnxruntime", "coordinates": "absolute_pixels", "detector": "PP-OCRv6 DB", "recognizer": "PP-OCRv6 CTC"},
	}, nil
}

func (p *PP) detect(input image.Image) ([]Box, error) {
	resized, scaleX, scaleY := resizeDetector(input, p.maxSide)
	bounds := resized.Bounds()
	shape := ort.Shape{1, 3, int64(bounds.Dy()), int64(bounds.Dx())}
	tensor, err := ort.NewTensor(shape, normalizeBGR(resized))
	if err != nil {
		return nil, fmt.Errorf("create OCR detector tensor: %w", err)
	}
	defer tensor.Destroy()
	outputs := []ort.Value{nil}
	if err := p.detector.Run([]ort.Value{tensor}, outputs); err != nil {
		return nil, fmt.Errorf("run OCR detector: %w", err)
	}
	output, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		if outputs[0] != nil {
			_ = outputs[0].Destroy()
		}
		return nil, errors.New("OCR detector output is not a float32 tensor")
	}
	defer output.Destroy()
	return detectorBoxes(output.GetData(), output.GetShape(), input.Bounds(), scaleX, scaleY), nil
}

func (p *PP) recognize(input image.Image, box Box) (string, float64, error) {
	crop := imaging.Crop(input, image.Rect(box.X, box.Y, box.X+box.Width, box.Y+box.Height))
	resized := resizeRecognizer(crop)
	bounds := resized.Bounds()
	shape := ort.Shape{1, 3, int64(bounds.Dy()), int64(bounds.Dx())}
	tensor, err := ort.NewTensor(shape, normalizeRecognizer(resized))
	if err != nil {
		return "", 0, fmt.Errorf("create OCR recognizer tensor: %w", err)
	}
	defer tensor.Destroy()
	outputs := []ort.Value{nil}
	if err := p.recognizer.Run([]ort.Value{tensor}, outputs); err != nil {
		return "", 0, fmt.Errorf("run OCR recognizer: %w", err)
	}
	output, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		if outputs[0] != nil {
			_ = outputs[0].Destroy()
		}
		return "", 0, errors.New("OCR recognizer output is not a float32 tensor")
	}
	defer output.Destroy()
	return decodeCTC(output.GetData(), output.GetShape(), p.characters)
}

func (p *PP) Close() error {
	p.closeOnce.Do(func() { _ = p.detector.Destroy(); _ = p.recognizer.Destroy() })
	return nil
}

func resizeDetector(input image.Image, maxSide int) (image.Image, float64, float64) {
	bounds := input.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scale := 1.0
	if longest := max(width, height); longest > maxSide {
		scale = float64(maxSide) / float64(longest)
	}
	newWidth := max(32, int(math.Round(float64(width)*scale)))
	newHeight := max(32, int(math.Round(float64(height)*scale)))
	newWidth = int(math.Ceil(float64(newWidth)/32) * 32)
	newHeight = int(math.Ceil(float64(newHeight)/32) * 32)
	return imaging.Resize(input, newWidth, newHeight, imaging.Lanczos), float64(newWidth) / float64(width), float64(newHeight) / float64(height)
}

func resizeRecognizer(input image.Image) image.Image {
	bounds := input.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	newWidth := max(1, min(320, int(math.Ceil(float64(width)*48/float64(max(1, height))))))
	return imaging.Resize(input, newWidth, 48, imaging.Lanczos)
}

func normalizeBGR(input image.Image) []float32 {
	bounds := input.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	data := make([]float32, 3*width*height)
	means := [3]float32{0.485, 0.456, 0.406}
	std := [3]float32{0.229, 0.224, 0.225}
	for y := range height {
		for x := range width {
			r, g, b, _ := input.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			index := y*width + x
			values := [3]float32{float32(b>>8) / 255, float32(g>>8) / 255, float32(r>>8) / 255}
			for channel := range 3 {
				data[channel*width*height+index] = (values[channel] - means[channel]) / std[channel]
			}
		}
	}
	return data
}

func normalizeRecognizer(input image.Image) []float32 {
	bounds := input.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	data := make([]float32, 3*width*height)
	for y := range height {
		for x := range width {
			r, g, b, _ := input.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			index := y*width + x
			values := [3]float32{float32(b>>8) / 255, float32(g>>8) / 255, float32(r>>8) / 255}
			for channel := range 3 {
				data[channel*width*height+index] = (values[channel] - 0.5) / 0.5
			}
		}
	}
	return data
}

func detectorBoxes(values []float32, shape ort.Shape, original image.Rectangle, scaleX, scaleY float64) []Box {
	if len(shape) != 4 || shape[0] != 1 || shape[1] != 1 || shape[2] < 1 || shape[3] < 1 {
		return []Box{}
	}
	height, width := int(shape[2]), int(shape[3])
	if len(values) != height*width {
		return []Box{}
	}
	visited := make([]bool, len(values))
	boxes := make([]Box, 0)
	for start, score := range values {
		if visited[start] || score < 0.2 {
			continue
		}
		box, average, pixels := connectedComponent(values, width, height, start, visited)
		if pixels < 9 || average < 0.4 {
			continue
		}
		box = expandBox(box, 1.4, width, height)
		mapped := Box{
			X:      max(0, int(math.Floor(float64(box.X)/scaleX))),
			Y:      max(0, int(math.Floor(float64(box.Y)/scaleY))),
			Width:  max(1, int(math.Ceil(float64(box.Width)/scaleX))),
			Height: max(1, int(math.Ceil(float64(box.Height)/scaleY))),
		}
		if mapped.X+mapped.Width > original.Dx() {
			mapped.Width = original.Dx() - mapped.X
		}
		if mapped.Y+mapped.Height > original.Dy() {
			mapped.Height = original.Dy() - mapped.Y
		}
		if mapped.Width > 0 && mapped.Height > 0 {
			boxes = append(boxes, mapped)
		}
	}
	return boxes
}

func connectedComponent(values []float32, width, height, start int, visited []bool) (Box, float64, int) {
	queue := make([]int, 0, 32)
	queue = append(queue, start)
	visited[start] = true
	minX, maxX, minY, maxY := width, 0, height, 0
	var sum float64
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		x, y := index%width, index/width
		minX, maxX, minY, maxY = min(minX, x), max(maxX, x), min(minY, y), max(maxY, y)
		sum += float64(values[index])
		for deltaY := -1; deltaY <= 1; deltaY++ {
			for deltaX := -1; deltaX <= 1; deltaX++ {
				if deltaX == 0 && deltaY == 0 {
					continue
				}
				nx, ny := x+deltaX, y+deltaY
				if nx < 0 || nx >= width || ny < 0 || ny >= height {
					continue
				}
				next := ny*width + nx
				if visited[next] || values[next] < 0.2 {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return Box{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1}, sum / float64(len(queue)), len(queue)
}

func expandBox(box Box, ratio float64, maxWidth, maxHeight int) Box {
	centerX, centerY := float64(box.X)+float64(box.Width)/2, float64(box.Y)+float64(box.Height)/2
	width, height := float64(box.Width)*ratio, float64(box.Height)*ratio
	left, top := max(0, int(math.Floor(centerX-width/2))), max(0, int(math.Floor(centerY-height/2)))
	right, bottom := min(maxWidth, int(math.Ceil(centerX+width/2))), min(maxHeight, int(math.Ceil(centerY+height/2)))
	return Box{X: left, Y: top, Width: max(1, right-left), Height: max(1, bottom-top)}
}

func decodeCTC(values []float32, shape ort.Shape, characters []string) (string, float64, error) {
	if len(shape) != 3 || shape[0] != 1 {
		return "", 0, fmt.Errorf("unexpected CTC output shape %v", shape)
	}
	steps, classes := int(shape[1]), int(shape[2])
	transpose := false
	if classes != len(characters)+1 && int(shape[1]) == len(characters)+1 {
		steps, classes, transpose = int(shape[2]), int(shape[1]), true
	}
	if classes != len(characters)+1 || len(values) != steps*classes {
		return "", 0, fmt.Errorf("CTC output class count %d does not match dictionary length %d", classes, len(characters))
	}
	var builder strings.Builder
	previous, confidenceSum, confidenceCount := 0, float64(0), 0
	for step := range steps {
		bestClass, bestScore := 0, values[step*classes]
		for class := 1; class < classes; class++ {
			index := step*classes + class
			if transpose {
				index = class*steps + step
			}
			if values[index] > bestScore {
				bestClass, bestScore = class, values[index]
			}
		}
		if bestClass != 0 && bestClass != previous {
			builder.WriteString(characters[bestClass-1])
			confidenceSum += float64(bestScore)
			confidenceCount++
		}
		previous = bestClass
	}
	if confidenceCount == 0 {
		return "", 0, nil
	}
	return builder.String(), confidenceSum / float64(confidenceCount), nil
}

func sortBlocks(blocks []Block) {
	sort.SliceStable(blocks, func(left, right int) bool {
		verticalDistance := abs(blocks[left].Box.Y - blocks[right].Box.Y)
		lineTolerance := max(blocks[left].Box.Height, blocks[right].Box.Height) / 2
		if verticalDistance <= lineTolerance {
			return blocks[left].Box.X < blocks[right].Box.X
		}
		return blocks[left].Box.Y < blocks[right].Box.Y
	})
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
