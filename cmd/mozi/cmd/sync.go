package cmd

import (
	"fmt"
	"os"

	moziapply "github.com/pangu-studio/mozi-builder/mozi/apply"
	"github.com/pangu-studio/mozi-builder/mozi/db"
	"github.com/pangu-studio/mozi-builder/mozi/manifest"

	"github.com/spf13/cobra"
)

var (
	syncModel string
	syncAll   bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Record the current model version as generated in the manifest",
	Long: `Records the current design DB version of a model in the .mozi/manifest.json file,
marking it as synced to code. After running this, the change-plan for the model
will show status "applied" instead of "pending".

Examples:
  mozi sync --model content/Deck
  mozi sync --all`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncModel, "model", "m", "", "Model reference: module/ModelName")
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Sync all models")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	projectRoot, err := moziapply.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	mf, err := manifest.Load(projectRoot)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if syncAll {
		return syncAllModels(designDB, mf)
	}

	if syncModel == "" {
		return fmt.Errorf("specify a model with --model (e.g., content/Deck) or --all")
	}

	return syncOneModel(designDB, mf, syncModel)
}

func syncOneModel(designDB string, mf *manifest.Manifest, modelRef string) error {
	store, err := openStore(designDB)
	if err != nil {
		return fmt.Errorf("open design database: %w", err)
	}
	defer store.DB.Close()

	modelName := modelRef
	if _, name, ok := splitModelRef(modelRef); ok {
		modelName = name
	}

	_, _, _, _, _, currentVersion, err := store.GetModel(modelName)
	if err != nil {
		return fmt.Errorf("get model %s: %w", modelRef, err)
	}

	mf.RecordGenWithMetadata(modelRef, currentVersion, getVersion(), "", nil, "advisory")
	if err := mf.Save(); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	fmt.Printf("✅ Synced %s at %s\n", modelRef, currentVersion)
	return nil
}

func syncAllModels(designDB string, mf *manifest.Manifest) error {
	store, err := openStore(designDB)
	if err != nil {
		return fmt.Errorf("open design database: %w", err)
	}
	defer store.DB.Close()

	project, err := store.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}

	count := 0
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			_, _, _, _, _, version, err := store.GetModel(m.Name)
			if err != nil {
				fmt.Printf("  ⚠️  %s/%s: %v\n", mod.Name, m.Name, err)
				continue
			}
			ref := mod.Name + "/" + m.Name
			mf.RecordGenWithMetadata(ref, version, getVersion(), "", nil, "advisory")
			fmt.Printf("  ✅ %s %s\n", ref, version)
			count++
		}
	}

	if err := mf.Save(); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	fmt.Printf("\n✅ Synced %d model(s)\n", count)
	return nil
}

func splitModelRef(ref string) (module, model string, ok bool) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '/' {
			return ref[:i], ref[i+1:], true
		}
	}
	return "", ref, false
}
