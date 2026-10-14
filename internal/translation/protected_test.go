package translation

import (
	"context"
	"strings"
	"testing"
)

type protectedStub struct{}

func (protectedStub) Translate(_ context.Context, request Request) (Result, error) {
	return Result{SourceText: request.Text, TranslatedText: strings.ToUpper(request.Text), Model: "stub", Metadata: map[string]any{}}, nil
}
func (protectedStub) Close() error { return nil }

func TestProtectedTranslatorPreservesTechnicalSpans(t *testing.T) {
	translator := NewProtectedTranslator(protectedStub{}, 64)
	result, err := translator.Translate(t.Context(), Request{Text: "read `go test` at https://example.test/a <b>now</b>", SourceLanguage: "en", TargetLanguage: "pt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"`go test`", "https://example.test/a", "<b>", "</b>"} {
		if !strings.Contains(result.TranslatedText, value) {
			t.Fatalf("translation %q misses %q", result.TranslatedText, value)
		}
	}
}

func TestSplitTranslationTextBreaksAtWhitespace(t *testing.T) {
	parts := splitTranslationText("one two three four", 8)
	if len(parts) < 2 || strings.Join(parts, "") != "one two three four" {
		t.Fatalf("parts = %#v", parts)
	}
}

func TestProtectedTranslatorPreservesWhitespaceAdjacentToURL(t *testing.T) {
	translator := NewProtectedTranslator(protectedStub{}, 64)
	result, err := translator.Translate(t.Context(), Request{Text: "hello https://example.test/a", SourceLanguage: "en", TargetLanguage: "pt"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TranslatedText != "HELLO https://example.test/a" {
		t.Fatalf("translation = %q", result.TranslatedText)
	}
}
