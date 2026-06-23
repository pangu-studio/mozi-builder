// Package mozi provides the core types for the model-driven development platform.
// These types form the Intermediate Representation (IR) that bridges YAML model definitions
// and code generation templates. This package has zero dependencies on memflow project code.
package mozi

// ============================================================================
// Project-level types
// ============================================================================

// ProjectIR represents the entire project configuration and all its modules.
type ProjectIR struct {
	Name     string         `yaml:"name" json:"name"`
	Module   string         `yaml:"module" json:"module"` // Go module path
	Backend  BackendConfig  `yaml:"backend" json:"backend"`
	Frontend FrontendConfig `yaml:"frontend" json:"frontend"`
	Modules  []*ModuleIR    `json:"modules"`
}

// BackendConfig holds backend framework configuration.
type BackendConfig struct {
	Package   string `yaml:"package" json:"package"`     // Go package prefix, e.g., "memflow/cloud"
	Framework string `yaml:"framework" json:"framework"` // gin | echo | chi
	ORM       string `yaml:"orm" json:"orm"`             // ent | gorm | sqlx
}

// FrontendConfig holds frontend framework configuration.
type FrontendConfig struct {
	Framework      string `yaml:"framework" json:"framework"`             // react-antd | vue-element
	PackageManager string `yaml:"package_manager" json:"package_manager"` // npm | pnpm | yarn
}

// ============================================================================
// Module-level types
// ============================================================================

// ModuleIR represents a business module — a group of related models.
type ModuleIR struct {
	Name        string     `yaml:"module" json:"module"`
	Label       string     `yaml:"label" json:"label"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Icon        string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	APIPrefix   string     `yaml:"api_prefix" json:"api_prefix"` // Route prefix, e.g., "content" → /api/content/...
	Models      []*ModelIR `json:"models"`
}

// ============================================================================
// Field type & relation type enums
// ============================================================================

// FieldType represents the primitive data type of a model field.
type FieldType string

const (
	FieldTypeString FieldType = "string"
	FieldTypeInt    FieldType = "int"
	FieldTypeFloat  FieldType = "float"
	FieldTypeBool   FieldType = "bool"
	FieldTypeTime   FieldType = "time"
	FieldTypeText   FieldType = "text"
	FieldTypeEnum   FieldType = "enum"
	FieldTypeJSON   FieldType = "json"
)

// RelationType represents the type of relationship between models.
type RelationType string

const (
	RelationHasOne     RelationType = "has_one"
	RelationHasMany    RelationType = "has_many"
	RelationBelongsTo  RelationType = "belongs_to"
	RelationManyToMany RelationType = "many_to_many"
)

// GeneratedType represents how a primary field's value is generated.
type GeneratedType string

const (
	GeneratedUUID   GeneratedType = "uuid"
	GeneratedAuto   GeneratedType = "auto"
	GeneratedManual GeneratedType = "manual"
)

// ============================================================================
// ModelIR — single model definition
// ============================================================================

// ModelIR is the intermediate representation of a parsed model YAML file.
type ModelIR struct {
	Module      string          `yaml:"module,omitempty" json:"module"` // populated by parser from parent dir
	Name        string          `yaml:"model" json:"model"`
	Label       string          `yaml:"label" json:"label"`
	Description string          `yaml:"description,omitempty" json:"description,omitempty"`
	Table       string          `yaml:"table" json:"table"`
	Fields      []FieldIR       `yaml:"fields" json:"fields"`
	Relations   []RelationIR    `yaml:"relations,omitempty" json:"relations,omitempty"`
	Admin       AdminConfig     `yaml:"admin,omitempty" json:"admin,omitempty"`
	Display     DisplayConfig   `yaml:"display,omitempty" json:"display,omitempty"`
	Semantics   SemanticConfig  `yaml:"semantics,omitempty" json:"semantics,omitempty"`
	UIIntent    UIIntentConfig  `yaml:"ui_intent,omitempty" json:"ui_intent,omitempty"`
	APIIntent   APIIntentConfig `yaml:"api_intent,omitempty" json:"api_intent,omitempty"`
}

// FieldIR is the intermediate representation of a single field in a model.
type FieldIR struct {
	Name       string        `yaml:"name" json:"name"`
	Type       FieldType     `yaml:"type" json:"type"`
	Label      string        `yaml:"label" json:"label"`
	Required   bool          `yaml:"required,omitempty" json:"required,omitempty"`
	Unique     bool          `yaml:"unique,omitempty" json:"unique,omitempty"`
	Sensitive  bool          `yaml:"sensitive,omitempty" json:"sensitive,omitempty"`
	Searchable bool          `yaml:"searchable,omitempty" json:"searchable,omitempty"`
	Listable   bool          `yaml:"listable,omitempty" json:"listable,omitempty"`
	Editable   bool          `yaml:"editable,omitempty" json:"editable,omitempty"`
	Default    *string       `yaml:"default,omitempty" json:"default,omitempty"`
	EnumValues []string      `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
	FormType   string        `yaml:"form_type,omitempty" json:"form_type,omitempty"`
	AutoNowAdd bool          `yaml:"auto_now_add,omitempty" json:"auto_now_add,omitempty"`
	AutoNow    bool          `yaml:"auto_now,omitempty" json:"auto_now,omitempty"`
	Primary    bool          `yaml:"primary,omitempty" json:"primary,omitempty"`
	Generated  GeneratedType `yaml:"generated,omitempty" json:"generated,omitempty"`
	Sortable   bool          `yaml:"sortable,omitempty" json:"sortable,omitempty"`
}

