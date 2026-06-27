package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pangu-sutido/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the design database and models directory",
	Long: `Creates the design database (PostgreSQL) and initializes the models/ directory.

Environment variable MOZI_DB sets the PostgreSQL connection string.
Default: postgres://localhost:5432/memflow_design?sslmode=disable

The init command:
1. Creates the design database schema (modules, models, versions, fields, relations, admin)
2. Creates the models/ directory structure
3. Optionally imports existing YAML files into the database`,
	RunE: runInit,
}

// DefaultModelsDir is the default directory for model YAML files.
const DefaultModelsDir = "models"

// modelsDirEnv is the environment variable that overrides the models directory path.
const modelsDirEnv = "MOZI_MODELS_DIR"

// resolveModelsDir returns the models directory path.
// Checks MOZI_MODELS_DIR env var first; if set, joins it with projectRoot
// (unless it's already an absolute path). Falls back to DefaultModelsDir.
func resolveModelsDir(projectRoot string) string {
	if dir := os.Getenv(modelsDirEnv); dir != "" {
		if filepath.IsAbs(dir) {
			return dir
		}
		return filepath.Join(projectRoot, dir)
	}
	return filepath.Join(projectRoot, DefaultModelsDir)
}

func runInit(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("could not find project root (looking for go.mod): %w", err)
	}

	// 1. Initialize design database
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	fmt.Printf("→ Connecting to design database: %s\n", designDB)
	storeDB, err := db.InitDB(designDB)
	if err != nil {
		return fmt.Errorf("initialize design database: %w\n\nMake sure PostgreSQL is running and the database exists.\nConnection: %s\nSet MOZI_DB to override.", err, designDB)
	}
	storeDB.Close()
	fmt.Printf("✓ Design database initialized: %s\n", designDB)

	// 2. Create models directory if it doesn't exist
	modelsDir := resolveModelsDir(projectRoot)
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return fmt.Errorf("create models directory: %w", err)
	}

	// Create .mozi directory for manifest and snapshots
	moziDir := filepath.Join(modelsDir, ".mozi")
	if err := os.MkdirAll(moziDir, 0755); err != nil {
		return fmt.Errorf("create .mozi directory: %w", err)
	}
	snapshotsDir := filepath.Join(moziDir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		return fmt.Errorf("create snapshots directory: %w", err)
	}

	// Create _project.yaml if it doesn't exist
	projectYAML := filepath.Join(modelsDir, "_project.yaml")
	if _, err := os.Stat(projectYAML); os.IsNotExist(err) {
		content := `# MemFlow Cloud 项目配置
name: memflow-cloud
module: memflow/cloud
backend:
  package: memflow/cloud
  framework: gin
  orm: ent
frontend:
  framework: react-antd
  package_manager: npm
`
		if err := os.WriteFile(projectYAML, []byte(content), 0644); err != nil {
			return fmt.Errorf("write _project.yaml: %w", err)
		}
		fmt.Printf("✓ Created _project.yaml\n")
	}

	fmt.Printf("\n✅ Initialization complete\n")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Import existing models: mozi import --dir models/")
	fmt.Println("  2. Validate models:        mozi validate")
	fmt.Println("  3. Inspect diffs:          mozi diff --model content/Deck")
	fmt.Println("  4. Get change plan:        mozi change-plan --model content/Deck")
	return nil
}

// findProjectRoot searches upward from the current directory for a go.mod file.
func findProjectRoot() (string, error) {
	if root, ok, err := configuredProjectRoot(); ok || err != nil {
		return root, err
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gomod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(gomod); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}
