package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"memflow/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
)

var historyModel string

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show version history for a model",
	Long: `Displays the version history of a model stored in the design database.
Each version shows the version number, change summary, author, and timestamp.

Example:
  mozi history --model content/Deck`,
	RunE: runHistory,
}

func init() {
	historyCmd.Flags().StringVarP(&historyModel, "model", "m", "", "Model reference: module/ModelName")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	if historyModel == "" {
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
	_, modelName := parseModelRef(historyModel)

	// Load model info
	_, _, label, _, _, currentVersion, err := store.GetModel(modelName)
	if err != nil {
		return fmt.Errorf("model '%s' not found: %w", modelName, err)
	}

	// Get version history
	versions, err := store.ListVersions(modelName)
	if err != nil {
		return fmt.Errorf("list versions: %w", err)
	}

	fmt.Printf("📋 Version history: %s (%s)\n", modelName, label)
	fmt.Printf("   Current version: %s\n\n", formatVersionDisplay(currentVersion))

	if len(versions) == 0 {
		fmt.Println("   No version history found")
		return nil
	}

	fmt.Println("   Version              │ Summary                   │ Author     │ Created")
	fmt.Println("   ─────────────────────┼───────────────────────────┼────────────┼──────────────────")
	for _, v := range versions {
		marker := " "
		if v.Version == currentVersion {
			marker = "▶"
		}
		summary := v.ChangeSummary
		if len(summary) > 25 {
			summary = summary[:25] + "..."
		}
		if summary == "" {
			summary = "(no summary)"
		}
		author := v.CreatedBy
		if author == "" {
			author = "system"
		}
		versionDisplay := formatVersionDisplay(v.Version)
		fmt.Printf("  %s %-19s │ %-25s │ %-10s │ %s\n",
			marker, versionDisplay, summary, author, v.CreatedAt)
	}

	return nil
}

// formatVersionDisplay parses a version string (YYYYMMDDHHmmss) and formats it as human-readable local time.
// If parsing fails, returns the original string unchanged.
func formatVersionDisplay(version string) string {
	// Handle collision suffixes like "20260621203015_1"
	base := version
	suffix := ""
	if idx := strings.LastIndex(version, "_"); idx > 0 && len(base) >= 14 {
		base = version[:idx]
		suffix = version[idx:]
	}

	t, err := time.Parse("20060102150405", base)
	if err != nil {
		return version
	}
	return t.Format("2006-01-02 15:04:05") + suffix
}
