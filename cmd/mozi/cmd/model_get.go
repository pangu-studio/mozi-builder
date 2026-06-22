package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pangu-sutido/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	modelGetName string
	modelGetJSON bool
)

var modelGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Read a model's full ModelIR from the design database",
	Long: `Loads a complete ModelIR (including semantics, ui_intent, api_intent) from the design
database and outputs it as YAML (default) or JSON.

Examples:
  mozi model get --model content/Card
  mozi model get --model content/Card --json`,
	RunE: runModelGet,
}

func init() {
	modelGetCmd.Flags().StringVarP(&modelGetName, "model", "m", "", "Model reference: module/ModelName")
	modelGetCmd.Flags().BoolVar(&modelGetJSON, "json", false, "Output as JSON instead of YAML")
	modelCmd.AddCommand(modelGetCmd)
}

func runModelGet(cmd *cobra.Command, args []string) error {
	if modelGetName == "" {
		return fmt.Errorf("specify a model with --model (e.g., content/Card)")
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

	_, modelName := parseModelRef(modelGetName)

	model, err := store.LoadModel(modelName)
	if err != nil {
		return fmt.Errorf("load model %s: %w", modelName, err)
	}

	var output []byte
	if modelGetJSON {
		output, err = json.MarshalIndent(model, "", "  ")
	} else {
		output, err = yaml.Marshal(model)
	}
	if err != nil {
		return fmt.Errorf("marshal model: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
