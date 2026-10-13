package main

import (
	"encoding/json"
	"fmt"

	"github.com/gabrielmoura/NudeDetector/internal/aimodel"
	"github.com/gabrielmoura/NudeDetector/internal/config"
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
	)
	return command
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
