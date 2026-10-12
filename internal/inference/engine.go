package inference

import "context"

type Logits []float32

type Engine interface {
	Infer(ctx context.Context, tensor []float32) (Logits, error)
	Close() error
	InputShape() []int64
}
