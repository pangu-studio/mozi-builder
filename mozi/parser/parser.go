// Package parser provides YAML model definition parsing.
// It reads model YAML files and produces mozi.ModelIR intermediate representations.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// Project-level parsing
// ============================================================================

// ParseProject reads the entire models/ directory tree and returns a ProjectIR.
func ParseProject(dir string) (*mozi.ProjectIR, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("read models directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	project := &mozi.ProjectIR{}

	// Try to read _project.yaml
	projectPath := filepath.Join(dir, "_project.yaml")
	if data, err := os.ReadFile(projectPath); err == nil {
		if err := yaml.Unmarshal(data, project); err != nil {
			return nil, fmt.Errorf("parse _project.yaml: %w", err)
		}
	}
	applyProjectDefaults(project)

	// Scan for module directories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read models directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip .mozi etc
		}

		modDir := filepath.Join(dir, name)
		mod, err := ParseModule(modDir)
		if err != nil {
			return nil, fmt.Errorf("parse module %s: %w", name, err)
		}
		project.Modules = append(project.Modules, mod)
	}

	return project, nil
}

func applyProjectDefaults(p *mozi.ProjectIR) {
	if p.SchemaVersion == 0 {
		p.SchemaVersion = mozi.CurrentSchemaVersion
	}
	if p.Name == "" {
		p.Name = filepath.Base(".")
	}
	if p.Backend.Package == "" {
		p.Backend.Package = p.Module
	}
	if p.Backend.Framework == "" {
		p.Backend.Framework = "gin"
	}
	if p.Backend.ORM == "" {
		p.Backend.ORM = "ent"
	}
	if p.Frontend.Framework == "" {
		p.Frontend.Framework = "react-antd"
	}
}

// ============================================================================
// Module-level parsing
// ============================================================================

// ParseModule reads a single module directory and returns a ModuleIR.
func ParseModule(dir string) (*mozi.ModuleIR, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("read module directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	modName := filepath.Base(dir)
	mod := &mozi.ModuleIR{
		Name: modName,
	}

	// Try to read _module.yaml
	modConfigPath := filepath.Join(dir, "_module.yaml")
	if data, err := os.ReadFile(modConfigPath); err == nil {
		if err := yaml.Unmarshal(data, mod); err != nil {
			return nil, fmt.Errorf("parse _module.yaml in %s: %w", dir, err)
		}
	}
	applyModuleDefaults(mod)

	// Parse model YAML files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read module directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue // skip config files and hidden files
		}
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		modelPath := filepath.Join(dir, name)
		model, err := ParseFile(modelPath, mod.Name)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		mod.Models = append(mod.Models, model)
	}

	return mod, nil
}

func applyModuleDefaults(mod *mozi.ModuleIR) {
	if mod.Name == "" {
		return
	}
	if mod.Label == "" {
		mod.Label = mod.Name
	}
	if mod.APIPrefix == "" {
		mod.APIPrefix = mod.Name
	}
}

// ============================================================================
// Single model parsing
// ============================================================================

// ParseFile reads a single YAML model file and returns its ModelIR.
// moduleName is the name of the module this model belongs to.
func ParseFile(path string, moduleName string) (*mozi.ModelIR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read model file %s: %w", path, err)
	}
	return ParseFileFromContent(data, moduleName)
}

// ParseFileFromContent parses YAML content directly (used by dev platform API).
func ParseFileFromContent(data []byte, moduleName string) (*mozi.ModelIR, error) {
	var model mozi.ModelIR
	if err := yaml.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("parse model YAML: %w", err)
	}

	NormalizeModel(&model, moduleName)
	return &model, nil
}

// NormalizeModel fills defaults and resolved relation fields on an in-memory ModelIR.
func NormalizeModel(model *mozi.ModelIR, moduleName string) {
	// Set module (from parent directory, can be overridden by explicit field)
	if model.Module == "" {
		model.Module = moduleName
	}

	// Apply defaults
	applyModelDefaults(model)

	// Resolve relation targets
	for i := range model.Relations {
		model.Relations[i].ResolveRelationTarget(model.Module)
	}
}

func applyModelDefaults(model *mozi.ModelIR) {
	if model.SchemaVersion == 0 {
		model.SchemaVersion = mozi.CurrentSchemaVersion
	}
	// Default table name from model name (snake_case)
	if model.Table == "" {
		model.Table = toSnakeCase(model.Name) + "s"
	}
	// Default label from model name
	if model.Label == "" {
		model.Label = model.Name
	}
	// Default admin config
	if model.Admin.PageSize == 0 {
		model.Admin.PageSize = 20
	}
	if model.Admin.DefaultOrder == "" {
		model.Admin.DefaultOrder = "desc"
	}
	// Auto-populate list_columns from listable fields
	if len(model.Admin.ListColumns) == 0 {
		for _, f := range model.Fields {
			if f.Listable && !f.Sensitive {
				model.Admin.ListColumns = append(model.Admin.ListColumns, f.Name)
			}
		}
	}
	// Auto-populate search_fields from searchable fields
	if len(model.Admin.SearchFields) == 0 {
		for _, f := range model.Fields {
			if f.Searchable {
				model.Admin.SearchFields = append(model.Admin.SearchFields, f.Name)
			}
		}
	}
	// Auto-populate default_sort
	if model.Admin.DefaultSort == "" {
		model.Admin.DefaultSort = "created_at"
	}

	// Apply field defaults
	for i := range model.Fields {
		f := &model.Fields[i]
		// Default form_type from field type
		if f.FormType == "" {
			f.FormType = defaultFormType(f.Type)
		}
	}
}

// ParseDir is a convenience function that flattens all modules into a model list.
// Kept for backward compatibility with simpler use cases.
func ParseDir(dir string) ([]*mozi.ModelIR, error) {
	project, err := ParseProject(dir)
	if err != nil {
		return nil, err
	}
	var models []*mozi.ModelIR
	for _, mod := range project.Modules {
		models = append(models, mod.Models...)
	}
	return models, nil
}

// ============================================================================
// Helpers
// ============================================================================

func defaultFormType(ft mozi.FieldType) string {
	switch ft {
	case mozi.FieldTypeString:
		return "text"
	case mozi.FieldTypeInt, mozi.FieldTypeFloat:
		return "number"
	case mozi.FieldTypeBool:
		return "switch"
	case mozi.FieldTypeTime:
		return "date"
	case mozi.FieldTypeText:
		return "textarea"
	case mozi.FieldTypeEnum:
		return "select"
	case mozi.FieldTypeJSON:
		return "textarea"
	default:
		return "text"
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
