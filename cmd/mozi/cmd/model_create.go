package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"memflow/mozi-builder/mozi"
	"memflow/mozi-builder/mozi/db"
	"memflow/mozi-builder/mozi/parser"

	"github.com/spf13/cobra"
)

var modelCreateJSON string

var modelCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new model in the design database",
	Long: `Creates a new model from a complete ModelIR JSON payload and saves it to the design database.
A new version is created automatically.

The JSON payload must be a complete ModelIR object including module, model name, fields,
relations, admin config, and optionally semantics, ui_intent, and api_intent.

Example:
  mozi model create --json '{
    "module": "content",
    "model": "Note",
    "label": "笔记",
    "description": "用户的学习笔记",
    "table": "notes",
    "fields": [...],
    "relations": [...],
    "admin": {...}
  }'`,
	RunE: runModelCreate,
}

func init() {
	modelCreateCmd.Flags().StringVar(&modelCreateJSON, "json", "", "Complete ModelIR JSON payload")
	modelCmd.AddCommand(modelCreateCmd)
}

func runModelCreate(cmd *cobra.Command, args []string) error {
	if modelCreateJSON == "" {
		// Try reading from stdin if --json not provided
		return fmt.Errorf("specify a model with --json (complete ModelIR), or pipe JSON via stdin")
	}

	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	store, err := openStore(designDB)
	if err != nil {
		return fmt.Errorf("open design database: %w", err)
	}
	defer store.DB.Close()

	var model mozi.ModelIR
	if err := json.Unmarshal([]byte(modelCreateJSON), &model); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	// Support both "model" and "name" fields in JSON
	if model.Name == "" {
		var alias struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal([]byte(modelCreateJSON), &alias)
		model.Name = alias.Name
	}

	if model.Module == "" || model.Name == "" {
		return fmt.Errorf("model payload must include 'module' and 'model' (or 'name') fields")
	}

	// Normalize defaults and resolve relation targets
	parser.NormalizeModel(&model, model.Module)

	// Validate before saving
	result := parser.Validate(&model)
	if !result.Valid {
		var errs []string
		for _, e := range result.Errors {
			errs = append(errs, e.Error())
		}
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "⚠ %s\n", w.Error())
		}
	}

	if err := store.SaveModel(&model, "Created via CLI", ""); err != nil {
		return fmt.Errorf("save model: %w", err)
	}

	fmt.Printf("✅ Created %s/%s\n", model.Module, model.Name)
	return nil
}
