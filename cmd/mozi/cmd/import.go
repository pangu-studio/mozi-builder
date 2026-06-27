package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/db"
	"github.com/pangu-studio/mozi-builder/mozi/parser"

	"github.com/spf13/cobra"
)

var (
	importDir  string
	importFile string
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import model YAML files into the design database",
	Long: `Reads model YAML files from models/ directory and imports them into the design database.

Examples:
  mozi import --dir models/              # Import entire models/ directory
  mozi import --file models/content/deck.yaml  # Import a single model`,
	RunE: runImport,
}

func init() {
	importCmd.Flags().StringVar(&importDir, "dir", "", "Directory of YAML models to import")
	importCmd.Flags().StringVar(&importFile, "file", "", "Single YAML file to import")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	store, err := openStore(designDB)
	if err != nil {
		return err
	}
	defer store.DB.Close()

	if importFile != "" {
		return importSingleFile(store, importFile)
	}
	if importDir != "" {
		return importDirModels(store, importDir)
	}

	// Default: import from models/ directory
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("could not find project root: %w", err)
	}
	return importDirModels(store, resolveModelsDir(projectRoot))
}

func importSingleFile(store *db.Store, path string) error {
	// Determine module name from parent directory
	moduleName := filepath.Base(filepath.Dir(path))
	model, err := parser.ParseFile(path, moduleName)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateImportModel(model); err != nil {
		return err
	}

	if err := store.SaveModel(model, "Imported from "+path, ""); err != nil {
		return fmt.Errorf("save model %s: %w", model.Name, err)
	}

	fmt.Printf("✓ Imported [%s] %s (%s)\n", model.Module, model.Name, model.Label)
	return nil
}

func importDirModels(store *db.Store, dir string) error {
	project, err := parser.ParseProject(dir)
	if err != nil {
		return fmt.Errorf("parse project: %w", err)
	}

	total := 0
	for _, mod := range project.Modules {
		// Upsert module first
		if err := store.UpsertModule(mod); err != nil {
			return fmt.Errorf("upsert module %s: %w", mod.Name, err)
		}
		fmt.Printf("  Module: %s (%s)\n", mod.Name, mod.Label)

		for _, model := range mod.Models {
			if err := validateImportModel(model); err != nil {
				return err
			}
			if err := store.SaveModel(model, "Imported from YAML", ""); err != nil {
				return fmt.Errorf("save model %s: %w", model.Name, err)
			}
			fmt.Printf("    ✓ %s (%s) — %d fields, %d relations\n",
				model.Name, model.Label, len(model.Fields), len(model.Relations))
			total++
		}
	}

	fmt.Printf("\n✅ Imported %d model(s) into design database\n", total)
	return nil
}

func validateImportModel(model *mozi.ModelIR) error {
	result := parser.Validate(model)
	if result.Valid {
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "⚠ %s\n", w.Error())
		}
		return nil
	}

	var errs []string
	for _, e := range result.Errors {
		errs = append(errs, e.Error())
	}
	return fmt.Errorf("validation failed for %s/%s:\n  %s", model.Module, model.Name, strings.Join(errs, "\n  "))
}

func openStore(connStr string) (*db.Store, error) {
	database, err := db.InitDB(connStr)
	if err != nil {
		return nil, fmt.Errorf("open design database: %w", err)
	}
	return db.NewStore(database), nil
}
