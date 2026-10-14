package translation

import (
	"context"
	"strings"
	"time"
)

// ProtectedTranslator preserves technical spans while translating adjacent prose.
// It scans code spans, HTML tags, and URLs instead of rewriting them with regexes.
type ProtectedTranslator struct {
	inner              Translator
	maxChunkCharacters int
}

func NewProtectedTranslator(inner Translator, maxChunkCharacters int) *ProtectedTranslator {
	return &ProtectedTranslator{inner: inner, maxChunkCharacters: maxChunkCharacters}
}

func (p *ProtectedTranslator) Translate(ctx context.Context, request Request) (Result, error) {
	started := time.Now()
	parts := protectedParts(request.Text)
	var translated strings.Builder
	var first Result
	chunks := 0
	for _, part := range parts {
		if part.protected {
			translated.WriteString(part.text)
			continue
		}
		leading, prose, trailing := splitOuterWhitespace(part.text)
		translated.WriteString(leading)
		for _, chunk := range splitTranslationText(prose, p.maxChunkCharacters) {
			if chunk == "" {
				continue
			}
			result, err := p.inner.Translate(ctx, Request{Text: chunk, SourceLanguage: request.SourceLanguage, TargetLanguage: request.TargetLanguage})
			if err != nil {
				return Result{}, err
			}
			if chunks == 0 {
				first = result
			}
			chunks++
			translated.WriteString(result.TranslatedText)
		}
		translated.WriteString(trailing)
	}
	first.SourceText = request.Text
	first.TranslatedText = translated.String()
	first.ProcessingTimeMS = time.Since(started).Milliseconds()
	if first.Metadata == nil {
		first.Metadata = map[string]any{}
	}
	first.Metadata["protected_translation_chunks"] = chunks
	return first, nil
}

func splitOuterWhitespace(text string) (leading, prose, trailing string) {
	start := 0
	for start < len(text) && isSpace(text[start]) {
		start++
	}
	end := len(text)
	for end > start && isSpace(text[end-1]) {
		end--
	}
	return text[:start], text[start:end], text[end:]
}

func (p *ProtectedTranslator) Close() error { return p.inner.Close() }

type protectedPart struct {
	text      string
	protected bool
}

func protectedParts(text string) []protectedPart {
	parts := make([]protectedPart, 0, 4)
	start := 0
	for i := 0; i < len(text); {
		end := protectedEnd(text, i)
		if end == i {
			i++
			continue
		}
		if start < i {
			parts = append(parts, protectedPart{text: text[start:i]})
		}
		parts = append(parts, protectedPart{text: text[i:end], protected: true})
		start, i = end, end
	}
	if start < len(text) {
		parts = append(parts, protectedPart{text: text[start:]})
	}
	return parts
}

func protectedEnd(text string, start int) int {
	if strings.HasPrefix(text[start:], "http://") || strings.HasPrefix(text[start:], "https://") {
		end := start
		for end < len(text) && !isSpace(text[end]) {
			end++
		}
		return end
	}
	if text[start] == '<' {
		if end := strings.IndexByte(text[start:], '>'); end >= 0 {
			return start + end + 1
		}
	}
	if text[start] == '`' {
		marker := "`"
		if strings.HasPrefix(text[start:], "```") {
			marker = "```"
		}
		if end := strings.Index(text[start+len(marker):], marker); end >= 0 {
			return start + len(marker) + end + len(marker)
		}
	}
	return start
}

func isSpace(value byte) bool { return value == ' ' || value == '\n' || value == '\r' || value == '\t' }

func splitTranslationText(text string, maximum int) []string {
	if maximum < 1 || len([]rune(text)) <= maximum {
		return []string{text}
	}
	parts := make([]string, 0, len(text)/maximum+1)
	for len(text) > 0 {
		end := maximum
		if end > len(text) {
			end = len(text)
		}
		if end < len(text) {
			for end > 0 && !isSpace(text[end-1]) {
				end--
			}
			if end == 0 {
				end = maximum
			}
		}
		parts, text = append(parts, text[:end]), text[end:]
	}
	return parts
}
