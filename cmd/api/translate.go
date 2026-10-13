package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gabrielmoura/NudeDetector/internal/translation"
	"github.com/spf13/cobra"
)

func newTranslateCommand(options *runtimeOptions) *cobra.Command {
	var modelID, source, target, filePath, format string
	command := &cobra.Command{Use: "translate [TEXT]", Short: "Translates text with an installed local model", Args: cobra.MaximumNArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
		var text string
		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read translation input: %w", err)
			}
			text = string(data)
		} else if len(arguments) == 1 {
			text = arguments[0]
		} else {
			return fmt.Errorf("TEXT or --file is required")
		}
		runtimeOptions := *options
		runtimeOptions.Models = modelID
		runtime, err := newRuntime(runtimeOptions, false)
		if err != nil {
			return err
		}
		defer runtime.Close()
		if runtime.translator == nil {
			return fmt.Errorf("selected model %q is not a translator", modelID)
		}
		result, err := runtime.translator.Translate(command.Context(), translation.Request{Text: text, SourceLanguage: source, TargetLanguage: target})
		if err != nil {
			return err
		}
		if format == "text" {
			_, err = fmt.Fprintln(command.OutOrStdout(), result.TranslatedText)
			return err
		}
		if format != "json" {
			return fmt.Errorf("--format must be text or json")
		}
		return json.NewEncoder(command.OutOrStdout()).Encode(result)
	}}
	command.Flags().StringVar(&modelID, "model", "opus-mt-en-romance", "installed translation model ID")
	command.Flags().StringVar(&source, "from", "", "source language, for example en")
	command.Flags().StringVar(&target, "to", "", "target language, for example pt")
	command.Flags().StringVar(&filePath, "file", "", "read source text from a file")
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}
