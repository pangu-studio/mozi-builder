package generator

import (
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// TemplateContext is the rich context object passed to all code generation templates.
// It contains the model IR plus pre-computed derived values that templates commonly need.
type TemplateContext struct {
	// Model is the original model IR.
	Model *mozi.ModelIR

	// Module is the parent module IR (may be nil for single-model generation).
	Module *mozi.ModuleIR

	// Project is the project IR (may be nil for single-model generation).
	Project *mozi.ProjectIR

	// Name variants (model-level)
	Name       string // PascalCase: User
	NameLower  string // lowercase: user
	NameSnake  string // snake_case: user
	NamePlural string // snake_case plural: users
	NameCamel  string // camelCase: user
	NameKebab  string // kebab-case: user

	// Module-level name variants
	ModuleName      string // module name: content
	ModuleLabel     string // module label: 内容管理
	ModuleAPIPrefix string // module API prefix: content

	// Package path helpers
	ModulePath string // e.g., memflow/cloud
	ModelPkg   string // e.g., memflow/cloud/internal/model/content

	// Fields
	Fields         []mozi.FieldIR // All fields
	ListFields     []mozi.FieldIR // Listable fields
	EditableFields []mozi.FieldIR // Editable (form) fields
	RequiredFields []mozi.FieldIR // Required fields
	SearchFields   []mozi.FieldIR // Searchable fields
	PrimaryField   *mozi.FieldIR  // Primary key field
	HasTimeFields  bool           // Whether the model has time fields

	// Relations
	Relations     []mozi.RelationIR // All relations
	HasManyRels   []mozi.RelationIR
	BelongsToRels []mozi.RelationIR
	HasOneRels    []mozi.RelationIR
	HasRelations  bool

	// Admin config
	Admin mozi.AdminConfig

	// Generation metadata
	MarkerStart string
	MarkerEnd   string

	// Display
	DisplayIcon string // Icon, falling back to module icon
}

// BuildContext creates a TemplateContext from a ModelIR, computing all derived values.
func BuildContext(model *mozi.ModelIR) *TemplateContext {
	return BuildContextWithModule(model, nil, nil)
}

// BuildContextWithModule creates a TemplateContext with full module and project context.
func BuildContextWithModule(model *mozi.ModelIR, mod *mozi.ModuleIR, project *mozi.ProjectIR) *TemplateContext {
	ctx := &TemplateContext{
		Model:        model,
		Module:       mod,
		Project:      project,
		Name:         model.Name,
		NameLower:    strings.ToLower(model.Name),
		NameSnake:    moziToSnake(model.Name),
		NameCamel:    moziToCamel(model.Name),
		NameKebab:    moziToKebab(model.Name),
		Fields:       model.Fields,
		Relations:    model.Relations,
		HasRelations: len(model.Relations) > 0,
		Admin:        model.Admin,
		ModulePath:   "memflow/cloud",
	}

	ctx.NamePlural = ctx.NameSnake + "s"

	// Module context
	if mod != nil {
		ctx.ModuleName = mod.Name
		ctx.ModuleLabel = mod.Label
		ctx.ModuleAPIPrefix = mod.APIPrefix
	}
	if project != nil && project.Backend.Package != "" {
		ctx.ModulePath = project.Backend.Package
	}

	// Model package path
	if ctx.ModuleName != "" {
		ctx.ModelPkg = ctx.ModulePath + "/internal/model/" + ctx.ModuleName
	}

	// Display icon
	ctx.DisplayIcon = model.Display.Icon
	if ctx.DisplayIcon == "" && mod != nil {
		ctx.DisplayIcon = mod.Icon
	}

	// Categorize fields
	for _, f := range model.Fields {
		if f.Primary {
			ctx.PrimaryField = &f
		}
		if f.Listable && !f.Sensitive {
			ctx.ListFields = append(ctx.ListFields, f)
		}
		if f.Editable && !f.Primary && !f.AutoNowAdd && !f.AutoNow {
			ctx.EditableFields = append(ctx.EditableFields, f)
		}
		if f.Required && !f.Primary {
			ctx.RequiredFields = append(ctx.RequiredFields, f)
		}
		if f.Searchable {
			ctx.SearchFields = append(ctx.SearchFields, f)
		}
		if f.Type == mozi.FieldTypeTime {
			ctx.HasTimeFields = true
		}
	}

	// Categorize relations
	for _, r := range model.Relations {
		switch r.Type {
		case mozi.RelationHasMany:
			ctx.HasManyRels = append(ctx.HasManyRels, r)
		case mozi.RelationBelongsTo:
			ctx.BelongsToRels = append(ctx.BelongsToRels, r)
		case mozi.RelationHasOne:
			ctx.HasOneRels = append(ctx.HasOneRels, r)
		case mozi.RelationManyToMany:
			ctx.HasManyRels = append(ctx.HasManyRels, r)
		}
	}

	// Marker comments
	ctx.MarkerStart = "// mozi:section"
	ctx.MarkerEnd = "// mozi:end"

	return ctx
}

// ============================================================================
// Naming helpers
// ============================================================================

// FkFieldName returns the foreign key field name for a belongs_to relation.
func (ctx *TemplateContext) FkFieldName(rel mozi.RelationIR) string {
	return rel.Name + "_id"
}

// TargetModelSnake returns the snake_case of the target model name for a relation.
func (ctx *TemplateContext) TargetModelSnake(rel mozi.RelationIR) string {
	return moziToSnake(rel.TargetModel)
}

// GoTypeForField returns the Go type string for a field.
func GoTypeForField(f mozi.FieldIR) string {
	if f.Primary && f.Generated == mozi.GeneratedUUID {
		return "string"
	}
	return f.Type.GoType()
}

// TSTypeForField returns the TypeScript type string for a field.
func TSTypeForField(f mozi.FieldIR) string {
	return f.Type.TSType()
}

// EntFieldMethod returns the ent field builder method for a field.
func EntFieldMethod(f mozi.FieldIR) string {
	return f.Type.EntFieldType()
}

// AdminServiceName returns the admin service struct name for a model.
func AdminServiceName(modelName string) string {
	return "Admin" + modelName + "Service"
}

// AdminHandlerName returns the admin handler struct name for a model.
func AdminHandlerName(modelName string) string {
	return "Admin" + modelName + "Handler"
}

// OutputFileName returns the file name for generated code based on model name.
func OutputFileName(modelName string) string {
	return moziToSnake(modelName) + ".go"
}

// AdminOutputFileName returns the admin handler/service output file name.
func AdminOutputFileName(modelName string) string {
	return "admin_" + moziToSnake(modelName) + ".go"
}

// EntSchemaFileName returns the ent schema file name: {model_snake}.go
// Note: ent requires unique schema type names across all files. Using just the model
// name ensures the generated file overwrites any previous schema for the same model.
func EntSchemaFileName(moduleName, modelName string) string {
	return moziToSnake(modelName) + ".go"
}

// ModelPackagePath returns the package path for internal/model/{module}/
func ModelPackagePath(modulePath, moduleName string) string {
	return modulePath + "/internal/model/" + moduleName
}

// ServicePackagePath returns the package path for internal/service/{module}/
func ServicePackagePath(modulePath, moduleName string) string {
	return modulePath + "/internal/service/" + moduleName
}

// HandlerPackagePath returns the package path for internal/handler/{module}/
func HandlerPackagePath(modulePath, moduleName string) string {
	return modulePath + "/internal/handler/" + moduleName
}
