package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/db"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	exportDir   string
	exportModel string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export models from the design database to YAML files",
	Long: `Reads model definitions from the design database and writes them as YAML files.

Examples:
  mozi export --dir models/                # Export all to models/
  mozi export --module content             # Export only the content module
  mozi export --model content/Card         # Export a single model`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportDir, "dir", "", "Output directory for YAML files")
	exportCmd.Flags().StringVar(&genModule, "module", "", "Export only a specific module")
	exportCmd.Flags().StringVarP(&exportModel, "model", "m", "", "Export a single model (module/ModelName)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	designDB := os.Getenv("MOZI_DB")
	if designDB == "" {
		designDB = db.DefaultDesignDB
	}

	store, err := openStore(designDB)
	if err != nil {
		return err
	}
	defer store.DB.Close()

	project, err := store.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}

	outDir := exportDir
	if outDir == "" {
		projectRoot, err := findProjectRoot()
		if err != nil {
			return fmt.Errorf("could not find project root: %w", err)
		}
		outDir = resolveModelsDir(projectRoot)
	}
	if exportModel == "" && genModule == "" {
		if err := writeProjectYAML(outDir, project); err != nil {
			return err
		}
	}

	// Filter modules if specified
	modules := project.Modules
	if exportModel != "" {
		// Export a single model
		modName, modelName := parseModelRef(exportModel)
		modules = filterModulesToModel(project.Modules, modName, modelName)
		if len(modules) == 0 {
			return fmt.Errorf("model '%s' not found", exportModel)
		}
	} else if genModule != "" {
		modules = nil
		for _, m := range project.Modules {
			if m.Name == genModule {
				modules = append(modules, m)
				break
			}
		}
		if len(modules) == 0 {
			return fmt.Errorf("module '%s' not found", genModule)
		}
	}

	total := 0
	for _, mod := range modules {
		modDir := filepath.Join(outDir, mod.Name)
		if err := os.MkdirAll(modDir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", modDir, err)
		}

		// Write _module.yaml
		modYAML := marshalModuleYAML(mod)
		modPath := filepath.Join(modDir, "_module.yaml")
		if err := os.WriteFile(modPath, []byte(modYAML), 0644); err != nil {
			return fmt.Errorf("write %s: %w", modPath, err)
		}
		fmt.Printf("  Module: %s (%s)\n", mod.Name, mod.Label)

		for _, model := range mod.Models {
			modelYAML := marshalModelYAML(model)
			modelPath := filepath.Join(modDir, toSnake(model.Name)+".yaml")
			if err := os.WriteFile(modelPath, []byte(modelYAML), 0644); err != nil {
				return fmt.Errorf("write %s: %w", modelPath, err)
			}
			fmt.Printf("    ✓ %s\n", model.Name)
			total++
		}
	}

	fmt.Printf("\n✅ Exported %d model(s) to %s\n", total, outDir)
	return nil
}

func writeProjectYAML(outDir string, project *mozi.ProjectIR) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	payload := struct {
		SchemaVersion int                 `yaml:"schema_version"`
		Name          string              `yaml:"name"`
		Module        string              `yaml:"module,omitempty"`
		Backend       mozi.BackendConfig  `yaml:"backend,omitempty"`
		Frontend      mozi.FrontendConfig `yaml:"frontend,omitempty"`
		ErrorCodes    []mozi.ErrorCodeIR  `yaml:"error_codes,omitempty"`
	}{project.SchemaVersion, project.Name, project.Module, project.Backend, project.Frontend, project.ErrorCodes}
	data, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal _project.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "_project.yaml"), data, 0644); err != nil {
		return fmt.Errorf("write _project.yaml: %w", err)
	}
	return nil
}

// marshalModuleYAML converts a ModuleIR to YAML string.
func marshalModuleYAML(mod *mozi.ModuleIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n", mod.Label))
	sb.WriteString(fmt.Sprintf("module: %s\n", mod.Name))
	sb.WriteString(fmt.Sprintf("label: %s\n", mod.Label))
	if mod.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", mod.Description))
	}
	if mod.Icon != "" {
		sb.WriteString(fmt.Sprintf("icon: %s\n", mod.Icon))
	}
	sb.WriteString(fmt.Sprintf("api_prefix: %s\n", mod.APIPrefix))
	return sb.String()
}

