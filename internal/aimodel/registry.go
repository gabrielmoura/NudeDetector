// Package aimodel provides the static catalog and runtime selection of AI models.
package aimodel

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type Capability string

const (
	CapabilityModeration  Capability = "moderation"
	CapabilityDetection   Capability = "detection"
	CapabilityASR         Capability = "asr"
	CapabilityOCR         Capability = "ocr"
	CapabilityTranslation Capability = "translation"

	NSFWClassifierID  = "nsfw-classifier"
	NudeNetDetectorID = "nudenet-detector"
)

type State string

const (
	StateAvailable    State = "available"
	StateNotInstalled State = "not_installed"
	StateLoaded       State = "loaded"
)

type Descriptor struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Version           string       `json:"version"`
	Capabilities      []Capability `json:"capabilities"`
	Engine            string       `json:"engine"`
	Format            string       `json:"format"`
	Quantization      string       `json:"quantization,omitempty"`
	Languages         []string     `json:"languages"`
	EstimatedMemoryMB int          `json:"estimated_memory_mb"`
	License           string       `json:"license"`
}

type Model struct {
	Descriptor
	State   State `json:"state"`
	Enabled bool  `json:"enabled"`
}

type Registry struct {
	descriptors map[string]Descriptor
	paths       map[string][]string
	enabled     map[string]struct{}
	loaded      map[string]struct{}
}

