package cmd

import (
	"fmt"
	"os"

	"github.com/pangu-sutido/mozi-builder/mozi/db"
	"github.com/pangu-sutido/mozi-builder/mozi/differ"
	"github.com/pangu-sutido/mozi-builder/mozi/manifest"

	"github.com/spf13/cobra"
)

var (
	diffModel       string
	diffFromVersion string
	diffToVersion   string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show structured differences between model versions",
	Long: `Compares two versions of a model in the design database and shows a structured change report.

Examples:
  mozi diff --model content/Deck                    # Current vs last generated version
  mozi diff --model content/Deck --from 3 --to 5   # Compare specific versions`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVarP(&diffModel, "model", "m", "", "Model reference: module/ModelName")
	diffCmd.Flags().StringVar(&diffFromVersion, "from", "", "Source version (default: last generated)")
	diffCmd.Flags().StringVar(&diffToVersion, "to", "", "Target version (default: latest)")
}

func runDiff(cmd *cobra.Command, args []string) error {
	if diffModel == "" {
		return fmt.Errorf("specify a model with --model (e.g., content/Deck)")
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

	// Parse model reference
	modName, modelName := parseModelRef(diffModel)
	_ = modName

	// Load current model
	currentModel, err := store.LoadModel(modelName)
	if err != nil {
		return fmt.Errorf("load model %s: %w", modelName, err)
	}

	// Determine versions to compare
	_, _, _, _, _, currentVersion, err := store.GetModel(modelName)
	if err != nil {
		return fmt.Errorf("get model version: %w", err)
	}

	fromVersion := diffFromVersion
	toVersion := diffToVersion
	if toVersion == "" {
		toVersion = currentVersion
	}
	if fromVersion == "" {
		// Use last generated version from manifest
		projectRoot, _ := findProjectRoot()
		m, err := manifest.Load(projectRoot)
		if err == nil {
			info := m.GetGenInfo(diffModel)
			if info.LastGenVersion != "" {
				fromVersion = info.LastGenVersion
			} else {
				fromVersion = "1" // compare with initial version
			}
		} else {
			fromVersion = "1"
		}
	}
	if fromVersion >= toVersion {
		return fmt.Errorf("no changes: from version %s >= to version %s", fromVersion, toVersion)
	}

	// Load historical version via the shared Store method.
	// Prefers YAML snapshot (includes semantics/ui_intent/api_intent),
	// falls back to structured tables.
	fromModel, err := store.LoadModelVersion(modelName, fromVersion, currentModel.Module)
	if err != nil {
		return fmt.Errorf("load model at version %s: %w", fromVersion, err)
	}
	// Ensure model identity metadata matches current (may be absent in old snapshots)
	if fromModel.Label == "" {
		fromModel.Label = currentModel.Label
	}
	fromModel.Name = currentModel.Name
	fromModel.Module = currentModel.Module

	// Run diff
	result := differ.Compare(fromModel, currentModel, fromVersion, toVersion)

	// Display results
	fmt.Printf("🔍 Diff: %s (%s → %s)\n\n", diffModel, fromVersion, toVersion)

	if !result.HasChanges {
		fmt.Println("  ✅ No changes detected")
		return nil
	}

	fmt.Printf("  Changes: +%d added, ~%d modified, -%d removed\n\n",
		countChanges(result.Changes, differ.ChangeAdded),
		countChanges(result.Changes, differ.ChangeModified),
		countChanges(result.Changes, differ.ChangeRemoved))

	for _, c := range result.Changes {
		icon := "  "
		switch c.Type {
		case differ.ChangeAdded:
			icon = "  +"
		case differ.ChangeRemoved:
			icon = "  -"
		case differ.ChangeModified:
			icon = "  ~"
		}
		fmt.Printf("%s %s\n", icon, c.Detail)
	}

	// Show affected files
	files := result.AffectedFiles()
	if len(files) > 0 {
		fmt.Printf("\n  📁 Affected files (%d):\n", len(files))
		for _, f := range files {
			fmt.Printf("     %s — %s\n", f.Path, f.Description)
		}
	}

	fmt.Println()
	return nil
}

func countChanges(changes []differ.FieldChange, typ differ.ChangeType) int {
	n := 0
	for _, c := range changes {
		if c.Type == typ {
			n++
		}
	}
	return n
}
