package cmd

import (
	"fmt"
	"os"

	"github.com/pangu-sutido/mozi-builder/mozi"
	"github.com/pangu-sutido/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
)

var (
	rollbackModel   string
	rollbackVersion string
	rollbackForce   bool
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback a model to a previous version",
	Long: `Restores a model to a specified previous version by creating a new version
with the old model's fields, relations, and admin config.

The rollback creates a new version (not deleting intermediate versions),
so the full history is preserved.

Example:
  mozi rollback --model content/Deck --version 3
  mozi rollback --model content/Deck --version 3 --force  # Skip confirmation`,
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().StringVarP(&rollbackModel, "model", "m", "", "Model reference: module/ModelName")
	rollbackCmd.Flags().StringVarP(&rollbackVersion, "version", "v", "", "Version to rollback to")
	rollbackCmd.Flags().BoolVarP(&rollbackForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(rollbackCmd)
}

func runRollback(cmd *cobra.Command, args []string) error {
	if rollbackModel == "" {
		return fmt.Errorf("specify a model with --model (e.g., content/Deck)")
	}
	if rollbackVersion == "" {
		return fmt.Errorf("specify a target version with --version")
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

	_, modelName := parseModelRef(rollbackModel)

	// Load current model
	currentModel, err := store.LoadModel(modelName)
	if err != nil {
		return fmt.Errorf("load model %s: %w", modelName, err)
	}
	_, _, _, _, _, currentVersion, _ := store.GetModel(modelName)

	// Load target version data
	targetFields, err := store.GetFields(modelName, rollbackVersion)
	if err != nil {
		return fmt.Errorf("load fields at version %s: %w", rollbackVersion, err)
	}
	targetRelations, err := store.GetRelations(modelName, rollbackVersion)
	if err != nil {
		return fmt.Errorf("load relations at version %s: %w", rollbackVersion, err)
	}
	targetAdmin, err := store.GetAdmin(modelName, rollbackVersion)
	if err != nil {
		return fmt.Errorf("load admin at version %s: %w", rollbackVersion, err)
	}

	// Show what will change
	fmt.Printf("⏪ Rollback: %s\n", rollbackModel)
	fmt.Printf("   From version %s → version %s\n\n", currentVersion, rollbackVersion)
	fmt.Printf("   Fields:    %d (current) → %d (target)\n", len(currentModel.Fields), len(targetFields))
	fmt.Printf("   Relations: %d (current) → %d (target)\n", len(currentModel.Relations), len(targetRelations))
	fmt.Println()

	if !rollbackForce {
		fmt.Print("   Confirm rollback? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("   Rollback cancelled")
			return nil
		}
	}

	// Apply rollback by saving as a new version
	rollbackModelIR := &ModelIRFromDB{
		Module:    currentModel.Module,
		Name:      currentModel.Name,
		Label:     currentModel.Label,
		Desc:      currentModel.Description,
		Table:     currentModel.Table,
		Fields:    targetFields,
		Relations: targetRelations,
		Admin:     *targetAdmin,
	}

	summary := fmt.Sprintf("Rollback from %s to %s", currentVersion, rollbackVersion)

	if err := saveModelFromIR(store, rollbackModelIR, summary); err != nil {
		return fmt.Errorf("save rollback: %w", err)
	}

	_, _, _, _, _, newVersion, _ := store.GetModel(modelName)
	fmt.Printf("\n✅ Rolled back %s to version %s (saved as version %s)\n",
		rollbackModel, rollbackVersion, newVersion)
	return nil
}

// ModelIRFromDB holds the data to create a model version from DB-loaded data.
type ModelIRFromDB struct {
	Module    string
	Name      string
	Label     string
	Desc      string
	Table     string
	Fields    []mozi.FieldIR
	Relations []mozi.RelationIR
	Admin     mozi.AdminConfig
}

// saveModelFromIR persists model data as a new version.
func saveModelFromIR(store *db.Store, m *ModelIRFromDB, summary string) error {
	model := &mozi.ModelIR{
		Module:      m.Module,
		Name:        m.Name,
		Label:       m.Label,
		Description: m.Desc,
		Table:       m.Table,
		Fields:      m.Fields,
		Relations:   m.Relations,
		Admin:       m.Admin,
	}
	return store.SaveModel(model, summary, "")
}
