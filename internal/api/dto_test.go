package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/moderation"
)

func TestParseDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
		err   bool
	}{
		{name: "omitted", want: false},
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "one", value: "1", want: true},
		{name: "invalid", value: "yes", err: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDetails(test.value)
			if (err != nil) != test.err {
				t.Fatalf("parseDetails(%q) error = %v, want error=%t", test.value, err, test.err)
			}
			if got != test.want {
				t.Fatalf("parseDetails(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestModerationResponseDetailsAreOptional(t *testing.T) {
	result := &moderation.ModerationResult{
		PornographyScore:   0.8,
		PornographyPercent: 80,
		Classification:     "pornographic",
		Scores: moderation.ClassificationScores{
			Porn: 0.8,
			Sexy: 0.1,
		},
	}

	compact, err := json.Marshal(newModerationResponse(result, false, nil))
	if err != nil {
		t.Fatalf("marshal compact response: %v", err)
	}
	if strings.Contains(string(compact), "scores") {
		t.Fatalf("compact response contains scores: %s", compact)
	}

	detailed, err := json.Marshal(newModerationResponse(result, true, nil))
	if err != nil {
		t.Fatalf("marshal detailed response: %v", err)
	}
	if !strings.Contains(string(detailed), `"scores"`) || !strings.Contains(string(detailed), `"porn":0.8`) {
		t.Fatalf("detailed response misses scores: %s", detailed)
	}
}
