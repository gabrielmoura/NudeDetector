package detector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"sort"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

var labels = []string{
	"FEMALE_GENITALIA_COVERED", "FACE_FEMALE", "BUTTOCKS_EXPOSED",
	"FEMALE_BREAST_EXPOSED", "FEMALE_GENITALIA_EXPOSED", "MALE_BREAST_EXPOSED",
	"ANUS_EXPOSED", "FEET_EXPOSED", "BELLY_COVERED", "FEET_COVERED",
	"ARMPITS_COVERED", "ARMPITS_EXPOSED", "FACE_MALE", "BELLY_EXPOSED",
	"MALE_GENITALIA_EXPOSED", "ANUS_COVERED", "FEMALE_BREAST_COVERED", "BUTTOCKS_COVERED",
}

type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"width"`
	H int `json:"height"`
}

type Detection struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
	Box   Box     `json:"box"`
}

type Detector struct {
	session        *ort.DynamicAdvancedSession
	inputName      string
	outputName     string
	shape          ort.Shape
	scoreThreshold float32
	iouThreshold   float32
}

func New(modelPath string, scoreThreshold, iouThreshold float32) (*Detector, error) {
	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect detector model: %w", err)
	}
	if len(inputs) != 1 || len(outputs) != 1 {
		return nil, errors.New("detector model must have one input and one output")
	}
	shape := ort.Shape{1, 3, 320, 320}
	if len(inputs[0].Dimensions) != len(shape) {
		return nil, fmt.Errorf("unsupported detector input shape %v", inputs[0].Dimensions)
	}
	for index, dimension := range inputs[0].Dimensions {
		if dimension > 0 {
			shape[index] = dimension
		}
	}
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{inputs[0].Name},
		[]string{outputs[0].Name},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create detector session: %w", err)
	}
	return &Detector{
		session:        session,
		inputName:      inputs[0].Name,
		outputName:     outputs[0].Name,
		shape:          shape,
		scoreThreshold: scoreThreshold,
		iouThreshold:   iouThreshold,
	}, nil
}

func (d *Detector) Close() error { return d.session.Destroy() }

func (d *Detector) Detect(ctx context.Context, data []byte) ([]Detection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	imageValue, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image for detection: %w", err)
	}
	bounds := imageValue.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	maxSide := max(width, height)
	padded := image.NewRGBA(image.Rect(0, 0, maxSide, maxSide))
	draw.Draw(padded, image.Rect(0, 0, width, height), imageValue, bounds.Min, draw.Src)
	resized := imaging.Resize(padded, int(d.shape[3]), int(d.shape[2]), imaging.Lanczos)
	tensor := nchw(resized)

	input, err := ort.NewTensor(d.shape, tensor)
	if err != nil {
		return nil, fmt.Errorf("create detector input: %w", err)
	}
	defer input.Destroy()
	outputs := []ort.Value{nil}
	if err := d.session.Run([]ort.Value{input}, outputs); err != nil {
		return nil, fmt.Errorf("run detector: %w", err)
	}
	output, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		if outputs[0] != nil {
			outputs[0].Destroy()
		}
		return nil, errors.New("detector output is not a float32 tensor")
	}
	defer output.Destroy()

	return d.postprocess(output.GetData(), width, height, maxSide), nil
}

func (d *Detector) postprocess(values []float32, width, height, maxSide int) []Detection {
	const attributes = 22
	if len(values)%attributes != 0 {
		return []Detection{}
	}
	rows := len(values) / attributes
	detections := make([]Detection, 0, rows)
	for row := range rows {
		classID, score := bestClass(values, rows, row)
		if score < d.scoreThreshold || classID >= len(labels) {
			continue
		}
		scale := float32(maxSide) / float32(d.shape[2])
		x := (values[row] - values[2*rows+row]/2) * scale
		y := (values[rows+row] - values[3*rows+row]/2) * scale
		w := values[2*rows+row] * scale
		h := values[3*rows+row] * scale
		box := clipBox(x, y, w, h, width, height)
		if box.W == 0 || box.H == 0 {
			continue
		}
		detections = append(detections, Detection{Label: labels[classID], Score: float64(score), Box: box})
	}
	sort.Slice(detections, func(left, right int) bool { return detections[left].Score > detections[right].Score })
	return nms(detections, d.iouThreshold)
}

func bestClass(values []float32, rows, row int) (int, float32) {
	classID, score := 0, float32(0)
	for index := range labels {
		candidate := values[(4+index)*rows+row]
		if candidate > score {
			classID, score = index, candidate
		}
	}
	return classID, score
}

func clipBox(x, y, width, height float32, maxWidth, maxHeight int) Box {
	left := max(0, int(math.Round(float64(x))))
	top := max(0, int(math.Round(float64(y))))
	right := min(maxWidth, int(math.Round(float64(x+width))))
	bottom := min(maxHeight, int(math.Round(float64(y+height))))
	return Box{X: left, Y: top, W: max(0, right-left), H: max(0, bottom-top)}
}

func nms(candidates []Detection, threshold float32) []Detection {
	accepted := make([]Detection, 0, len(candidates))
	for _, candidate := range candidates {
		keep := true
		for _, existing := range accepted {
			if candidate.Label == existing.Label && iou(candidate.Box, existing.Box) > float64(threshold) {
				keep = false
				break
			}
		}
		if keep {
			accepted = append(accepted, candidate)
		}
	}
	return accepted
}

func iou(left, right Box) float64 {
	intersectionWidth := max(0, min(left.X+left.W, right.X+right.W)-max(left.X, right.X))
	intersectionHeight := max(0, min(left.Y+left.H, right.Y+right.H)-max(left.Y, right.Y))
	intersection := intersectionWidth * intersectionHeight
	union := left.W*left.H + right.W*right.H - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func nchw(input image.Image) []float32 {
	bounds := input.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	data := make([]float32, 3*width*height)
	for y := range height {
		for x := range width {
			r, g, b, _ := input.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			index := y*width + x
			data[index] = float32(r>>8) / 255
			data[width*height+index] = float32(g>>8) / 255
			data[2*width*height+index] = float32(b>>8) / 255
		}
	}
	return data
}
