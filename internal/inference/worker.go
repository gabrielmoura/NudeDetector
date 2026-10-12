package inference

import (
	"context"
	"errors"
	"sync"
)

type Work struct {
	Tensor []float32
	Ctx    context.Context
	Result chan Result
}

type Result struct {
	Logits Logits
	Err    error
}

type Worker struct {
	engine Engine
	sem    chan struct{}
	queue  chan *Work
	mu     sync.Mutex
}

func NewWorker(engine Engine, concurrency, maxQueue int) *Worker {
	w := &Worker{
		engine: engine,
		sem:    make(chan struct{}, concurrency),
		queue:  make(chan *Work, maxQueue),
	}
	go w.loop()
	return w
}

func (w *Worker) loop() {
	for wk := range w.queue {
		select {
		case w.sem <- struct{}{}:
			go func(wk *Work) {
				defer func() { <-w.sem }()
				l, err := w.engine.Infer(wk.Ctx, wk.Tensor)
				select {
				case wk.Result <- Result{Logits: l, Err: err}:
				default:
				}
			}(wk)
		case <-wk.Ctx.Done():
			wk.Result <- Result{Err: wk.Ctx.Err()}
		}
	}
}

var ErrQueueFull = errors.New("inference queue full")

func (w *Worker) Infer(ctx context.Context, tensor []float32) (Logits, error) {
	resCh := make(chan Result, 1)
	wk := &Work{Tensor: tensor, Ctx: ctx, Result: resCh}
	select {
	case w.queue <- wk:
		select {
		case r := <-resCh:
			return r.Logits, r.Err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	default:
		return nil, ErrQueueFull
	}
}

func (w *Worker) QueueDepth() int {
	return len(w.queue)
}

func (w *Worker) Close() {
	close(w.queue)
}
