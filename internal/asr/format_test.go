package asr

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteResultFormatsSubtitles(t *testing.T) {
	result := Result{Segments: []Segment{{Start: 1.25, End: 2.5, Text: "Olá"}}}
	var srt bytes.Buffer
	if err := WriteResult(&srt, result, "srt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(srt.String(), "00:00:01,250 --> 00:00:02,500") {
		t.Fatalf("SRT = %q", srt.String())
	}
	var vtt bytes.Buffer
	if err := WriteResult(&vtt, result, "vtt"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(vtt.String(), "WEBVTT\n\n") || !strings.Contains(vtt.String(), "00:00:01.250") {
		t.Fatalf("VTT = %q", vtt.String())
	}
}

func TestWriteResultRejectsSubtitlesWithoutSegments(t *testing.T) {
	if err := WriteResult(&bytes.Buffer{}, Result{}, "srt"); err == nil {
		t.Fatal("WriteResult() error = nil")
	}
}
