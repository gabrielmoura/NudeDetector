package translation

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	cases := map[string]string{"pt-BR": "pt", " EN ": "en", "": ""}
	for input, want := range cases {
		if got := normalizeLanguage(input); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestArgmaxLastToken(t *testing.T) {
	logits := make([]float32, decoderVocabulary*2)
	logits[7] = 100
	logits[decoderVocabulary+42] = 3
	logits[decoderVocabulary+9] = 1
	if got := argmaxLastToken(logits); got != 42 {
		t.Fatalf("argmaxLastToken() = %d, want 42", got)
	}
}
