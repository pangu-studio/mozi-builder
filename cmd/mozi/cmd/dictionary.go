package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pangu-studio/mozi-builder/devplatform"
	"github.com/pangu-studio/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
)

var dictionaryCmd = &cobra.Command{
	Use:   "dictionary",
	Short: "Manage design-database dictionaries",
	Long: `Manage business-specific design dictionaries such as api_consumers.

These commands operate directly on the current design database and are the
stable CLI surface used by mozi skills.`,
}

var (
	dictionaryListIncludeDisabled bool
	dictionaryListJSON            bool

	dictionaryUpsertLabel       string
	dictionaryUpsertDescription string
	dictionaryUpsertAliases     []string
	dictionaryUpsertSortOrder   int
	dictionaryUpsertDisabled    bool
	dictionaryUpsertJSON        bool

	dictionaryDeleteJSON bool
)

var dictionaryListCmd = &cobra.Command{
	Use:   "list <dictionary>",
	Short: "List items in a design dictionary",
	Args:  cobra.ExactArgs(1),
	Example: `  mozi dictionary list api_consumers
  mozi dictionary list api_consumers --include-disabled --json`,
	RunE: runDictionaryList,
}

var dictionaryUpsertCmd = &cobra.Command{
	Use:   "upsert <dictionary> <value>",
	Short: "Create or update a design dictionary item",
	Args:  cobra.ExactArgs(2),
	Example: `  mozi dictionary upsert api_consumers desktop --label "桌面端（Tauri）" --alias "桌面端" --alias tauri
  mozi dictionary upsert api_consumers mobile_app --label "移动 App" --sort 60 --json`,
	RunE: runDictionaryUpsert,
}

var dictionaryDeleteCmd = &cobra.Command{
	Use:   "delete <dictionary> <value>",
	Short: "Delete a design dictionary item",
	Args:  cobra.ExactArgs(2),
	Example: `  mozi dictionary delete api_consumers mobile_app
  mozi dictionary delete api_consumers mobile_app --json`,
	RunE: runDictionaryDelete,
}

func init() {
	dictionaryListCmd.Flags().BoolVar(&dictionaryListIncludeDisabled, "include-disabled", false, "Include disabled items")
	dictionaryListCmd.Flags().BoolVar(&dictionaryListJSON, "json", false, "Output as JSON")

	dictionaryUpsertCmd.Flags().StringVar(&dictionaryUpsertLabel, "label", "", "Display label")
	dictionaryUpsertCmd.Flags().StringVar(&dictionaryUpsertDescription, "description", "", "Description")
	dictionaryUpsertCmd.Flags().StringArrayVar(&dictionaryUpsertAliases, "alias", nil, "Alias value; can be repeated")
	dictionaryUpsertCmd.Flags().IntVar(&dictionaryUpsertSortOrder, "sort", 0, "Sort order")
	dictionaryUpsertCmd.Flags().BoolVar(&dictionaryUpsertDisabled, "disabled", false, "Save item as disabled")
	dictionaryUpsertCmd.Flags().BoolVar(&dictionaryUpsertJSON, "json", false, "Output as JSON")

	dictionaryDeleteCmd.Flags().BoolVar(&dictionaryDeleteJSON, "json", false, "Output as JSON")

	dictionaryCmd.AddCommand(dictionaryListCmd)
	dictionaryCmd.AddCommand(dictionaryUpsertCmd)
	dictionaryCmd.AddCommand(dictionaryDeleteCmd)
	rootCmd.AddCommand(dictionaryCmd)
}

func runDictionaryList(cmd *cobra.Command, args []string) error {
	svc, cleanup, err := openDevPlatformService()
	if err != nil {
		return err
	}
	defer cleanup()

	items, err := svc.ListDesignDictionaryItems(cmd.Context(), args[0], dictionaryListIncludeDisabled)
	if err != nil {
		return fmt.Errorf("list dictionary items: %w", err)
	}
	if dictionaryListJSON {
		return printJSON(items)
	}
	for _, item := range items {
		status := "enabled"
		if !item.Enabled {
			status = "disabled"
		}
		aliases := strings.Join(item.Aliases, ", ")
		if aliases != "" {
			aliases = " aliases=[" + aliases + "]"
		}
		fmt.Printf("%s\t%s\t%s%s\n", item.Value, item.Label, status, aliases)
	}
	return nil
}

func runDictionaryUpsert(cmd *cobra.Command, args []string) error {
	svc, cleanup, err := openDevPlatformService()
	if err != nil {
		return err
	}
	defer cleanup()

	enabled := !dictionaryUpsertDisabled
	input := devplatform.DesignDictionaryItemInput{
		Value:       args[1],
		Label:       dictionaryUpsertLabel,
		Description: dictionaryUpsertDescription,
		Aliases:     dictionaryUpsertAliases,
		SortOrder:   dictionaryUpsertSortOrder,
		Enabled:     &enabled,
	}
	if err := svc.SaveDesignDictionaryItem(cmd.Context(), args[0], input); err != nil {
		return fmt.Errorf("save dictionary item: %w", err)
	}
	if dictionaryUpsertJSON {
		return printJSON(map[string]string{
			"status":        "saved",
			"dictionary_id": args[0],
			"value":         args[1],
		})
	}
	fmt.Printf("saved %s/%s\n", args[0], args[1])
	return nil
}

func runDictionaryDelete(cmd *cobra.Command, args []string) error {
	svc, cleanup, err := openDevPlatformService()
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.DeleteDesignDictionaryItem(cmd.Context(), args[0], args[1]); err != nil {
		return fmt.Errorf("delete dictionary item: %w", err)
	}
	if dictionaryDeleteJSON {
		return printJSON(map[string]string{
			"status":        "deleted",
			"dictionary_id": args[0],
			"value":         args[1],
		})
	}
	fmt.Printf("deleted %s/%s\n", args[0], args[1])
	return nil
}

func openDevPlatformService() (*devplatform.Service, func(), error) {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}
	store, err := openStore(designDB)
	if err != nil {
		return nil, nil, fmt.Errorf("open design database: %w", err)
	}
	engine := devplatform.NewDevPlatformEngine()
	return devplatform.NewService(store, engine), func() { _ = store.DB.Close() }, nil
}

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
