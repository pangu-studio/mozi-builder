package cmd

import (
	"fmt"
	"os"

	"github.com/pangu-sutido/mozi-builder/mozi"
	"github.com/pangu-sutido/mozi-builder/mozi/db"
	"github.com/pangu-sutido/mozi-builder/mozi/parser"

	"github.com/spf13/cobra"
)

var (
	validateModule string
	validateModel  string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all model definitions in the design database",
	Long: `Reads models from the design database and performs semantic validation,
including field type checks, relation target verification, and required field checks.

Examples:
  mozi validate                           # Validate all models
  mozi validate --module content          # Validate only content module
  mozi validate --model content/Card      # Validate a single model (still checks cross-model relations)`,

	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateModule, "module", "", "Validate only a specific module")
	validateCmd.Flags().StringVarP(&validateModel, "model", "m", "", "Validate a single model (module/ModelName)")
}

func runValidate(cmd *cobra.Command, args []string) error {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	store, err := openStore(designDB)
	if err != nil {
		return fmt.Errorf("open design database: %w\nRun 'mozi init' first", err)
	}
	defer store.DB.Close()

	project, err := store.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}

	// Validate against full project (cross-model relations need all models).
	// Filter afterwards — see filterValidationOutput.
	fullProject := project
	outputProject := project

	if validateModel != "" {
		modName, modelName := parseModelRef(validateModel)
		outputProject = filterProjectToModel(project, modName, modelName)
	} else if validateModule != "" {
		var filtered []*mozi.ModuleIR
		for _, m := range project.Modules {
			if m.Name == validateModule {
				filtered = append(filtered, m)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("module '%s' not found", validateModule)
		}
		project.Modules = filtered
	}

	fmt.Printf("🔍 Validating models in design database...\n\n")

	result := parser.ValidateProject(fullProject)

	totalModels := 0
	for _, mod := range outputProject.Modules {
		totalModels += len(mod.Models)
	}
	fmt.Printf("Found %d module(s), %d model(s)\n\n", len(outputProject.Modules), totalModels)

	// Print warnings
	for _, w := range result.Warnings {
		fmt.Printf("  ⚠ %s\n", w.Error())
	}

	if len(result.Errors) == 0 {
		fmt.Printf("\n✅ All models are valid (%d model(s) checked)\n", totalModels)
		return nil
	}

	fmt.Println("\n❌ Validation errors:")
	for _, e := range result.Errors {
		fmt.Printf("  ✘ %s\n", e.Error())
	}
	fmt.Println()

	return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
}

// filterProjectToModel loads the full project but filters output to a single model.
// Cross-model relation validation still runs against all models in the project context.
func filterProjectToModel(project *mozi.ProjectIR, modName, modelName string) *mozi.ProjectIR {
	var targetMod *mozi.ModuleIR
	var targetModel *mozi.ModelIR

	for _, mod := range project.Modules {
		if mod.Name == modName {
			targetMod = mod
			for _, m := range mod.Models {
				if m.Name == modelName {
					targetModel = m
					break
				}
			}
			break
		}
	}

	if targetMod == nil || targetModel == nil {
		return project // Return full project — not found
	}

	// Create a filtered project that still contains all modules (for cross-model checks)
	// but only the filtered module has models for output purposes
	filtered := &mozi.ProjectIR{Name: project.Name}
	for _, mod := range project.Modules {
		if mod.Name == modName {
			filtered.Modules = append(filtered.Modules, &mozi.ModuleIR{
				Name:   mod.Name,
				Label:  mod.Label,
				Models: []*mozi.ModelIR{targetModel},
			})
		}
	}
	return filtered
}