// marshalModelYAML converts a ModelIR to YAML string.
func marshalModelYAML(model *mozi.ModelIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s — %s\n", model.Name, model.Label))
	if model.Description != "" {
		sb.WriteString(fmt.Sprintf("# %s\n", model.Description))
	}
	sb.WriteString(fmt.Sprintf("module: %s\n", model.Module))
	sb.WriteString(fmt.Sprintf("model: %s\n", model.Name))
	sb.WriteString(fmt.Sprintf("label: %s\n", model.Label))
	sb.WriteString(fmt.Sprintf("description: %s\n", model.Description))
	sb.WriteString(fmt.Sprintf("table: %s\n", model.Table))

	if hasSemanticConfig(model.Semantics) {
		sb.WriteString("\nsemantics:\n")
		if model.Semantics.Purpose != "" {
			sb.WriteString(fmt.Sprintf("  purpose: %q\n", model.Semantics.Purpose))
		}
		writeYAMLStringList(&sb, "audience", model.Semantics.Audience)
		if model.Semantics.UserValue != "" {
			sb.WriteString(fmt.Sprintf("  user_value: %q\n", model.Semantics.UserValue))
		}
		writeYAMLStringList(&sb, "business_rules", model.Semantics.BusinessRules)
		writeYAMLStringList(&sb, "permissions", model.Semantics.Permissions)
		if len(model.Semantics.PermissionRules) > 0 {
			data, _ := yaml.Marshal(model.Semantics.PermissionRules)
			sb.WriteString("  permission_rules:\n")
			for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
				sb.WriteString("    " + line + "\n")
			}
		}
		writeYAMLStringList(&sb, "lifecycle", model.Semantics.Lifecycle)
	}

	if hasUIIntentConfig(model.UIIntent) {
		sb.WriteString("\nui_intent:\n")
		if model.UIIntent.PrimaryView != "" {
			sb.WriteString(fmt.Sprintf("  primary_view: %s\n", model.UIIntent.PrimaryView))
		}
		writeYAMLStringList(&sb, "primary_actions", model.UIIntent.PrimaryActions)
		if model.UIIntent.ListIntent != "" {
			sb.WriteString(fmt.Sprintf("  list_intent: %q\n", model.UIIntent.ListIntent))
		}
		if model.UIIntent.FormIntent != "" {
			sb.WriteString(fmt.Sprintf("  form_intent: %q\n", model.UIIntent.FormIntent))
		}
		if model.UIIntent.DetailIntent != "" {
			sb.WriteString(fmt.Sprintf("  detail_intent: %q\n", model.UIIntent.DetailIntent))
		}
		if model.UIIntent.EmptyState != "" {
			sb.WriteString(fmt.Sprintf("  empty_state: %q\n", model.UIIntent.EmptyState))
		}
		writeYAMLStringList(&sb, "interaction_notes", model.UIIntent.InteractionNotes)
	}
	if !reflect.DeepEqual(model.APIIntent, mozi.APIIntentConfig{}) {
		data, _ := yaml.Marshal(model.APIIntent)
		sb.WriteString("\napi_intent:\n")
		for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			sb.WriteString("  " + line + "\n")
		}
	}

	// Fields
	sb.WriteString("\nfields:\n")
	for _, f := range model.Fields {
		sb.WriteString(fmt.Sprintf("  - name: %s\n", f.Name))
		sb.WriteString(fmt.Sprintf("    type: %s\n", f.Type))
		sb.WriteString(fmt.Sprintf("    label: %s\n", f.Label))
		if f.I18nKey != "" {
			sb.WriteString(fmt.Sprintf("    i18n_key: %s\n", f.I18nKey))
		}
		if f.Primary {
			sb.WriteString("    primary: true\n")
		}
		if f.Generated != "" && f.Generated != "manual" {
			sb.WriteString(fmt.Sprintf("    generated: %s\n", f.Generated))
		}
		if f.Required {
			sb.WriteString("    required: true\n")
		}
		if f.Unique {
			sb.WriteString("    unique: true\n")
		}
		if f.Sensitive {
			sb.WriteString("    sensitive: true\n")
		}
		if f.Default != nil && *f.Default != "" {
			sb.WriteString(fmt.Sprintf("    default: \"%s\"\n", *f.Default))
		} else if f.Default != nil {
			sb.WriteString("    default: \"\"\n")
		}
		if len(f.EnumValues) > 0 {
			sb.WriteString(fmt.Sprintf("    enum_values: [%s]\n", strings.Join(f.EnumValues, ", ")))
		}
		if f.FormType != "" && f.FormType != "text" {
			sb.WriteString(fmt.Sprintf("    form_type: %s\n", f.FormType))
		}
		if f.Searchable {
			sb.WriteString("    searchable: true\n")
		}
		if !f.Listable {
			sb.WriteString("    listable: false\n")
		}
		if !f.Editable {
			sb.WriteString("    editable: false\n")
		}
		if f.AutoNowAdd {
			sb.WriteString("    auto_now_add: true\n")
		}
		if f.AutoNow {
			sb.WriteString("    auto_now: true\n")
		}
	}

	// Relations
	if len(model.Relations) > 0 {
		sb.WriteString("\nrelations:\n")
		for _, r := range model.Relations {
			sb.WriteString(fmt.Sprintf("  - name: %s\n", r.Name))
			if r.Label != "" {
				sb.WriteString(fmt.Sprintf("    label: %s\n", r.Label))
			}
			sb.WriteString(fmt.Sprintf("    type: %s\n", r.Type))
			sb.WriteString(fmt.Sprintf("    target: %s\n", r.Target))
			if r.BackRef != "" {
				sb.WriteString(fmt.Sprintf("    back_ref: %s\n", r.BackRef))
			}
			if r.Cascade {
				sb.WriteString("    cascade: true\n")
			}
		}
	}

	// Admin config
	sb.WriteString("\nadmin:\n")
	if len(model.Admin.ListColumns) > 0 {
		sb.WriteString(fmt.Sprintf("  list_columns: [%s]\n", strings.Join(model.Admin.ListColumns, ", ")))
	}
	if len(model.Admin.SearchFields) > 0 {
		sb.WriteString(fmt.Sprintf("  search_fields: [%s]\n", strings.Join(model.Admin.SearchFields, ", ")))
	}
	sb.WriteString(fmt.Sprintf("  default_sort: %s\n", model.Admin.DefaultSort))
	sb.WriteString(fmt.Sprintf("  default_order: %s\n", model.Admin.DefaultOrder))
	sb.WriteString(fmt.Sprintf("  page_size: %d\n", model.Admin.PageSize))

	// Display
	if model.Display.Icon != "" {
		sb.WriteString(fmt.Sprintf("\ndisplay:\n  icon: %s\n", model.Display.Icon))
	}

	return sb.String()
}