// RelationIR is the intermediate representation of a relationship between two models.
type RelationIR struct {
	Name         string       `yaml:"name" json:"name"`
	Label        string       `yaml:"label,omitempty" json:"label,omitempty"`
	Type         RelationType `yaml:"type" json:"type"`
	Target       string       `yaml:"target" json:"target"` // "Module/Model" or "Model" (same module)
	TargetModule string       `json:"target_module"`        // parsed from Target (set by parser)
	TargetModel  string       `json:"target_model"`         // parsed from Target (set by parser)
	BackRef      string       `yaml:"back_ref,omitempty" json:"back_ref,omitempty"`
	Cascade      bool         `yaml:"cascade,omitempty" json:"cascade,omitempty"`
	Required     bool         `yaml:"required,omitempty" json:"required,omitempty"`
	Unique       bool         `yaml:"unique,omitempty" json:"unique,omitempty"`
}

// AdminConfig represents the admin panel display configuration for a model.
type AdminConfig struct {
	ListColumns  []string `yaml:"list_columns,omitempty" json:"list_columns,omitempty"`
	SearchFields []string `yaml:"search_fields,omitempty" json:"search_fields,omitempty"`
	DefaultSort  string   `yaml:"default_sort,omitempty" json:"default_sort,omitempty"`
	DefaultOrder string   `yaml:"default_order,omitempty" json:"default_order,omitempty"`
	PageSize     int      `yaml:"page_size,omitempty" json:"page_size,omitempty"`
}

