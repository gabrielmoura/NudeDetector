package inference

import (
	"context"
	"errors"
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

type ONNX struct {
	session *ort.DynamicAdvancedSession
	shape   ort.Shape
}

func NewONNX(modelPath string, intra, inter int) (*ONNX, error) {
	if modelPath == "" {
		return nil, errors.New("empty model path")
	}

	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect model inputs and outputs: %w", err)
	}
	if len(inputs) != 1 || len(outputs) != 1 {
		return nil, fmt.Errorf(
			"expected one model input and output, got %d and %d",
			len(inputs),
			len(outputs),
		)
	}
	if len(inputs[0].Dimensions) != 4 {
		return nil, fmt.Errorf(
			"expected a four-dimensional model input, got %v",
			inputs[0].Dimensions,
		)
	}

	shape := ort.Shape{1, 3, 224, 224}
	for index := range shape {
		modelDimension := inputs[0].Dimensions[index]
		if modelDimension == 0 {
			continue
		}
		if modelDimension < 0 {
			continue
		}
		shape[index] = modelDimension
	}

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{inputs[0].Name},
		[]string{outputs[0].Name},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &ONNX{session: session, shape: shape}, nil
}

func (o *ONNX) InputShape() []int64 {
	shape := make([]int64, len(o.shape))
	copy(shape, o.shape)
	return shape
}

func (o *ONNX) Close() error {
	return o.session.Destroy()
}

func (o *ONNX) Infer(ctx context.Context, tensor []float32) (Logits, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	input, err := ort.NewTensor(o.shape, tensor)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer input.Destroy()

	outputs := []ort.Value{nil}
	if err := o.session.Run([]ort.Value{input}, outputs); err != nil {
		return nil, fmt.Errorf("run ONNX session: %w", err)
	}
	output, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		if outputs[0] != nil {
			outputs[0].Destroy()
		}
		return nil, errors.New("ONNX output is not a float32 tensor")
	}
	defer output.Destroy()

	logits := make(Logits, len(output.GetData()))
	copy(logits, output.GetData())
	return logits, nil
}
