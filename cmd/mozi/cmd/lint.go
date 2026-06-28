package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pangu-studio/mozi-builder/mozi/db"
	"github.com/pangu-studio/mozi-builder/mozi/parser"
	"github.com/spf13/cobra"
)

var lintStrict, lintJSON bool

var lintCmd = &cobra.Command{
	Use: "lint", Short: "Run project-wide design lint rules",
	RunE: runLint,
}

func init() {
	lintCmd.Flags().BoolVar(&lintStrict, "strict", false, "Promote warnings to errors")
	lintCmd.Flags().BoolVar(&lintJSON, "json", false, "Output machine-readable JSON")
	rootCmd.AddCommand(lintCmd)
}

func runLint(cmd *cobra.Command, args []string) error {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}
	store, err := openStore(designDB)
	if err != nil {
		return fmt.Errorf("open design database: %w", err)
	}
	defer store.DB.Close()
	project, err := store.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	result := parser.LintProject(project, parser.LintOptions{Strict: lintStrict})
	if lintJSON {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		for _, issue := range result.Issues {
			fmt.Printf("%s\t%s\t%s\t%s\n", issue.Severity, issue.Code, issue.Model, issue.Message)
		}
		fmt.Printf("\n%d issue(s)\n", len(result.Issues))
	}
	if !result.Valid {
		return fmt.Errorf("lint failed")
	}
	return nil
}
