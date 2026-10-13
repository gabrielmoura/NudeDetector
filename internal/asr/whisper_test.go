package asr

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"pt-BR": "pt",
		" EN ":  "en",
		"":      "",
	} {
		if got := normalizeLanguage(input); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}
