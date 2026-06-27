// Package cmd provides the CLI command implementations for mozi.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const projectRootEnv = "MOZI_PROJECT_ROOT"

// version is set via ldflags at build time (e.g. -X 'main.Version=0.1.0').
// When empty, falls back to searching for a VERSION file relative to the
// executable or working directory.
var version string

var projectRootFlag string
var versionFlag bool

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
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Println("mozi version", getVersion())
			return
		}
		_ = cmd.Help()
	},
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
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the mozi version")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(diffCmd)
}

// getVersion returns the version string. If set via ldflags, it returns that
// value. Otherwise it searches for a VERSION file relative to the executable
// or working directory.
func getVersion() string {
	if version != "" {
		return version
	}
	if v := findVersionFile(); v != "" {
		return v
	}
	return "dev"
}

// findVersionFile searches for a VERSION file in likely locations and returns
// its trimmed contents, or "" if not found.
func findVersionFile() string {
	// Search paths relative to the executable.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, dir := range searchDirs(exeDir) {
			if v := readVersion(filepath.Join(dir, "VERSION")); v != "" {
				return v
			}
		}
	}
	// Search paths relative to the working directory.
	if wd, err := os.Getwd(); err == nil {
		for _, dir := range searchDirs(wd) {
			if v := readVersion(filepath.Join(dir, "VERSION")); v != "" {
				return v
			}
		}
	}
	// Search relative to the source file (for "go run" scenarios).
	if _, filename, _, ok := runtime.Caller(0); ok {
		srcDir := filepath.Dir(filename)
		for _, dir := range searchDirs(srcDir) {
			if v := readVersion(filepath.Join(dir, "VERSION")); v != "" {
				return v
			}
		}
	}
	return ""
}

// searchDirs returns a list of directories to search, starting from the given
// directory and walking up to its ancestors (up to 4 levels).
func searchDirs(start string) []string {
	var dirs []string
	dirs = append(dirs, start)
	current := start
	for range 4 {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		dirs = append(dirs, parent)
		current = parent
	}
	return dirs
}

// readVersion reads and trims the contents of the file at path, returning ""
// on any error.
func readVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
