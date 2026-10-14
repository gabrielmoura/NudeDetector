package translation

import "testing"

func TestArgmaxM2MLastToken(t *testing.T) {
	logits := make([]float32, m2mVocabulary*2)
	logits[7] = 100
	logits[m2mVocabulary+42] = 3
	logits[m2mVocabulary+9] = 1

	got, err := argmaxM2M(logits)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("argmaxM2M() = %d, want 42", got)
	}
}

func TestArgmaxM2MRejectsUnexpectedLogits(t *testing.T) {
	if _, err := argmaxM2M(make([]float32, m2mVocabulary-1)); err == nil {
		t.Fatal("argmaxM2M() error = nil, want unexpected logits error")
	}
}