func writeYAMLStringList(sb *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("  %s:\n", key))
	for _, value := range values {
		sb.WriteString(fmt.Sprintf("    - %q\n", value))
	}
}

func hasSemanticConfig(s mozi.SemanticConfig) bool {
	return s.Purpose != "" || s.UserValue != "" || len(s.Audience) > 0 ||
		len(s.BusinessRules) > 0 || len(s.Permissions) > 0 || len(s.PermissionRules) > 0 || len(s.Lifecycle) > 0
}

func hasUIIntentConfig(s mozi.UIIntentConfig) bool {
	return s.PrimaryView != "" || len(s.PrimaryActions) > 0 || s.ListIntent != "" ||
		s.FormIntent != "" || s.DetailIntent != "" || s.EmptyState != "" ||
		len(s.InteractionNotes) > 0
}

func toSnake(s string) string {
	var r strings.Builder
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				r.WriteByte('_')
			}
			r.WriteRune(c + 32)
		} else {
			r.WriteRune(c)
		}
	}
	return r.String()
}

// filterModulesToModel filters the project to only the specified model within its module.
func filterModulesToModel(modules []*mozi.ModuleIR, modName, modelName string) []*mozi.ModuleIR {
	for _, mod := range modules {
		if mod.Name == modName {
			for _, m := range mod.Models {
				if m.Name == modelName {
					return []*mozi.ModuleIR{{
						Name:   mod.Name,
						Label:  mod.Label,
						Models: []*mozi.ModelIR{m},
					}}
				}
			}
		}
	}
	return nil
}
