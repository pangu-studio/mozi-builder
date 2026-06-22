package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pangu-sutido/mozi-builder/devplatform"
	"github.com/pangu-sutido/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
)

var (
	changePlanModel string
	changePlanJSON  bool
)

var changePlanCmd = &cobra.Command{
	Use:   "change-plan",
	Short: "Get AI Coding change plan for a model",
	Long: `Fetches the AI Coding task contract for a model change, including diff,
tasks, contracts, and verification checks. Use this instead of curl + serve.

Examples:
  mozi change-plan --model content/Deck
  mozi change-plan --model content/Deck --json`,
	RunE: runChangePlan,
}

func init() {
	changePlanCmd.Flags().StringVarP(&changePlanModel, "model", "m", "", "Model reference: module/ModelName")
	changePlanCmd.Flags().BoolVar(&changePlanJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(changePlanCmd)
}

func runChangePlan(cmd *cobra.Command, args []string) error {
	if changePlanModel == "" {
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

	engine := devplatform.NewDevPlatformEngine()
	svc := devplatform.NewService(store, engine)

	plan, err := svc.ChangePlan(cmd.Context(), changePlanModel)
	if err != nil {
		return fmt.Errorf("get change plan: %w", err)
	}

	if changePlanJSON {
		out, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	// Pretty-print the change plan
	statusIcon := "🔵"
	switch plan.Status {
	case devplatform.ChangePlanApplied:
		statusIcon = "✅"
	case devplatform.ChangePlanNoDiff:
		statusIcon = "⚪"
	}
	fmt.Printf("📋 Change Plan: %s  %s %s\n\n", plan.ModelRef, statusIcon, plan.Status)

	// Intent
	fmt.Printf("🎯 Intent: %s\n\n", plan.Intent)

	// Diff summary
	if plan.Diff != nil && plan.Diff.HasChanges {
		fmt.Println("📊 Changes:")
		for _, c := range plan.Diff.Changes {
			icon := "  +"
			switch c.Type {
			case "removed":
				icon = "  -"
			case "modified":
				icon = "  ~"
			}
			fmt.Printf("%s %s\n", icon, c.Detail)
		}
		fmt.Println()
	} else {
		fmt.Println("  ✅ No changes detected")
	}

	// Affected files
	if len(plan.AffectedFiles) > 0 {
		fmt.Printf("📁 Affected files (%d):\n", len(plan.AffectedFiles))
		for _, f := range plan.AffectedFiles {
			fmt.Printf("   %s — %s\n", f.Path, f.Description)
		}
		fmt.Println()
	}

	// Tasks
	fmt.Println("📝 Tasks:")
	for _, t := range plan.Tasks {
		fmt.Printf("   [%s] %s\n", t.Area, t.Description)
		if len(t.Files) > 0 {
			for _, f := range t.Files {
				fmt.Printf("      → %s\n", f)
			}
		}
	}
	fmt.Println()

	// Contracts
	fmt.Println("📜 Contracts:")
	for _, c := range plan.Contracts {
		fmt.Printf("   • %s\n", c)
	}
	fmt.Println()

	// Verification checks
	fmt.Println("✅ Verification:")
	for _, c := range plan.Checks {
		fmt.Printf("   $ %s\n", c)
	}
	return nil
}
