package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/config"
	"github.com/gabrielmoura/NudeDetector/internal/modelstore"
	"github.com/spf13/cobra"
)

var startupInstallableModels = map[string]struct{}{
	"whisper-tiny":       {},
	"whisper-base":       {},
	"whisper-small":      {},
	"pp-ocrv6-tiny":      {},
	"pp-ocrv6-small":     {},
	"opus-mt-en-romance": {},
	"opus-mt-romance-en": {},
	"m2m100-418m":        {},
}

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
	const (
		maxArtifactBytes     = 200 * 1024 * 1024
		maxWhisperSmallBytes = 512 * 1024 * 1024
		maxM2MArtifactBytes  = 1024 * 1024 * 1024
	)
	var revision, repository, destination string
	var files [][2]string
	switch modelID {
	case "whisper-tiny":
		repository, revision, destination = "csukuangfj/sherpa-onnx-whisper-tiny", "65176e2deb88badc814a94058666cadccc29b61c", filepath.Join(cfg.ASRModelsDir, modelID)
		files = [][2]string{{"tiny-encoder.int8.onnx", "d24fb083ae3b1041fc24e97971d60e280c9342201fbb67b0ab428a8b4a51a434"}, {"tiny-decoder.int8.onnx", "d2fece8dd42771f1df975c6c0445770d0c292bf7547c2cae04a6c0cc57540925"}, {"tiny-tokens.txt", "b34b360dbb493e781e479794586d661700670d65564001f23024971d1f2fa126"}}
	case "whisper-base":
		repository, revision, destination = "csukuangfj/sherpa-onnx-whisper-base", "31dc0cc3b59ba4a0b52941f1d6dce9c0e30311b4", filepath.Join(cfg.ASRModelsDir, modelID)
		files = [][2]string{{"base-encoder.int8.onnx", "38d196d447cf451f98193eb013ef1e0d23d83e6967031607497eb9eecbb0a06c"}, {"base-decoder.int8.onnx", "f0b4d0314d591215f96fbfd78636fa524f51c5949688535b0ebd7f2b5b1f19a3"}, {"base-tokens.txt", "b34b360dbb493e781e479794586d661700670d65564001f23024971d1f2fa126"}}
	case "whisper-small":
		repository, revision, destination = "csukuangfj/sherpa-onnx-whisper-small", "8f3c18b358db4d1f2fc1eae49d75cd20989e4309", filepath.Join(cfg.ASRModelsDir, modelID)
		files = [][2]string{{"small-encoder.int8.onnx", "4cbe7b22fa9026b843b60a68640c747de05bafb1a11b57edc0e66c232d9f33a9"}, {"small-decoder.int8.onnx", "acad50b5c782696e91b55914cc5ab4f756f1532f76e22aa6fc615f39fb69a8ee"}, {"small-tokens.txt", "b34b360dbb493e781e479794586d661700670d65564001f23024971d1f2fa126"}}
	case "pp-ocrv6-tiny":
		return ppOCRTinyArtifacts(cfg)
	case "pp-ocrv6-small":
		return ppOCRSmallArtifacts(cfg)
	case "opus-mt-en-romance":
		repository, revision, destination = "Xenova/opus-mt-en-ROMANCE", "9d2ba69ac80c8e8453c3d9a1e2323a0e7b8ca3cd", filepath.Join(cfg.TranslationModelsDir, modelID)
		files = [][2]string{{"onnx/encoder_model_int8.onnx", "f26bc5cd040987159e82a7e65d047de4a302e44cc9676ed240a44d5866b8cbe5"}, {"onnx/decoder_model_merged_int8.onnx", "dc25e58bd664fb9a23c6e962cbb5fc078f68de59654d9dff41573c7dcd1453e9"}, {"tokenizer.json", "31cabbd7e3857f21bcde266b0e84585ef3282133acce42d3765e8f31cb5f4040"}, {"vocab.json", "d9c3b6b01d3a9c9403befa60fd17e4a50dc4929698e3d6b9c2e7f066a9d1f041"}}
	case "opus-mt-romance-en":
		repository, revision, destination = "Xenova/opus-mt-ROMANCE-en", "f60feaabe8bb435a5871e7702b25f53a01e770f8", filepath.Join(cfg.TranslationModelsDir, modelID)
		files = [][2]string{{"onnx/encoder_model_int8.onnx", "df97d2f70156a0e3b993c75877e7710831e3f9fc30e6f26f7cad25e0466806c6"}, {"onnx/decoder_model_merged_int8.onnx", "61729b29859d821786fa85ddbb44c7d04bcf7deb6d14e9b614bd6c5f6aedb31f"}, {"tokenizer.json", "34979cc78f4dfa92f7b39ad8f0d4068dc7d1be61940f09db5ac77818e013bd99"}, {"vocab.json", "32c8eeec099b448425a7da4e09ec10acbe424a733780c9bcec2062846f03e706"}}
	case "m2m100-418m":
		repository, revision, destination = "Xenova/m2m100_418M", "9c374f0b7aca709787cea97b047bfbbd1559d177", filepath.Join(cfg.TranslationModelsDir, modelID)
		files = [][2]string{{"onnx/encoder_model_int8.onnx", "b47e2d5f3fe1123a8916a30ac0cf648870faef00d33fbd06afa9a333d8a3c457"}, {"onnx/decoder_model_merged_int8.onnx", "2974783b0627ef59fff6ccc89eb538efe27cf82599c837546d2cd85c081ae31d"}, {"tokenizer.json", "03d9e111731c2d71f39a2c2a88499743e4c251385d07f0384b4349a23ba54363"}}
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
		maxBytes := int64(maxArtifactBytes)
		switch modelID {
		case "whisper-small":
			maxBytes = maxWhisperSmallBytes
		case "m2m100-418m":
			maxBytes = maxM2MArtifactBytes
		}
		artifacts = append(artifacts, modelstore.Artifact{Path: filepath.Join(destination, target), URL: "https://huggingface.co/" + repository + "/resolve/" + revision + "/" + file[0], SHA256: file[1], MaxBytes: maxBytes})
	}
	return artifacts, nil
}

