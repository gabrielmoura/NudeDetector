package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gabrielmoura/NudeDetector/internal/asr"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/spf13/cobra"
)

func newASRCommand(options *runtimeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "asr",
		Short: "Transcribes local audio with an installed Whisper model",
	}
	command.AddCommand(newTranscribeCommand(options))
	return command
}

func newTranscribeCommand(options *runtimeOptions) *cobra.Command {
	var modelID, language, format, outputPath string
	var timestamps bool
	command := &cobra.Command{
		Use:   "transcribe AUDIO",
		Short: "Transcribes WAV, MP3, OGG, M4A/AAC, or FLAC audio",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			data, err := os.ReadFile(arguments[0])
			if err != nil {
				return fmt.Errorf("read audio: %w", err)
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			audio, err := asr.DecodeAudio(command.Context(), data, cfg.ASRMaxDurationSeconds)
			if err != nil {
				return fmt.Errorf("decode audio: %w", err)
			}
			if modelID == "" {
				return fmt.Errorf("--model is required")
			}
			runtimeOptions := *options
			runtimeOptions.Models = modelID
			runtime, err := newRuntime(runtimeOptions, false)
			if err != nil {
				return err
			}
			defer runtime.Close()
			if runtime.transcriber == nil {
				return fmt.Errorf("selected model %q is not a transcriber", modelID)
			}
			result, err := runtime.transcriber.Transcribe(command.Context(), audio, asr.Request{
				Language:   language,
				Timestamps: timestamps,
			})
			if err != nil {
				return err
			}
			writer, closeOutput, err := outputWriter(outputPath, command.OutOrStdout())
			if err != nil {
				return fmt.Errorf("open output: %w", err)
			}
			defer closeOutput()
			return writeTranscription(writer, result, format)
		},
	}
	command.Flags().StringVar(&modelID, "model", "whisper-base", "installed Whisper model ID")
	command.Flags().StringVar(&language, "language", "", "optional language hint, for example pt or en")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().StringVarP(&outputPath, "output", "o", "", "write output to a file instead of stdout")
	command.Flags().BoolVar(&timestamps, "timestamps", false, "include token timestamps when returned by the model")
	return command
}

func writeTranscription(writer io.Writer, result asr.Result, format string) error {
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
		return fmt.Errorf("--format must be text, json, srt, or vtt")
	}
}

func subtitleTime(seconds float64, separator string) string {
	if seconds < 0 {
		seconds = 0
	}
	milliseconds := int64(seconds*1000 + .5)
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", milliseconds/3_600_000, (milliseconds/60_000)%60, (milliseconds/1000)%60, separator, milliseconds%1000)
}
