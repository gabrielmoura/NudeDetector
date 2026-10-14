package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabrielmoura/NudeDetector/internal/api"
	"github.com/gabrielmoura/NudeDetector/internal/detector"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
	"github.com/spf13/cobra"
)

type scanOptions struct {
	InputPath   string
	OutputDir   string
	JSONOutput  string
	Recursive   bool
	DrawBoxes   bool
	Blur        bool
	BlurRadius  float64
	FrameTimeMS int64
}

type scanRecord struct {
	Path       string                       `json:"path"`
	Kind       string                       `json:"kind"`
	Result     *moderation.ModerationResult `json:"result,omitempty"`
	Output     string                       `json:"output,omitempty"`
	Detections int                          `json:"detections,omitempty"`
	Details    []detector.Detection         `json:"details,omitempty"`
	Error      string                       `json:"error,omitempty"`
}

type scanSummary struct {
	Type      string `json:"type"`
	Files     int    `json:"files"`
	Processed int    `json:"processed"`
	Skipped   int    `json:"skipped"`
	Failed    int    `json:"failed"`
}

func newScanCommand(options *runtimeOptions) *cobra.Command {
	var scan scanOptions
	command := &cobra.Command{
		Use:   "scan",
		Short: "Reads all supported media files from a directory",
		RunE: func(command *cobra.Command, arguments []string) error {
			if scan.InputPath == "" {
				return fmt.Errorf("--input is required")
			}
			info, err := os.Stat(scan.InputPath)
			if err != nil {
				return fmt.Errorf("inspect input directory: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("--input must be a directory")
			}
			inputAbs, err := filepath.Abs(scan.InputPath)
			if err != nil {
				return fmt.Errorf("resolve input directory: %w", err)
			}
			if scan.OutputDir != "" {
				outputAbs, err := filepath.Abs(scan.OutputDir)
				if err != nil {
					return fmt.Errorf("resolve output directory: %w", err)
				}
				if pathWithin(inputAbs, outputAbs) {
					return fmt.Errorf("--output-dir must not be inside --input")
				}
			}
			if scan.JSONOutput == "" {
				scan.JSONOutput = "-"
			}
			if scan.BlurRadius < 1 || scan.BlurRadius > 100 {
				return fmt.Errorf("--blur-radius must be between 1 and 100")
			}
			if scan.FrameTimeMS < 0 {
				return fmt.Errorf("--frame-time-ms must not be negative")
			}
			jsonWriter, closeJSON, err := openJSONOutput(command, scan.JSONOutput)
			if err != nil {
				return err
			}
			defer closeJSON()

			runtime, err := newRuntime(*options, true, false)
			if err != nil {
				return err
			}
			defer runtime.Close()
			return scanDirectory(command, runtime, scan, jsonWriter)
		},
	}
	command.Flags().StringVarP(&scan.InputPath, "input", "i", "", "directory containing images, GIFs, or videos")
	command.Flags().StringVarP(&scan.OutputDir, "output-dir", "o", "", "directory for exported JPEGs; empty disables image export")
	command.Flags().StringVar(&scan.JSONOutput, "json-output", "-", "JSONL destination; '-' writes to stdout")
	command.Flags().BoolVar(&scan.Recursive, "recursive", true, "scan nested directories")
	command.Flags().BoolVar(&scan.DrawBoxes, "draw-boxes", true, "draw labels and detection boxes on exported media")
	command.Flags().BoolVar(&scan.Blur, "blur", false, "blur detected regions on exported media")
	command.Flags().Float64Var(&scan.BlurRadius, "blur-radius", 25, "blur radius between 1 and 100")
	command.Flags().Int64Var(&scan.FrameTimeMS, "frame-time-ms", 0, "GIF or video frame time in milliseconds")
	return command
}

func scanDirectory(command *cobra.Command, runtime *runtime, options scanOptions, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	summary := scanSummary{Type: "summary"}

	if options.OutputDir != "" {
		if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	err := filepath.WalkDir(options.InputPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := command.Context().Err(); err != nil {
			return err
		}
		if path != options.InputPath && entry.IsDir() && !options.Recursive {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		summary.Files++

		kind := scanMediaKind(path)
		if kind == "" {
			summary.Skipped++
			return nil
		}
		summary.Processed++
		record := scanRecord{Path: path, Kind: kind}
		data, err := os.ReadFile(path)
		if err == nil && kind == "image" {
			record.Result, err = runtime.handler.Classify(command.Context(), data)
		}
		if err == nil && kind == "media" {
			var detections []detector.Detection
			if options.OutputDir == "" {
				detections, err = runtime.handler.DetectDetails(
					command.Context(), data, detectContentType(path, data), options.FrameTimeMS,
				)
			} else {
				var result []byte
				result, detections, err = runtime.handler.ExportDetailed(
					command.Context(),
					data,
					detectContentType(path, data),
					api.ExportOptions{
						DrawBoxes:   options.DrawBoxes,
						Blur:        options.Blur,
						BlurRadius:  options.BlurRadius,
						FrameTimeMS: options.FrameTimeMS,
					},
				)
				if err == nil {
					record.Output = scanOutputPath(options.InputPath, options.OutputDir, path)
					err = os.MkdirAll(filepath.Dir(record.Output), 0o755)
				}
				if err == nil {
					err = os.WriteFile(record.Output, result, 0o644)
				}
			}
			record.Detections = len(detections)
			record.Details = detections
		}
		if err != nil {
			record.Error = err.Error()
			summary.Failed++
		}
		return encoder.Encode(record)
	})
	if err != nil {
		return err
	}
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("write scan summary: %w", err)
	}
	if summary.Failed > 0 {
		return fmt.Errorf("scan completed with %d failed file(s)", summary.Failed)
	}
	return nil
}

func openJSONOutput(command *cobra.Command, path string) (io.Writer, func(), error) {
	if path == "-" {
		return command.OutOrStdout(), func() {}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create JSON output directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("create JSON output: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}

func scanMediaKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return "image"
	case ".gif", ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return "media"
	default:
		return ""
	}
}

func scanOutputPath(inputDir, outputDir, path string) string {
	relative, err := filepath.Rel(inputDir, path)
	if err != nil {
		relative = filepath.Base(path)
	}
	extension := filepath.Ext(relative)
	return filepath.Join(outputDir, strings.TrimSuffix(relative, extension)+"-moderated.jpg")
}

func pathWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	isSamePath := relative == "."
	isParentPath := relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	return isSamePath || !isParentPath
}
