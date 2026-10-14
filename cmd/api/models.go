package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/modelstore"
	"github.com/spf13/cobra"
)

func newModelsCommand() *cobra.Command {
	command := &cobra.Command{Use: "models", Short: "Lists and verifies local AI model artifacts"}
	command.AddCommand(
		&cobra.Command{Use: "list", Short: "Lists the model catalog and local state", RunE: func(command *cobra.Command, _ []string) error {
			registry, err := configuredModelRegistry()
			if err != nil {
				return err
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(registry.List())
		}},
		&cobra.Command{Use: "info MODEL", Short: "Shows one model descriptor and local state", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
			registry, err := configuredModelRegistry()
			if err != nil {
				return err
			}
			model, found := registry.Get(arguments[0])
			if !found {
				return fmt.Errorf("model %q is not registered", arguments[0])
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(model)
		}},
		&cobra.Command{Use: "verify MODEL", Short: "Verifies that every required local artifact is present", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if artifacts, manifestErr := installArtifacts(arguments[0], cfg); manifestErr == nil {
				if err := modelstore.Verify(artifacts); err != nil {
					return fmt.Errorf("verify %s: %w", arguments[0], err)
				}
			}
			registry, err := configuredModelRegistry()
			if err != nil {
				return err
			}
			model, found := registry.Get(arguments[0])
			if !found {
				return fmt.Errorf("model %q is not registered", arguments[0])
			}
			if model.State != aimodel.StateAvailable {
				return fmt.Errorf("model %q is not fully installed", arguments[0])
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(model)
		}},
		&cobra.Command{Use: "install MODEL", Short: "Downloads a pinned model release with SHA-256 verification", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			artifacts, err := installArtifacts(arguments[0], cfg)
			if err != nil {
				return err
			}
			if err := modelstore.Install(command.Context(), artifacts); err != nil {
				return fmt.Errorf("install %s: %w", arguments[0], err)
			}
			_, err = fmt.Fprintln(command.OutOrStdout(), arguments[0]+" installed")
			return err
		}},
		&cobra.Command{Use: "remove MODEL", Short: "Removes the exact local artifacts of a managed model", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, arguments []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			artifacts, err := installArtifacts(arguments[0], cfg)
			if err != nil {
				return err
			}
			if err := modelstore.Remove(artifacts); err != nil {
				return fmt.Errorf("remove %s: %w", arguments[0], err)
			}
			_, err = fmt.Fprintln(command.OutOrStdout(), arguments[0]+" removed")
			return err
		}},
	)
	return command
}

