package model

import (
	"encoding/json"
	"fmt"
	"os"
)

type PreprocessConfig struct {
	ImageMean   []float32 `json:"image_mean,omitempty"`
	ImageStd    []float32 `json:"image_std,omitempty"`
	Rescale     float32   `json:"rescale_factor,omitempty"`
	DoNormalize bool      `json:"do_normalize,omitempty"`
	DoRescale   bool      `json:"do_rescale,omitempty"`
	Size        *Size     `json:"size,omitempty"`
	DoResize    bool      `json:"do_resize,omitempty"`
}

type Size struct {
	Height int `json:"height,omitempty"`
	Width  int `json:"width,omitempty"`
}

type ModelMetadata struct {
	Name       string            `json:"name,omitempty"`
	Version    string            `json:"version,omitempty"`
	Labels     []string          `json:"labels,omitempty"`
	IDToLabel  map[string]string `json:"id2label,omitempty"`
	LabelToID  map[string]int    `json:"label2id,omitempty"`
	PornLabel  string            `json:"porn_label,omitempty"`
	Width      int               `json:"width,omitempty"`
	Height     int               `json:"height,omitempty"`
	ImageSize  int               `json:"image_size,omitempty"`
	Preprocess *PreprocessConfig `json:"preprocess,omitempty"`
	InputName  string            `json:"input_name,omitempty"`
	OutputName string            `json:"output_name,omitempty"`
}

func LoadMetadata(path string) (*ModelMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var m ModelMetadata
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	// Infer sizes
	if m.Width == 0 && m.Height == 0 && m.ImageSize > 0 {
		m.Width, m.Height = m.ImageSize, m.ImageSize
	}
	if m.Width == 0 && m.Height == 0 && m.Preprocess != nil && m.Preprocess.Size != nil {
		m.Width, m.Height = m.Preprocess.Size.Width, m.Preprocess.Size.Height
	}
	if m.PornLabel == "" {
		m.PornLabel = "porn"
	}
	return &m, nil
}

func (m *ModelMetadata) GetPornIndex() (int, error) {
	if m.LabelToID != nil {
		if idx, ok := m.LabelToID[m.PornLabel]; ok {
			return idx, nil
		}
	}
	if m.Labels != nil {
		for i, l := range m.Labels {
			if l == m.PornLabel {
				return i, nil
			}
		}
	}
	return -1, fmt.Errorf("porn label %q not found", m.PornLabel)
}

func (m *ModelMetadata) GetLabel(index int) string {
	if m.Labels != nil && index >= 0 && index < len(m.Labels) {
		return m.Labels[index]
	}
	return ""
}
