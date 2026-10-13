package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gabrielmoura/NudeDetector/internal/asr"
)

func TestWriteTranscriptionFormatsSubtitles(t *testing.T) {
	result := asr.Result{Segments: []asr.Segment{{Start: 1.25, End: 2.5, Text: "Olá"}}}
	var srt bytes.Buffer
	if err := writeTranscription(&srt, result, "srt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(srt.String(), "00:00:01,250 --> 00:00:02,500") {
		t.Fatalf("SRT = %q", srt.String())
	}
	var vtt bytes.Buffer
	if err := writeTranscription(&vtt, result, "vtt"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(vtt.String(), "WEBVTT\n\n") || !strings.Contains(vtt.String(), "00:00:01.250") {
		t.Fatalf("VTT = %q", vtt.String())
	}
}

func TestWriteTranscriptionRejectsSubtitlesWithoutSegments(t *testing.T) {
	if err := writeTranscription(&bytes.Buffer{}, asr.Result{}, "srt"); err == nil {
		t.Fatal("writeTranscription() error = nil")
	}
}
