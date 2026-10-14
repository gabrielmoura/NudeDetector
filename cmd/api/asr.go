package main

import (
	"fmt"
	"io"
	"os"

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
			runtime, err := newRuntime(runtimeOptions, false, false)
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
	return asr.WriteResult(writer, result, format)
}
