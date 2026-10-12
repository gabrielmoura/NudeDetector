package inference

import (
	"math"
)

func StableSoftmax(logits []float32) []float32 {
	if len(logits) == 0 {
		return logits
	}
	// find max
	maxv := logits[0]
	for i := 1; i < len(logits); i++ {
		if logits[i] > maxv {
			maxv = logits[i]
		}
	}
	// exp and sum
	sum := float64(0)
	out := make([]float32, len(logits))
	for i := 0; i < len(logits); i++ {
		e := math.Exp(float64(logits[i]) - float64(maxv))
		sum += e
		out[i] = float32(e)
	}
	// normalize
	if sum == 0 {
		return out
	}
	inv := 1.0 / sum
	for i := 0; i < len(logits); i++ {
		out[i] = out[i] * float32(inv)
	}
	return out
}
