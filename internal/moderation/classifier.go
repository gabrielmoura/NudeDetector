package moderation

import (
	"errors"

	"github.com/gabrielmoura/NudeDetector/internal/model"
)

type classifier struct {
	meta *model.ModelMetadata
}

func NewClassifier(m *model.ModelMetadata) (Classifier, error) {
	if m == nil {
		return nil, errors.New("nil metadata")
	}
	return &classifier{meta: m}, nil
}

func (c *classifier) Classify(logits []float32) (*ClassificationScores, error) {
	// softmax done by caller usually; we expect probs? but we do internal mapping
	// caller passes probabilities after softmax? We'll be flexible: if looks like logits sum>1.5 treat? not needed
	s := ClassificationScores{}
	// map by labels order (default)
	labels := []string{"drawings", "hentai", "neutral", "porn", "sexy"}
	if c.meta != nil && len(c.meta.Labels) == len(logits) {
		labels = c.meta.Labels
	}
	for i := range logits {
		if i >= len(labels) {
			continue
		}
		v := float64(logits[i])
		switch labels[i] {
		case "drawings":
			s.Drawings = v
		case "hentai":
			s.Hentai = v
		case "neutral":
			s.Neutral = v
		case "porn":
			s.Porn = v
		case "sexy":
			s.Sexy = v
		}
	}
	return &s, nil
}
