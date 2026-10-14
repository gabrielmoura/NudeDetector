package asr

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteResult serializes a transcription in one of the supported public formats.
func WriteResult(writer io.Writer, result Result, format string) error {
	switch format {
	case "json":
		return json.NewEncoder(writer).Encode(result)
	case "text":
		_, err := fmt.Fprintln(writer, result.Text)
		return err
	case "srt", "vtt":
		if len(result.Segments) == 0 {
			return fmt.Errorf("%s requires model timestamps", format)
		}
		if format == "vtt" {
			if _, err := fmt.Fprintln(writer, "WEBVTT"); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		for index, segment := range result.Segments {
			if format == "srt" {
				if _, err := fmt.Fprintln(writer, index+1); err != nil {
					return err
				}
			}
			separator := ","
			if format == "vtt" {
				separator = "."
			}
			if _, err := fmt.Fprintf(writer, "%s --> %s\n%s\n\n", subtitleTime(segment.Start, separator), subtitleTime(segment.End, separator), strings.TrimSpace(segment.Text)); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("format must be text, json, srt, or vtt")
	}
}

func subtitleTime(seconds float64, separator string) string {
	if seconds < 0 {
		seconds = 0
	}
	milliseconds := int64(seconds*1000 + .5)
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", milliseconds/3_600_000, (milliseconds/60_000)%60, (milliseconds/1000)%60, separator, milliseconds%1000)
}