// DisplayConfig represents display metadata for the model designer.
type DisplayConfig struct {
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// SemanticConfig captures product and domain meaning that cannot be inferred
// from table fields alone.
type SemanticConfig struct {
	Purpose       string   `yaml:"purpose,omitempty" json:"purpose,omitempty"`
	Audience      []string `yaml:"audience,omitempty" json:"audience,omitempty"`
	UserValue     string   `yaml:"user_value,omitempty" json:"user_value,omitempty"`
	BusinessRules []string `yaml:"business_rules,omitempty" json:"business_rules,omitempty"`
	Permissions   []string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Lifecycle     []string `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
}

// UIIntentConfig describes how the model should appear and behave in product UI.
type UIIntentConfig struct {
	ProductGoal      string                           `yaml:"product_goal,omitempty" json:"product_goal,omitempty"`
	UserTasks        []UIUserTaskConfig               `yaml:"user_tasks,omitempty" json:"user_tasks,omitempty"`
	Shared           UISharedIntentConfig             `yaml:"shared,omitempty" json:"shared,omitempty"`
	SurfacesConfig   map[string]UISurfaceIntentConfig `yaml:"surfaces_config,omitempty" json:"surfaces_config,omitempty"`
	Surfaces         []string                         `yaml:"surfaces,omitempty" json:"surfaces,omitempty"`
	PrimaryView      string                           `yaml:"primary_view,omitempty" json:"primary_view,omitempty"`
	PrimaryActions   []string                         `yaml:"primary_actions,omitempty" json:"primary_actions,omitempty"`
	ListIntent       string                           `yaml:"list_intent,omitempty" json:"list_intent,omitempty"`
	FormIntent       string                           `yaml:"form_intent,omitempty" json:"form_intent,omitempty"`
	DetailIntent     string                           `yaml:"detail_intent,omitempty" json:"detail_intent,omitempty"`
	EmptyState       string                           `yaml:"empty_state,omitempty" json:"empty_state,omitempty"`
	InteractionNotes []string                         `yaml:"interaction_notes,omitempty" json:"interaction_notes,omitempty"`
	SurfaceNotes     []string                         `yaml:"surface_notes,omitempty" json:"surface_notes,omitempty"`
}

// UIUserTaskConfig describes a cross-surface user task for a model.
type UIUserTaskConfig struct {
	Key      string `yaml:"key,omitempty" json:"key,omitempty"`
	Label    string `yaml:"label,omitempty" json:"label,omitempty"`
	Priority string `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// UISharedIntentConfig contains UI meaning shared across all surfaces.
type UISharedIntentConfig struct {
	PrimaryEntities []string          `yaml:"primary_entities,omitempty" json:"primary_entities,omitempty"`
	PrimaryActions  []string          `yaml:"primary_actions,omitempty" json:"primary_actions,omitempty"`
	EmptyState      string            `yaml:"empty_state,omitempty" json:"empty_state,omitempty"`
	Terminology     map[string]string `yaml:"terminology,omitempty" json:"terminology,omitempty"`
}

// UISurfaceIntentConfig contains UI strategy for one surface, such as admin,
// desktop, or miniapp.
type UISurfaceIntentConfig struct {
	Role         string                         `yaml:"role,omitempty" json:"role,omitempty"`
	EnabledTasks []string                       `yaml:"enabled_tasks,omitempty" json:"enabled_tasks,omitempty"`
	Views        map[string]UISurfaceViewConfig `yaml:"views,omitempty" json:"views,omitempty"`
	Actions      []string                       `yaml:"actions,omitempty" json:"actions,omitempty"`
	Constraints  []string                       `yaml:"constraints,omitempty" json:"constraints,omitempty"`
}

// UISurfaceViewConfig describes one view on a specific UI surface.
type UISurfaceViewConfig struct {
	Intent  string   `yaml:"intent,omitempty" json:"intent,omitempty"`
	Density string   `yaml:"density,omitempty" json:"density,omitempty"`
	Fields  []string `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// APIIntentConfig describes the public API contract expected for this model.
type APIIntentConfig struct {
	Exposure           string   `yaml:"exposure,omitempty" json:"exposure,omitempty"`
	Consumers          []string `yaml:"consumers,omitempty" json:"consumers,omitempty"`
	Auth               string   `yaml:"auth,omitempty" json:"auth,omitempty"`
	BasePath           string   `yaml:"base_path,omitempty" json:"base_path,omitempty"`
	Operations         []string `yaml:"operations,omitempty" json:"operations,omitempty"`
	RequestNotes       []string `yaml:"request_notes,omitempty" json:"request_notes,omitempty"`
	ResponseNotes      []string `yaml:"response_notes,omitempty" json:"response_notes,omitempty"`
	ErrorCases         []string `yaml:"error_cases,omitempty" json:"error_cases,omitempty"`
	Idempotency        string   `yaml:"idempotency,omitempty" json:"idempotency,omitempty"`
	RateLimit          string   `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	Versioning         string   `yaml:"versioning,omitempty" json:"versioning,omitempty"`
	CompatibilityNotes []string `yaml:"compatibility_notes,omitempty" json:"compatibility_notes,omitempty"`
}

// ============================================================================
// Helper methods
// ============================================================================

// ResolveRelationTarget parses a relation target like "content/Deck" or "Deck"
// and fills in TargetModule and TargetModel.
func (r *RelationIR) ResolveRelationTarget(currentModule string) {
	if r.TargetModule != "" && r.TargetModel != "" {
		return // already resolved
	}
	parts := splitTarget(r.Target)
	if len(parts) == 2 {
		r.TargetModule = parts[0]
		r.TargetModel = parts[1]
	} else {
		r.TargetModule = currentModule
		r.TargetModel = r.Target
	}
}

// splitTarget splits "module/Model" into [module, Model] or returns ["Model"].
func splitTarget(target string) []string {
	for i := len(target) - 1; i >= 0; i-- {
		if target[i] == '/' {
			return []string{target[:i], target[i+1:]}
		}
	}
	return []string{target}
}

// LookupTable builds a lookup table of models by full key "Module/Name".
func LookupTable(project *ProjectIR) map[string]*ModelIR {
	t := make(map[string]*ModelIR)
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			key := mod.Name + "/" + m.Name
			t[key] = m
			// Also register without module prefix for same-module lookups
			t[m.Name] = m
		}
	}
	return t
}

