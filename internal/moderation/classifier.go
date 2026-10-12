package moderation

import (
	"errors"
	"fmt"

	"github.com/gabrielmoura/NudeDetector/internal/model"
)

type classifier struct {
	meta *model.ModelMetadata
}

func NewClassifier(m *model.ModelMetadata) (Classifier, error) {
	if m == nil {
		return nil, errors.New("nil metadata")
	}
	if len(m.Labels) == 0 {
		return nil, errors.New("model metadata does not define labels")
	}
	if _, err := m.GetPornIndex(); err != nil {
		return nil, err
	}
	for _, label := range []string{"drawings", "hentai", "neutral", "porn", "sexy"} {
		if !contains(m.Labels, label) {
			return nil, fmt.Errorf("model metadata is missing label %q", label)
		}
	}
	return &classifier{meta: m}, nil
}

func (c *classifier) Classify(probabilities []float32) (*ClassificationScores, error) {
	if len(probabilities) != len(c.meta.Labels) {
		return nil, fmt.Errorf(
			"model output has %d classes, metadata defines %d labels",
			len(probabilities),
			len(c.meta.Labels),
		)
	}
	s := ClassificationScores{}
	for index, label := range c.meta.Labels {
		value := float64(probabilities[index])
		switch label {
		case "drawings":
			s.Drawings = value
		case "hentai":
			s.Hentai = value
		case "neutral":
			s.Neutral = value
		case "porn":
			s.Porn = value
		case "sexy":
			s.Sexy = value
		}
	}
	return &s, nil
}

func contains(labels []string, wanted string) bool {
	for _, label := range labels {
		if label == wanted {
			return true
		}
	}
	return false
}
