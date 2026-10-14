package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gabrielmoura/NudeDetector/internal/ocr"
	"github.com/spf13/cobra"
)

func newOCRCommand(options *runtimeOptions) *cobra.Command {
	var modelID, language, format, outputPath string
	var boxes bool
	command := &cobra.Command{Use: "ocr", Short: "Recognizes text in an image with PP-OCRv6"}
	command.AddCommand(&cobra.Command{Use: "recognize IMAGE", Short: "Recognizes text in JPEG, PNG, or WebP", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
		data, err := os.ReadFile(arguments[0])
		if err != nil {
			return fmt.Errorf("read image: %w", err)
		}
		runtimeOptions := *options
		runtimeOptions.Models = modelID
		runtime, err := newRuntime(runtimeOptions, false, false)
		if err != nil {
			return err
		}
		defer runtime.Close()
		if runtime.ocrRecognizer == nil {
			return fmt.Errorf("selected model %q is not an OCR recognizer", modelID)
		}
		result, err := runtime.ocrRecognizer.Recognize(command.Context(), data, ocr.Request{Language: language, ReturnBoxes: boxes})
		if err != nil {
			return err
		}
		if !boxes {
			result.Blocks = []ocr.Block{}
		}
		writer, closeOutput, err := outputWriter(outputPath, command.OutOrStdout())
		if err != nil {
			return fmt.Errorf("open output: %w", err)
		}
		defer closeOutput()
		if format == "text" {
			_, err = fmt.Fprintln(writer, result.Text)
			return err
		}
		if format != "json" {
			return fmt.Errorf("--format must be text or json")
		}
		return json.NewEncoder(writer).Encode(result)
	}})
	recognize := command.Commands()[0]
	recognize.Flags().StringVar(&modelID, "model", "pp-ocrv6-tiny", "installed PP-OCRv6 model ID")
	recognize.Flags().StringVar(&language, "language", "", "language hint")
	recognize.Flags().StringVar(&format, "format", "text", "output format: text or json")
	recognize.Flags().StringVarP(&outputPath, "output", "o", "", "write output to a file instead of stdout")
	recognize.Flags().BoolVar(&boxes, "boxes", false, "include bounding boxes")
	return command
}
