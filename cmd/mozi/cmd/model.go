package cmd

import (
	"github.com/spf13/cobra"
)

// modelCmd is the parent command for model CRUD operations.
// These commands operate directly on the design database (no HTTP server needed).
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage models in the design database (no server required)",
	Long: `Direct CRUD operations on models in the design database.
These commands connect to the design DB directly — no HTTP server or admin UI needed.

Subcommands:
  get     Read a model's full ModelIR (YAML or JSON)
  create  Create a new model from a JSON payload
  update  Replace an existing model with a full JSON payload

Examples:
  mozi model get --model content/Card
  mozi model get --model content/Card --json
  mozi model create --json '{"module":"content","model":"Note",...}'
  mozi model update --model content/Card --json '{"module":"content","model":"Card",...}'

The update command requires a COMPLETE ModelIR payload — it does not do partial (JSON Merge Patch) updates.
This prevents accidentally clearing semantics, ui_intent, api_intent, or relations that are not passed.`,
}

func init() {
	rootCmd.AddCommand(modelCmd)
}
