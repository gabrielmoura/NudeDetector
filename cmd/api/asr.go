package main

import (
	"encoding/json"
	"fmt"
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
	var modelID, language, format string
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
				return fmt.Errorf("decode WAV: %w", err)
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
			switch format {
			case "json":
				return json.NewEncoder(command.OutOrStdout()).Encode(result)
			case "text":
				_, err := fmt.Fprintln(command.OutOrStdout(), result.Text)
				return err
			default:
				return fmt.Errorf("--format must be text or json")
			}
		},
	}
	command.Flags().StringVar(&modelID, "model", "whisper-base", "installed Whisper model ID")
	command.Flags().StringVar(&language, "language", "", "optional language hint, for example pt or en")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	command.Flags().BoolVar(&timestamps, "timestamps", false, "include token timestamps when returned by the model")
	return command
}
