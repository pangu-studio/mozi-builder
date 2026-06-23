package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pangu-sutido/mozi-builder/mozi"
	"github.com/pangu-sutido/mozi-builder/mozi/db"
	"github.com/pangu-sutido/mozi-builder/mozi/parser"

	"github.com/spf13/cobra"
)

var (
	modelUpdateName string
	modelUpdateJSON string
)

var modelUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Replace an existing model in the design database",
	Long: `Replaces an existing model with a complete ModelIR JSON payload and saves it as a new version.

⚠️  This command requires a COMPLETE ModelIR payload — it does NOT do partial (JSON Merge Patch) updates.
Passing a partial payload will lose any fields, relations, semantics, ui_intent, or api_intent
that are not explicitly included. Use 'mozi model get' first to inspect the current model,
modify the output, then pass the full result to this command.

Every relation must include "label": the business predicate shown in ER diagrams, such as
"包含", "归属于", "创建", "产生", "记录", or "表示".

Example:
  mozi model get --model content/Card --json > card.json
  # edit card.json
  mozi model update --model content/Card --json "$(cat card.json)"`,
	RunE: runModelUpdate,
}

func init() {
	modelUpdateCmd.Flags().StringVarP(&modelUpdateName, "model", "m", "", "Model reference: module/ModelName")
	modelUpdateCmd.Flags().StringVar(&modelUpdateJSON, "json", "", "Complete ModelIR JSON payload (required)")
	modelCmd.AddCommand(modelUpdateCmd)
}

func runModelUpdate(cmd *cobra.Command, args []string) error {
	if modelUpdateName == "" {
		return fmt.Errorf("specify a model with --model (e.g., content/Card)")
	}
	if modelUpdateJSON == "" {
		return fmt.Errorf("specify the updated model with --json (complete ModelIR payload)")
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

	_, modelName := parseModelRef(modelUpdateName)

	// Load existing to ensure it exists and to inherit identity fields
	existing, err := store.LoadModel(modelName)
	if err != nil {
		return fmt.Errorf("load existing model %s: %w", modelName, err)
	}

	var model mozi.ModelIR
	if err := json.Unmarshal([]byte(modelUpdateJSON), &model); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	// Inherit identity from existing when not provided
	if model.Module == "" {
		model.Module = existing.Module
	}
	if model.Name == "" {
		model.Name = existing.Name
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

	if err := store.SaveModel(&model, "Updated via CLI", ""); err != nil {
		return fmt.Errorf("save model: %w", err)
	}

	fmt.Printf("✅ Updated %s/%s (new version created)\n", model.Module, model.Name)
	return nil
}