// GetField returns the field with the given name, or nil.
func (m *ModelIR) GetField(name string) *FieldIR {
	for i := range m.Fields {
		if m.Fields[i].Name == name {
			return &m.Fields[i]
		}
	}
	return nil
}

// ListableFields returns all fields marked as listable.
func (m *ModelIR) ListableFields() []FieldIR {
	var fields []FieldIR
	for _, f := range m.Fields {
		if f.Listable && !f.Sensitive {
			fields = append(fields, f)
		}
	}
	return fields
}

// EditableFields returns all fields that can be edited.
func (m *ModelIR) EditableFields() []FieldIR {
	var fields []FieldIR
	for _, f := range m.Fields {
		if f.Editable && !f.Primary && !f.AutoNowAdd && !f.AutoNow {
			fields = append(fields, f)
		}
	}
	return fields
}

// RequiredFields returns all fields marked as required.
func (m *ModelIR) RequiredFields() []FieldIR {
	var fields []FieldIR
	for _, f := range m.Fields {
		if f.Required && !f.Primary {
			fields = append(fields, f)
		}
	}
	return fields
}

// PrimaryField returns the primary key field, or nil.
func (m *ModelIR) PrimaryField() *FieldIR {
	for i := range m.Fields {
		if m.Fields[i].Primary {
			return &m.Fields[i]
		}
	}
	return nil
}

// ============================================================================
// Type mapping helpers
// ============================================================================

// GoType returns the Go type for the given FieldType.
func (ft FieldType) GoType() string {
	switch ft {
	case FieldTypeString, FieldTypeText, FieldTypeEnum:
		return "string"
	case FieldTypeInt:
		return "int"
	case FieldTypeFloat:
		return "float64"
	case FieldTypeBool:
		return "bool"
	case FieldTypeTime:
		return "time.Time"
	case FieldTypeJSON:
		return "[]byte"
	default:
		return "string"
	}
}

// TSType returns the TypeScript type for the given FieldType.
func (ft FieldType) TSType() string {
	switch ft {
	case FieldTypeString, FieldTypeText, FieldTypeEnum:
		return "string"
	case FieldTypeInt, FieldTypeFloat:
		return "number"
	case FieldTypeBool:
		return "boolean"
	case FieldTypeTime:
		return "string"
	case FieldTypeJSON:
		return "any"
	default:
		return "string"
	}
}

// EntFieldType returns the ent field method name for the given FieldType.
func (ft FieldType) EntFieldType() string {
	switch ft {
	case FieldTypeString:
		return "String"
	case FieldTypeInt:
		return "Int"
	case FieldTypeFloat:
		return "Float"
	case FieldTypeBool:
		return "Bool"
	case FieldTypeTime:
		return "Time"
	case FieldTypeText:
		return "Text"
	case FieldTypeEnum:
		return "Enum"
	case FieldTypeJSON:
		return "JSON"
	default:
		return "String"
	}
}