// installSelectedModelArtifacts installs optional models selected for a server
// process before sessions are created. It is never called by request handlers.
func installSelectedModelArtifacts(
	ctx context.Context,
	cfg *config.Config,
	modelIDs []string,
	log *slog.Logger,
) error {
	for _, modelID := range modelIDs {
		if _, ok := startupInstallableModels[modelID]; !ok {
			continue
		}
		artifacts, err := installArtifacts(modelID, cfg)
		if err != nil {
			return fmt.Errorf("resolve install manifest for %s: %w", modelID, err)
		}
		log.Info("model_auto_installing", slog.String("model", modelID))
		if err := modelstore.Install(ctx, artifacts); err != nil {
			return fmt.Errorf("install %s at startup: %w", modelID, err)
		}
	}
	return nil
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

func ppOCRSmallArtifacts(cfg *config.Config) ([]modelstore.Artifact, error) {
	const maxArtifactBytes = 200 * 1024 * 1024
	dir := filepath.Join(cfg.OCRModelsDir, "pp-ocrv6-small")
	return []modelstore.Artifact{
		{Path: filepath.Join(dir, "det.onnx"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_det_onnx/resolve/28fe5895c24fd108c19eb3e8479f4ab385fbfc62/inference.onnx", SHA256: "d73e0058b7a8086bbd57f3d10b8bcd4ff95363f67e06e2762b5e814fe9c9410e", MaxBytes: maxArtifactBytes},
		{Path: filepath.Join(dir, "rec.onnx"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/b8f84f0b80c529de40b4fbb3544b84fa7233a513/inference.onnx", SHA256: "5435fd747c9e0efe15a96d0b378d5bd157e9492ed8fd80edf08f30d02fa24634", MaxBytes: maxArtifactBytes},
		{Path: filepath.Join(dir, "rec.yml"), URL: "https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/b8f84f0b80c529de40b4fbb3544b84fa7233a513/inference.yml", SHA256: "ab078671bb49f06228eadccd34f1bb501e157f7a047095ffb943ba81512c77d1", MaxBytes: maxArtifactBytes},
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
