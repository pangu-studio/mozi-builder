// Package cmd provides the CLI command implementations for mozi.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const projectRootEnv = "MOZI_PROJECT_ROOT"

var projectRootFlag string

var rootCmd = &cobra.Command{
	Use:   "mozi",
	Short: "Mozi — Model-Driven Development Platform",
	Long: `Mozi is a model-driven development platform that stores business models,
tracks version diffs, and exposes AI Coding change plans for incremental patches.

Workflow:
  mozi init                  # Initialize models/ directory with example
  mozi validate              # Validate all model YAML files
  mozi diff --model User     # Inspect model version changes
  mozi change-plan -m User   # Get an AI Coding change plan
  mozi dictionary list api_consumers`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if projectRootFlag == "" {
			return nil
		}
		projectRoot, err := normalizeProjectRoot(projectRootFlag)
		if err != nil {
			return err
		}
		os.Setenv(projectRootEnv, projectRoot)
		_ = godotenv.Load(filepath.Join(projectRoot, ".env"))
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// 加载 .env 文件（不存在则忽略）
	_ = godotenv.Load()

	rootCmd.PersistentFlags().StringVar(&projectRootFlag, "project-root", "", "Business project root; defaults to searching upward from the current directory or MOZI_PROJECT_ROOT")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(diffCmd)
}

func configuredProjectRoot() (string, bool, error) {
	value := projectRootFlag
	if value == "" {
		value = os.Getenv(projectRootEnv)
	}
	if value == "" {
		return "", false, nil
	}
	root, err := normalizeProjectRoot(value)
	return root, true, err
}

func normalizeProjectRoot(value string) (string, error) {
	root, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("project root %s: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root %s is not a directory", root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("project root %s must contain go.mod: %w", root, err)
	}
	return root, nil
}
