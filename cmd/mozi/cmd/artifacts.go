package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/apicontract"
	"github.com/pangu-studio/mozi-builder/mozi/db"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
	mozii18n "github.com/pangu-studio/mozi-builder/mozi/i18n"
	"github.com/pangu-studio/mozi-builder/mozi/migration"
	"github.com/pangu-studio/mozi-builder/mozi/rbac"
	"github.com/pangu-studio/mozi-builder/mozi/sdk"
	"github.com/spf13/cobra"
)

var artifactsCmd = &cobra.Command{Use: "artifacts", Short: "Generate reviewed contract artifacts"}
var artifactModel, artifactInput, artifactOutput, artifactLocale string

func init() {
	migrationCmd := &cobra.Command{Use: "migration", Short: "Generate safe SQL migration files", RunE: runArtifactMigration}
	migrationCmd.Flags().StringVarP(&artifactModel, "model", "m", "", "module/Model")
	migrationCmd.Flags().StringVarP(&artifactOutput, "out", "o", "migrations", "output directory")
	_ = migrationCmd.MarkFlagRequired("model")
	i18nCmd := &cobra.Command{Use: "i18n", Short: "Export an i18n source catalog", RunE: runArtifactI18n}
	i18nCmd.Flags().StringVarP(&artifactOutput, "out", "o", "locales/source.json", "output JSON")
	i18nCmd.Flags().StringVar(&artifactLocale, "locale", "zh-CN", "source locale")
	i18nValidateCmd := &cobra.Command{Use: "i18n-validate", Short: "Validate a translation JSON object", RunE: runArtifactI18nValidate}
	i18nValidateCmd.Flags().StringVarP(&artifactInput, "input", "i", "", "translation JSON")
	_ = i18nValidateCmd.MarkFlagRequired("input")
	i18nValidateCmd.Flags().StringVar(&artifactLocale, "locale", "en", "translation locale")
	sdkCmd := &cobra.Command{Use: "typescript-sdk", Short: "Generate a TypeScript SDK from OpenAPI", RunE: runArtifactSDK}
	sdkCmd.Flags().StringVarP(&artifactInput, "openapi", "i", "docs/swagger.json", "OpenAPI JSON")
	sdkCmd.Flags().StringVarP(&artifactOutput, "out", "o", "sdk/typescript/client.ts", "output file")
	contractCmd := &cobra.Command{Use: "bruno", Short: "Generate Bruno contract cases", RunE: runArtifactBruno}
	contractCmd.Flags().StringVarP(&artifactModel, "model", "m", "", "module/Model")
	contractCmd.Flags().StringVarP(&artifactInput, "openapi", "i", "docs/swagger.json", "OpenAPI JSON")
	contractCmd.Flags().StringVarP(&artifactOutput, "out", "o", "contracts/bruno", "output directory")
	_ = contractCmd.MarkFlagRequired("model")
	permissionCmd := &cobra.Command{Use: "permissions", Short: "Generate Go permission constants", RunE: runArtifactPermissions}
	permissionCmd.Flags().StringVarP(&artifactModel, "model", "m", "", "module/Model")
	permissionCmd.Flags().StringVarP(&artifactOutput, "out", "o", "internal/permissions/generated.go", "output file")
	_ = permissionCmd.MarkFlagRequired("model")
	artifactsCmd.AddCommand(migrationCmd, i18nCmd, i18nValidateCmd, sdkCmd, contractCmd, permissionCmd)
	rootCmd.AddCommand(artifactsCmd)
}

func artifactStore() (*db.Store, error) {
	dsn := os.Getenv("MOZI_DB")
	if dsn == "" {
		dsn = db.DefaultDesignDB
	}
	return openStore(dsn)
}
func artifactModelFromStore(store *db.Store, ref string) (*mozi.ModelIR, error) {
	_, name := parseModelRef(ref)
	return store.LoadModel(name)
}
func writeArtifact(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func runArtifactMigration(cmd *cobra.Command, args []string) error {
	store, err := artifactStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	model, err := artifactModelFromStore(store, artifactModel)
	if err != nil {
		return err
	}
	_, name := parseModelRef(artifactModel)
	_, _, _, _, _, version, err := store.GetModel(name)
	if err != nil {
		return err
	}
	prevVersion, err := store.PreviousVersion(name, version)
	if err != nil {
		return err
	}
	if prevVersion == "" {
		return fmt.Errorf("first model version requires a reviewed create-table migration")
	}
	prev, err := store.LoadModelVersion(name, prevVersion, model.Module)
	if err != nil {
		return err
	}
	diff := differ.Compare(prev, model, prevVersion, version)
	files, err := migration.RenderSafe(migration.Advise(prev, model, diff), time.Now().Format("20060102150405"), "update_"+name)
	if err != nil {
		return err
	}
	if err := writeArtifact(filepath.Join(artifactOutput, files.BaseName+".up.sql"), []byte(files.Up)); err != nil {
		return err
	}
	if err := writeArtifact(filepath.Join(artifactOutput, files.BaseName+".down.sql"), []byte(files.Down)); err != nil {
		return err
	}
	fmt.Println(files.BaseName)
	return nil
}
func runArtifactI18n(cmd *cobra.Command, args []string) error {
	store, err := artifactStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	project, err := store.LoadProject()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(mozii18n.Extract(project, artifactLocale), "", "  ")
	if err != nil {
		return err
	}
	return writeArtifact(artifactOutput, append(data, '\n'))
}
func runArtifactI18nValidate(cmd *cobra.Command, args []string) error {
	store, err := artifactStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	project, err := store.LoadProject()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(artifactInput)
	if err != nil {
		return err
	}
	translations := map[string]string{}
	if err := json.Unmarshal(data, &translations); err != nil {
		return fmt.Errorf("translation file must be a JSON object of string values: %w", err)
	}
	report := mozii18n.Validate(mozii18n.Extract(project, artifactLocale), translations)
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	if len(report.Missing) > 0 || len(report.PlaceholderErrors) > 0 {
		return fmt.Errorf("translation validation failed")
	}
	return nil
}
func runArtifactSDK(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(artifactInput)
	if err != nil {
		return err
	}
	out, err := sdk.GenerateTypeScript(data)
	if err != nil {
		return err
	}
	return writeArtifact(artifactOutput, []byte(out))
}
func runArtifactBruno(cmd *cobra.Command, args []string) error {
	store, err := artifactStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	model, err := artifactModelFromStore(store, artifactModel)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(artifactInput)
	if err != nil {
		return err
	}
	endpoints, err := apicontract.ParseOpenAPI(data)
	if err != nil {
		return err
	}
	artifacts, err := apicontract.GenerateBruno(model, endpoints)
	if err != nil {
		return err
	}
	for _, a := range artifacts {
		if err := writeArtifact(filepath.Join(artifactOutput, a.Path), []byte(a.Content)); err != nil {
			return err
		}
	}
	return nil
}
func runArtifactPermissions(cmd *cobra.Command, args []string) error {
	store, err := artifactStore()
	if err != nil {
		return err
	}
	defer store.DB.Close()
	model, err := artifactModelFromStore(store, artifactModel)
	if err != nil {
		return err
	}
	return writeArtifact(artifactOutput, []byte(rbac.GenerateGo(model)))
}
