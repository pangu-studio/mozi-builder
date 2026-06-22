package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Deprecated: template code generation has been retired",
	Long: `The old template overwrite workflow has been retired.

Use the model diff and dev-platform AI change plan instead:
  mozi diff --model content/Deck
  mozi change-plan --model content/Deck`,
	RunE: runGen,
}

func runGen(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("mozi gen has been retired; use model diff plus mozi change-plan to create an AI Coding patch")
}
