package inference

import (
	"math"
	"testing"
)

func TestStableSoftmaxProducesNormalizedProbabilities(t *testing.T) {
	t.Parallel()

	probabilities := StableSoftmax([]float32{1000, 999, 998})
	if len(probabilities) != 3 {
		t.Fatalf("probability count = %d, want 3", len(probabilities))
	}

	var sum float64
	for _, probability := range probabilities {
		if probability <= 0 || math.IsNaN(float64(probability)) || math.IsInf(float64(probability), 0) {
			t.Fatalf("invalid probability %f", probability)
		}
		sum += float64(probability)
	}
	if math.Abs(sum-1) > 0.000001 {
		t.Fatalf("probability sum = %.9f, want 1", sum)
	}
	if probabilities[0] <= probabilities[1] || probabilities[1] <= probabilities[2] {
		t.Fatalf("probabilities are not ordered by logits: %v", probabilities)
	}
}
