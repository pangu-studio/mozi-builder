package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/db"
	"github.com/spf13/cobra"
)

var errorCodeCmd = &cobra.Command{Use: "error-code", Short: "Manage the project error-code registry"}
var errorCodeListJSON bool
var errorCodeDomain, errorCodeCategory, errorCodeMessage, errorCodeDetails, errorCodeI18n string
var errorCodeStatus int
var errorCodeConsumerFacing, errorCodeRetryable, errorCodeDeprecated bool

func init() {
	list := &cobra.Command{Use: "list", RunE: runErrorCodeList}
	list.Flags().BoolVar(&errorCodeListJSON, "json", false, "Output JSON")
	upsert := &cobra.Command{Use: "upsert <CODE>", Args: cobra.ExactArgs(1), RunE: runErrorCodeUpsert}
	upsert.Flags().StringVar(&errorCodeDomain, "domain", "", "Business domain")
	upsert.Flags().IntVar(&errorCodeStatus, "status", 500, "HTTP status")
	upsert.Flags().StringVar(&errorCodeCategory, "category", "system", "Error category")
	upsert.Flags().StringVar(&errorCodeMessage, "message", "", "Default message")
	upsert.Flags().BoolVar(&errorCodeConsumerFacing, "consumer-facing", false, "May be exposed to clients")
	upsert.Flags().BoolVar(&errorCodeRetryable, "retryable", false, "Client may retry")
	upsert.Flags().StringVar(&errorCodeDetails, "details-schema", "", "OpenAPI details schema")
	upsert.Flags().StringVar(&errorCodeI18n, "i18n-key", "", "Translation key")
	upsert.Flags().BoolVar(&errorCodeDeprecated, "deprecated", false, "Mark deprecated")
	remove := &cobra.Command{Use: "delete <CODE>", Args: cobra.ExactArgs(1), RunE: runErrorCodeDelete}
	errorCodeCmd.AddCommand(list, upsert, remove)
	rootCmd.AddCommand(errorCodeCmd)
}

func errorCodeStore() (*db.Store, error) {
	dsn := os.Getenv("MOZI_DB")
	if dsn == "" {
		dsn = db.DefaultDesignDB
	}
	return openStore(dsn)
}

func runErrorCodeList(cmd *cobra.Command, args []string) error {
	store, err := errorCodeStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	items, err := store.ListErrorCodes()
	if err != nil {
		return err
	}
	if errorCodeListJSON {
		return printJSON(items)
	}
	for _, item := range items {
		fmt.Printf("%s\t%d\t%s\t%s\n", item.Code, item.HTTPStatus, item.Category, item.Message)
	}
	return nil
}

func runErrorCodeUpsert(cmd *cobra.Command, args []string) error {
	store, err := errorCodeStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	item := mozi.ErrorCodeIR{Code: args[0], Domain: errorCodeDomain, HTTPStatus: errorCodeStatus, Category: errorCodeCategory, Message: errorCodeMessage, ConsumerFacing: errorCodeConsumerFacing, Retryable: errorCodeRetryable, DetailsSchema: errorCodeDetails, I18nKey: errorCodeI18n, Deprecated: errorCodeDeprecated}
	if !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(item.Code) {
		return fmt.Errorf("error code must use uppercase letters, numbers, and underscores")
	}
	if item.HTTPStatus < 400 || item.HTTPStatus > 599 {
		return fmt.Errorf("status must be between 400 and 599")
	}
	validCategories := map[string]bool{"resource": true, "validation": true, "permission": true, "business": true, "system": true, "rate_limit": true, "auth": true}
	if !validCategories[item.Category] {
		return fmt.Errorf("invalid category %q", item.Category)
	}
	if item.ConsumerFacing && item.Message == "" {
		return fmt.Errorf("consumer-facing error requires --message")
	}
	if err := store.UpsertErrorCode(item); err != nil {
		return err
	}
	fmt.Printf("saved %s\n", item.Code)
	return nil
}

func runErrorCodeDelete(cmd *cobra.Command, args []string) error {
	store, err := errorCodeStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	if err := store.DeleteErrorCode(args[0]); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", args[0])
	return nil
}