func New(classifierPath, detectorPath string, enabled []string) (*Registry, error) {
	registry := &Registry{
		descriptors: defaultDescriptors(),
		paths: map[string][]string{
			NSFWClassifierID:  {classifierPath},
			NudeNetDetectorID: {detectorPath},
		},
		enabled: map[string]struct{}{},
		loaded:  map[string]struct{}{},
	}
	if len(enabled) == 0 {
		enabled = []string{NSFWClassifierID}
	}
	seen := make(map[string]struct{}, len(enabled))
	for _, id := range enabled {
		if _, ok := registry.descriptors[id]; !ok {
			return nil, fmt.Errorf("unknown enabled model %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("enabled model %q is duplicated", id)
		}
		seen[id] = struct{}{}
		registry.enabled[id] = struct{}{}
	}
	return registry, nil
}

func (r *Registry) Enabled(id string) bool {
	_, ok := r.enabled[id]
	return ok
}

func (r *Registry) EnabledIDs() []string {
	ids := make([]string, 0, len(r.enabled))
	for id := range r.enabled {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *Registry) MarkLoaded(id string) error {
	if !r.Enabled(id) {
		return fmt.Errorf("model %q is not enabled", id)
	}
	if _, ok := r.descriptors[id]; !ok {
		return fmt.Errorf("unknown model %q", id)
	}
	r.loaded[id] = struct{}{}
	return nil
}

// SetPaths replaces the local artifacts used to determine one model's
// installation state. Call it during composition, before serving requests.
func (r *Registry) SetPaths(id string, paths []string) error {
	if _, ok := r.descriptors[id]; !ok {
		return fmt.Errorf("unknown model %q", id)
	}
	r.paths[id] = append([]string(nil), paths...)
	return nil
}

func (r *Registry) List() []Model {
	ids := make([]string, 0, len(r.descriptors))
	for id := range r.descriptors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	models := make([]Model, 0, len(ids))
	for _, id := range ids {
		models = append(models, r.model(id))
	}
	return models
}

func (r *Registry) Get(id string) (Model, bool) {
	if _, ok := r.descriptors[id]; !ok {
		return Model{}, false
	}
	return r.model(id), true
}

func (r *Registry) model(id string) Model {
	_, enabled := r.enabled[id]
	_, loaded := r.loaded[id]
	state := StateNotInstalled
	if installed(r.paths[id]) {
		state = StateAvailable
	}
	if loaded {
		state = StateLoaded
	}
	return Model{Descriptor: r.descriptors[id], State: state, Enabled: enabled}
}

func installed(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if path == "" {
			return false
		}
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func ParseEnabled(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	parts := strings.Split(value, ",")
	models := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, fmt.Errorf("enabled models must not contain empty IDs")
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("enabled model %q is duplicated", id)
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	return models, nil
}

func defaultDescriptors() map[string]Descriptor {
	return map[string]Descriptor{
		NSFWClassifierID: {
			ID: NSFWClassifierID, Name: "NSFW Classifier", Version: "1.0.0",
			Capabilities: []Capability{CapabilityModeration}, Engine: "onnxruntime",
			Format: "onnx", Languages: []string{}, EstimatedMemoryMB: 90, License: "Apache-2.0",
		},
		NudeNetDetectorID: {
			ID: NudeNetDetectorID, Name: "NudeNet Detector", Version: "3",
			Capabilities: []Capability{CapabilityDetection}, Engine: "onnxruntime",
			Format: "onnx", Languages: []string{}, EstimatedMemoryMB: 80, License: "MIT",
		},
		"whisper-tiny": {
			ID: "whisper-tiny", Name: "Whisper Tiny", Version: "65176e2deb88badc814a94058666cadccc29b61c",
			Capabilities: []Capability{CapabilityASR}, Engine: "sherpa-onnx", Format: "onnx",
			Quantization: "int8 when installed", Languages: []string{"multilingual"}, EstimatedMemoryMB: 150, License: "MIT",
		},
		"whisper-base": {
			ID: "whisper-base", Name: "Whisper Base", Version: "31dc0cc3b59ba4a0b52941f1d6dce9c0e30311b4",
			Capabilities: []Capability{CapabilityASR}, Engine: "sherpa-onnx", Format: "onnx",
			Quantization: "int8 when installed", Languages: []string{"multilingual"}, EstimatedMemoryMB: 300, License: "MIT",
		},
		"whisper-small": {
			ID: "whisper-small", Name: "Whisper Small", Version: "8f3c18b358db4d1f2fc1eae49d75cd20989e4309",
			Capabilities: []Capability{CapabilityASR}, Engine: "sherpa-onnx", Format: "onnx",
			Quantization: "int8 when installed", Languages: []string{"multilingual"}, EstimatedMemoryMB: 700, License: "MIT",
		},
		"pp-ocrv6-tiny": {
			ID: "pp-ocrv6-tiny", Name: "PP-OCRv6 Tiny", Version: "det:2ba1506c0380b8f0b03dd142459aac66d4421f6c rec:2612ab37152ae0a677521bae4e1e3d4fb4cf7c30",
			Capabilities: []Capability{CapabilityOCR}, Engine: "onnxruntime", Format: "onnx",
			Languages: []string{"model dependent"}, EstimatedMemoryMB: 160, License: "PaddlePaddle model license",
		},
		"pp-ocrv6-small": {
			ID: "pp-ocrv6-small", Name: "PP-OCRv6 Small", Version: "det:28fe5895c24fd108c19eb3e8479f4ab385fbfc62 rec:b8f84f0b80c529de40b4fbb3544b84fa7233a513",
			Capabilities: []Capability{CapabilityOCR}, Engine: "onnxruntime", Format: "onnx",
			Languages: []string{"model dependent"}, EstimatedMemoryMB: 300, License: "PaddlePaddle model license",
		},
		"opus-mt-en-romance": {
			ID: "opus-mt-en-romance", Name: "OPUS-MT English to Romance", Version: "9d2ba69ac80c8e8453c3d9a1e2323a0e7b8ca3cd",
			Capabilities: []Capability{CapabilityTranslation}, Engine: "onnxruntime", Format: "onnx",
			Languages: []string{"en", "pt", "es", "fr", "it"}, EstimatedMemoryMB: 600, License: "Apache-2.0",
		},
		"opus-mt-romance-en": {
			ID: "opus-mt-romance-en", Name: "OPUS-MT Romance to English", Version: "f60feaabe8bb435a5871e7702b25f53a01e770f8",
			Capabilities: []Capability{CapabilityTranslation}, Engine: "onnxruntime", Format: "onnx",
			Languages: []string{"en", "pt", "es", "fr", "it"}, EstimatedMemoryMB: 600, License: "Apache-2.0",
		},
		"m2m100-418m": {
			ID: "m2m100-418m", Name: "M2M100 418M", Version: "9c374f0b7aca709787cea97b047bfbbd1559d177",
			Capabilities: []Capability{CapabilityTranslation}, Engine: "onnxruntime", Format: "onnx",
			Languages: []string{"multilingual"}, EstimatedMemoryMB: 1800, License: "MIT",
		},
	}
}