func installArtifacts(modelID string, cfg *config.Config) ([]modelstore.Artifact, error) {
	const maxArtifactBytes = 200 * 1024 * 1024
	var revision, repository, destination string
	var files [][2]string
	switch modelID {
	case "whisper-tiny":
		repository, revision, destination = "csukuangfj/sherpa-onnx-whisper-tiny", "65176e2deb88badc814a94058666cadccc29b61c", filepath.Join(cfg.ASRModelsDir, modelID)
		files = [][2]string{{"tiny-encoder.int8.onnx", "d24fb083ae3b1041fc24e97971d60e280c9342201fbb67b0ab428a8b4a51a434"}, {"tiny-decoder.int8.onnx", "d2fece8dd42771f1df975c6c0445770d0c292bf7547c2cae04a6c0cc57540925"}, {"tiny-tokens.txt", "b34b360dbb493e781e479794586d661700670d65564001f23024971d1f2fa126"}}
	case "pp-ocrv6-tiny":
		return ppOCRTinyArtifacts(cfg)
	case "opus-mt-en-romance":
		repository, revision, destination = "Xenova/opus-mt-en-ROMANCE", "9d2ba69ac80c8e8453c3d9a1e2323a0e7b8ca3cd", filepath.Join(cfg.TranslationModelsDir, modelID)
		files = [][2]string{{"onnx/encoder_model_int8.onnx", "f26bc5cd040987159e82a7e65d047de4a302e44cc9676ed240a44d5866b8cbe5"}, {"onnx/decoder_model_merged_int8.onnx", "dc25e58bd664fb9a23c6e962cbb5fc078f68de59654d9dff41573c7dcd1453e9"}, {"tokenizer.json", "31cabbd7e3857f21bcde266b0e84585ef3282133acce42d3765e8f31cb5f4040"}, {"vocab.json", "d9c3b6b01d3a9c9403befa60fd17e4a50dc4929698e3d6b9c2e7f066a9d1f041"}}
	case "opus-mt-romance-en":
		repository, revision, destination = "Xenova/opus-mt-ROMANCE-en", "f60feaabe8bb435a5871e7702b25f53a01e770f8", filepath.Join(cfg.TranslationModelsDir, modelID)
		files = [][2]string{{"onnx/encoder_model_int8.onnx", "df97d2f70156a0e3b993c75877e7710831e3f9fc30e6f26f7cad25e0466806c6"}, {"onnx/decoder_model_merged_int8.onnx", "61729b29859d821786fa85ddbb44c7d04bcf7deb6d14e9b614bd6c5f6aedb31f"}, {"tokenizer.json", "34979cc78f4dfa92f7b39ad8f0d4068dc7d1be61940f09db5ac77818e013bd99"}, {"vocab.json", "32c8eeec099b448425a7da4e09ec10acbe424a733780c9bcec2062846f03e706"}}
	default:
		return nil, fmt.Errorf("model %q does not yet have an install manifest", modelID)
	}
	artifacts := make([]modelstore.Artifact, 0, len(files))
	for _, file := range files {
		target := file[0]
		if target == "onnx/encoder_model_int8.onnx" {
			target = "encoder.onnx"
		} else if target == "onnx/decoder_model_merged_int8.onnx" {
			target = "decoder.onnx"
		}
		artifacts = append(artifacts, modelstore.Artifact{Path: filepath.Join(destination, target), URL: "https://huggingface.co/" + repository + "/resolve/" + revision + "/" + file[0], SHA256: file[1], MaxBytes: maxArtifactBytes})
	}
	return artifacts, nil
}

func ppOCRTinyArtifacts(cfg *config.Config) ([]modelstore.Artifact, error) {
	const maxArtifactBytes = 200 * 1024 * 1024
	dir := filepath.Join(cfg.OCRModelsDir, "pp-ocrv6-tiny")
	return []modelstore.Artifact{
		{Path: filepath.Join(dir, "det.onnx"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_tiny_det_onnx/resolve/2ba1506c0380b8f0b03dd142459aac66d4421f6c/inference.onnx", SHA256: "193bab7a04fca699a6c82e6abb5b81bdb28177f0abd4062552b04908dafb19f8", MaxBytes: maxArtifactBytes},
		{Path: filepath.Join(dir, "rec.onnx"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_tiny_rec_onnx/resolve/2612ab37152ae0a677521bae4e1e3d4fb4cf7c30/inference.onnx", SHA256: "9ef676d6ed3c88256a2d92c640c44f25b0c40947e111b14b8be8f594091563e6", MaxBytes: maxArtifactBytes},
		{Path: filepath.Join(dir, "rec.yml"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_tiny_rec_onnx/resolve/2612ab37152ae0a677521bae4e1e3d4fb4cf7c30/inference.yml", SHA256: "66170210bad538e83fff3c4a3867e547d6bf20b50d64b20347c4b913f3034ea1", MaxBytes: maxArtifactBytes},
	}, nil
}

func configuredModelRegistry() (*aimodel.Registry, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	registry, err := aimodel.New(cfg.ClassifierModelPath, cfg.DetectorModelPath, cfg.EnabledModels)
	if err != nil {
		return nil, err
	}
	if err := configureModelPaths(registry, cfg); err != nil {
		return nil, err
	}
	return registry, nil
}
