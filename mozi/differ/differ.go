// Package differ provides structured field-level diff between model versions.
// It compares two ModelIRs and produces a detailed change report.
package differ

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pangu-sutido/mozi-builder/mozi"
)

// DiffResult holds the structured difference between two model versions.
type DiffResult struct {
	ModelRef    string        `json:"model_ref"`    // "module/Model"
	FromVersion string        `json:"from_version"` // source version
	ToVersion   string        `json:"to_version"`   // target version
	Changes     []FieldChange `json:"changes"`
	HasChanges  bool          `json:"has_changes"`
}

// FieldChange represents a single change to a model field.
type FieldChange struct {
	Type     ChangeType `json:"type"`                // added, removed, modified
	Category string     `json:"category"`            // field, relation, admin
	Name     string     `json:"name"`                // field or relation name
	Detail   string     `json:"detail"`              // human-readable description
	OldValue string     `json:"old_value,omitempty"` // previous value (for modified)
	NewValue string     `json:"new_value,omitempty"` // new value (for modified)
}

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

// AffectedFile represents a file that would be affected by the changes.
type AffectedFile struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	ChangeCount int    `json:"change_count"`
}

// Compare compares two ModelIRs and returns a structured diff.
func Compare(from, to *mozi.ModelIR, fromVersion, toVersion string) *DiffResult {
	ref := to.Module + "/" + to.Name
	result := &DiffResult{
		ModelRef:    ref,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}

	changes := compareFields(from.Fields, to.Fields)
	changes = append(changes, compareRelations(from.Relations, to.Relations)...)
	changes = append(changes, compareAdmin(from.Admin, to.Admin)...)
	changes = append(changes, compareModelMeta(from, to)...)
	changes = append(changes, compareSemantics(from.Semantics, to.Semantics)...)
	changes = append(changes, compareUIIntent(from.UIIntent, to.UIIntent)...)
	changes = append(changes, compareAPIIntent(from.APIIntent, to.APIIntent)...)

	result.Changes = changes
	result.HasChanges = len(changes) > 0
	return result
}

// compareModelMeta compares model-level metadata (label, description, table, display).
// Table changes are flagged with a migration warning because they imply database schema changes.
func compareModelMeta(from, to *mozi.ModelIR) []FieldChange {
	var changes []FieldChange

	if from.Label != to.Label {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "meta",
			Name:     "label",
			Detail:   fmt.Sprintf("~ label changed: %s → %s", from.Label, to.Label),
			OldValue: from.Label,
			NewValue: to.Label,
		})
	}
	if from.Description != to.Description {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "meta",
			Name:     "description",
			Detail:   "~ description changed",
			OldValue: from.Description,
			NewValue: to.Description,
		})
	}
	if from.Table != to.Table {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "meta",
			Name:     "table",
			Detail:   fmt.Sprintf("~ table changed: %s → %s ⚠️ REQUIRES DB MIGRATION", from.Table, to.Table),
			OldValue: from.Table,
			NewValue: to.Table,
		})
	}
	if from.Display.Icon != to.Display.Icon {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "meta",
			Name:     "display",
			Detail:   fmt.Sprintf("~ display icon changed: %s → %s", from.Display.Icon, to.Display.Icon),
			OldValue: from.Display.Icon,
			NewValue: to.Display.Icon,
		})
	}

	return changes
}

func compareSemantics(from, to mozi.SemanticConfig) []FieldChange {
	if reflect.DeepEqual(from, to) {
		return nil
	}
	return []FieldChange{
		{
			Type:     ChangeModified,
			Category: "semantics",
			Name:     "semantics",
			Detail:   "~ semantics changed",
			OldValue: semanticSummary(from),
			NewValue: semanticSummary(to),
		},
	}
}

func compareUIIntent(from, to mozi.UIIntentConfig) []FieldChange {
	if reflect.DeepEqual(from, to) {
		return nil
	}
	var changes []FieldChange
	if from.ProductGoal != to.ProductGoal || !reflect.DeepEqual(from.UserTasks, to.UserTasks) || !reflect.DeepEqual(from.Shared, to.Shared) {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "ui_intent",
			Name:     "shared",
			Detail:   "~ ui_intent.shared changed",
			OldValue: uiSharedSummary(from),
			NewValue: uiSharedSummary(to),
		})
	}

	for _, surface := range changedUISurfaces(from.SurfacesConfig, to.SurfacesConfig) {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "ui_intent",
			Name:     "surfaces." + surface,
			Detail:   fmt.Sprintf("~ ui_intent.surfaces_config.%s changed", surface),
			OldValue: uiSurfaceSummary(from.SurfacesConfig[surface]),
			NewValue: uiSurfaceSummary(to.SurfacesConfig[surface]),
		})
	}

	if legacyUIIntentChanged(from, to) {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "ui_intent",
			Name:     "legacy",
			Detail:   "~ legacy ui_intent fields changed",
			OldValue: uiLegacyIntentSummary(from),
			NewValue: uiLegacyIntentSummary(to),
		})
	}

	if len(changes) == 0 {
		changes = append(changes, FieldChange{
			Type:     ChangeModified,
			Category: "ui_intent",
			Name:     "ui_intent",
			Detail:   "~ ui_intent changed",
			OldValue: uiIntentSummary(from),
			NewValue: uiIntentSummary(to),
		})
	}
	return changes
}

func compareAPIIntent(from, to mozi.APIIntentConfig) []FieldChange {
	if reflect.DeepEqual(from, to) {
		return nil
	}
	return []FieldChange{
		{
			Type:     ChangeModified,
			Category: "api_intent",
			Name:     "api_intent",
			Detail:   "~ api_intent changed",
			OldValue: apiIntentSummary(from),
			NewValue: apiIntentSummary(to),
		},
	}
}

// compareFields detects added, removed, and modified fields.
func compareFields(from, to []mozi.FieldIR) []FieldChange {
	fromMap := make(map[string]mozi.FieldIR)
	for _, f := range from {
		fromMap[f.Name] = f
	}
	toMap := make(map[string]mozi.FieldIR)
	for _, f := range to {
		toMap[f.Name] = f
	}

	var changes []FieldChange

	// Detect added and modified
	for name, tf := range toMap {
		ff, exists := fromMap[name]
		if !exists {
			changes = append(changes, FieldChange{
				Type:     ChangeAdded,
				Category: "field",
				Name:     name,
				Detail:   fmt.Sprintf("+ field: %s (%s, label: %s)", name, tf.Type, tf.Label),
				NewValue: fmt.Sprintf("type=%s label=%s", tf.Type, tf.Label),
			})
		} else {
			diffs := fieldDiffs(ff, tf)
			for _, d := range diffs {
				changes = append(changes, FieldChange{
					Type:     ChangeModified,
					Category: "field",
					Name:     name,
					Detail:   fmt.Sprintf("~ field: %s — %s", name, d),
					OldValue: fieldAttr(&ff, d),
					NewValue: fieldAttr(&tf, d),
				})
			}
		}
	}

	// Detect removed
	for name, ff := range fromMap {
		if _, exists := toMap[name]; !exists {
			changes = append(changes, FieldChange{
				Type:     ChangeRemoved,
				Category: "field",
				Name:     name,
				Detail:   fmt.Sprintf("- field: %s (%s)", name, ff.Type),
				OldValue: fmt.Sprintf("type=%s label=%s", ff.Type, ff.Label),
			})
		}
	}

	return changes
}

// fieldDiffs returns a list of attribute names that differ between two fields.
func fieldDiffs(a, b mozi.FieldIR) []string {
	var diffs []string
	if a.Type != b.Type {
		diffs = append(diffs, "type")
	}
	if a.Label != b.Label {
		diffs = append(diffs, "label")
	}
	if a.Required != b.Required {
		diffs = append(diffs, "required")
	}
	if a.Unique != b.Unique {
		diffs = append(diffs, "unique")
	}
	if a.FormType != b.FormType {
		diffs = append(diffs, "form_type")
	}
	if a.Listable != b.Listable {
		diffs = append(diffs, "listable")
	}
	if a.Editable != b.Editable {
		diffs = append(diffs, "editable")
	}
	if a.Searchable != b.Searchable {
		diffs = append(diffs, "searchable")
	}
	if !equalStrings(a.EnumValues, b.EnumValues) {
		diffs = append(diffs, "enum_values")
	}
	if a.Default != nil && b.Default != nil && *a.Default != *b.Default {
		diffs = append(diffs, "default")
	}
	if (a.Default == nil) != (b.Default == nil) {
		diffs = append(diffs, "default")
	}
	return diffs
}

func fieldAttr(f *mozi.FieldIR, attr string) string {
	switch attr {
	case "type":
		return string(f.Type)
	case "label":
		return f.Label
	case "required":
		return fmt.Sprintf("%v", f.Required)
	case "unique":
		return fmt.Sprintf("%v", f.Unique)
	case "form_type":
		return f.FormType
	case "listable":
		return fmt.Sprintf("%v", f.Listable)
	case "editable":
		return fmt.Sprintf("%v", f.Editable)
	case "searchable":
		return fmt.Sprintf("%v", f.Searchable)
	case "enum_values":
		return strings.Join(f.EnumValues, ",")
	case "default":
		if f.Default != nil {
			return *f.Default
		}
		return "<none>"
	}
	return ""
}

// compareRelations detects added, removed, and modified relations.
func compareRelations(from, to []mozi.RelationIR) []FieldChange {
	fromMap := make(map[string]mozi.RelationIR)
	for _, r := range from {
		fromMap[r.Name] = r
	}
	toMap := make(map[string]mozi.RelationIR)
	for _, r := range to {
		toMap[r.Name] = r
	}

	var changes []FieldChange

	for name, tr := range toMap {
		fr, exists := fromMap[name]
		if !exists {
			changes = append(changes, FieldChange{
				Type:     ChangeAdded,
				Category: "relation",
				Name:     name,
				Detail:   fmt.Sprintf("+ relation: %s (%s → %s/%s)", name, tr.Type, tr.TargetModule, tr.TargetModel),
				NewValue: relationValue(tr),
			})
		} else if fr.Label != tr.Label || fr.Type != tr.Type || fr.Target != tr.Target {
			changes = append(changes, FieldChange{
				Type:     ChangeModified,
				Category: "relation",
				Name:     name,
				Detail:   fmt.Sprintf("~ relation: %s", name),
				OldValue: relationValue(fr),
				NewValue: relationValue(tr),
			})
		}
	}

	for name, fr := range fromMap {
		if _, exists := toMap[name]; !exists {
			changes = append(changes, FieldChange{
				Type:     ChangeRemoved,
				Category: "relation",
				Name:     name,
				Detail:   fmt.Sprintf("- relation: %s (%s → %s/%s)", name, fr.Type, fr.TargetModule, fr.TargetModel),
				OldValue: relationValue(fr),
			})
		}
	}

	return changes
}

func relationValue(r mozi.RelationIR) string {
	return fmt.Sprintf("label=%s type=%s target=%s", r.Label, r.Type, r.Target)
}

// compareAdmin detects changes to admin config.
func compareAdmin(from, to mozi.AdminConfig) []FieldChange {
	var changes []FieldChange

	if from.PageSize != to.PageSize {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: "admin", Name: "page_size",
			Detail:   fmt.Sprintf("~ page_size: %d → %d", from.PageSize, to.PageSize),
			OldValue: fmt.Sprintf("%d", from.PageSize), NewValue: fmt.Sprintf("%d", to.PageSize),
		})
	}
	if from.DefaultSort != to.DefaultSort {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: "admin", Name: "default_sort",
			Detail:   fmt.Sprintf("~ default_sort: %s → %s", from.DefaultSort, to.DefaultSort),
			OldValue: from.DefaultSort, NewValue: to.DefaultSort,
		})
	}
	if from.DefaultOrder != to.DefaultOrder {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: "admin", Name: "default_order",
			Detail:   fmt.Sprintf("~ default_order: %s → %s", from.DefaultOrder, to.DefaultOrder),
			OldValue: from.DefaultOrder, NewValue: to.DefaultOrder,
		})
	}
	if !equalStrings(from.ListColumns, to.ListColumns) {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: "admin", Name: "list_columns",
			Detail:   "~ list_columns changed",
			OldValue: strings.Join(from.ListColumns, ","),
			NewValue: strings.Join(to.ListColumns, ","),
		})
	}
	if !equalStrings(from.SearchFields, to.SearchFields) {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: "admin", Name: "search_fields",
			Detail:   "~ search_fields changed",
			OldValue: strings.Join(from.SearchFields, ","),
			NewValue: strings.Join(to.SearchFields, ","),
		})
	}

	return changes
}

// AffectedFiles returns the list of files that would be modified by the diff.
func (d *DiffResult) AffectedFiles() []AffectedFile {
	if !d.HasChanges {
		return nil
	}

	parts := strings.SplitN(d.ModelRef, "/", 2)
	moduleName := parts[0]
	modelName := parts[1]
	modelSnake := moziToSnake(modelName)

	var files []AffectedFile

	hasFieldChanges := false
	hasRelationChanges := false
	hasAdminChanges := false
	hasMetaChanges := false
	hasSemanticChanges := false
	hasUIIntentChanges := false
	hasAPIIntentChanges := false
	for _, c := range d.Changes {
		switch c.Category {
		case "field":
			hasFieldChanges = true
		case "relation":
			hasRelationChanges = true
		case "admin":
			hasAdminChanges = true
		case "meta":
			hasMetaChanges = true
		case "semantics":
			hasSemanticChanges = true
		case "ui_intent":
			hasUIIntentChanges = true
		case "api_intent":
			hasAPIIntentChanges = true
		}
	}

	if hasFieldChanges || hasRelationChanges {
		files = append(files,
			AffectedFile{
				Path:        fmt.Sprintf("ent/schema/%s_%s.go", moduleName, modelSnake),
				Description: "Schema fields/edges updated",
				ChangeCount: countByCategory(d.Changes, "field") + countByCategory(d.Changes, "relation"),
			},
			AffectedFile{
				Path:        fmt.Sprintf("internal/model/%s/%s.go", moduleName, modelSnake),
				Description: "Request/response struct fields updated",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
		)
	}

	if hasFieldChanges {
		files = append(files,
			AffectedFile{
				Path:        fmt.Sprintf("internal/handler/%s/%s.go", moduleName, modelSnake),
				Description: "Handler updated (if field list changed)",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
			AffectedFile{
				Path:        fmt.Sprintf("internal/service/%s/%s.go", moduleName, modelSnake),
				Description: "Service updated (if field list changed)",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
			AffectedFile{
				Path:        fmt.Sprintf("admin/src/api/%s.ts", moduleName),
				Description: "API types/functions updated",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
			AffectedFile{
				Path:        fmt.Sprintf("admin/src/pages/%s/%sList.tsx", moduleName, modelName),
				Description: "List page columns updated",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
			AffectedFile{
				Path:        fmt.Sprintf("admin/src/pages/%s/%sForm.tsx", moduleName, modelName),
				Description: "Form fields updated",
				ChangeCount: countByCategory(d.Changes, "field"),
			},
		)
	}

	if hasAdminChanges {
		files = append(files,
			AffectedFile{
				Path:        fmt.Sprintf("internal/handler/%s/%s.go", moduleName, modelSnake),
				Description: "Handler updated (admin config changed)",
				ChangeCount: countByCategory(d.Changes, "admin"),
			},
		)
	}
	if hasMetaChanges {
		files = append(files, AffectedFile{
			Path:        fmt.Sprintf("models/%s/%s.yaml", moduleName, modelSnake),
			Description: "Model metadata (label/description/table/display) updated",
			ChangeCount: countByCategory(d.Changes, "meta"),
		})
	}

	if hasSemanticChanges || hasUIIntentChanges || hasAPIIntentChanges {
		files = append(files, AffectedFile{
			Path:        fmt.Sprintf("models/%s/%s.yaml", moduleName, modelSnake),
			Description: "Model semantics/UI/API intent snapshot updated",
			ChangeCount: countByCategory(d.Changes, "semantics") + countByCategory(d.Changes, "ui_intent") + countByCategory(d.Changes, "api_intent"),
		})
		if hasUIIntentChanges {
			uiChangeCount := countByCategory(d.Changes, "ui_intent")
			if uiAffectsSurface(d.Changes, "admin") {
				files = append(files, AffectedFile{
					Path:        fmt.Sprintf("admin/src/pages/%s/%sList.tsx", moduleName, modelName),
					Description: "Admin UI may need to follow the updated product intent",
					ChangeCount: uiChangeCount,
				})
			}
			if uiAffectsSurface(d.Changes, "desktop") {
				files = append(files, AffectedFile{
					Path:        "../memflow-desktop/src/pages/",
					Description: "Desktop client UI may need to follow the updated product intent",
					ChangeCount: uiChangeCount,
				})
			}
			if uiAffectsSurface(d.Changes, "miniapp") {
				files = append(files, AffectedFile{
					Path:        "../memflow-miniapp/src/pages/",
					Description: "Mini program UI may need to follow the updated product intent",
					ChangeCount: uiChangeCount,
				})
			}
		}
		if hasSemanticChanges {
			files = append(files, AffectedFile{
				Path:        fmt.Sprintf("internal/service/%s/%s.go", moduleName, modelSnake),
				Description: "Business behavior may need to follow updated semantics",
				ChangeCount: countByCategory(d.Changes, "semantics"),
			})
		}
		if hasAPIIntentChanges {
			apiChangeCount := countByCategory(d.Changes, "api_intent")
			files = append(files,
				AffectedFile{
					Path:        fmt.Sprintf("internal/handler/%s/%s.go", moduleName, modelSnake),
					Description: "HTTP API behavior may need to follow the updated API contract",
					ChangeCount: apiChangeCount,
				},
				AffectedFile{
					Path:        fmt.Sprintf("internal/service/%s/%s.go", moduleName, modelSnake),
					Description: "Service behavior may need to support the updated API contract",
					ChangeCount: apiChangeCount,
				},
				AffectedFile{
					Path:        "docs/swagger.yaml",
					Description: "OpenAPI documentation may need to reflect API contract changes",
					ChangeCount: apiChangeCount,
				},
				AffectedFile{
					Path:        "docs/swagger.json",
					Description: "Generated OpenAPI JSON may need to reflect API contract changes",
					ChangeCount: apiChangeCount,
				},
			)
		}
	}

	return files
}

func countByCategory(changes []FieldChange, category string) int {
	n := 0
	for _, c := range changes {
		if c.Category == category {
			n++
		}
	}
	return n
}

func moziToSnake(s string) string {
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

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func semanticSummary(s mozi.SemanticConfig) string {
	var parts []string
	if s.Purpose != "" {
		parts = append(parts, "purpose="+s.Purpose)
	}
	if s.UserValue != "" {
		parts = append(parts, "user_value="+s.UserValue)
	}
	if len(s.BusinessRules) > 0 {
		parts = append(parts, fmt.Sprintf("%d business_rules", len(s.BusinessRules)))
	}
	if len(s.Permissions) > 0 {
		parts = append(parts, fmt.Sprintf("%d permissions", len(s.Permissions)))
	}
	if len(s.Lifecycle) > 0 {
		parts = append(parts, fmt.Sprintf("%d lifecycle notes", len(s.Lifecycle)))
	}
	return strings.Join(parts, "; ")
}

func uiIntentSummary(s mozi.UIIntentConfig) string {
	parts := []string{}
	if shared := uiSharedSummary(s); shared != "" {
		parts = append(parts, shared)
	}
	if legacy := uiLegacyIntentSummary(s); legacy != "" {
		parts = append(parts, legacy)
	}
	if len(s.SurfacesConfig) > 0 {
		parts = append(parts, fmt.Sprintf("%d surface configs", len(s.SurfacesConfig)))
	}
	return strings.Join(parts, "; ")
}

func uiSharedSummary(s mozi.UIIntentConfig) string {
	var parts []string
	if s.ProductGoal != "" {
		parts = append(parts, "product_goal="+s.ProductGoal)
	}
	if len(s.UserTasks) > 0 {
		parts = append(parts, fmt.Sprintf("%d user_tasks", len(s.UserTasks)))
	}
	if len(s.Shared.PrimaryEntities) > 0 {
		parts = append(parts, fmt.Sprintf("%d primary_entities", len(s.Shared.PrimaryEntities)))
	}
	if len(s.Shared.PrimaryActions) > 0 {
		parts = append(parts, fmt.Sprintf("%d shared_actions", len(s.Shared.PrimaryActions)))
	}
	if s.Shared.EmptyState != "" {
		parts = append(parts, "shared_empty_state="+s.Shared.EmptyState)
	}
	if len(s.Shared.Terminology) > 0 {
		parts = append(parts, fmt.Sprintf("%d terminology entries", len(s.Shared.Terminology)))
	}
	return strings.Join(parts, "; ")
}

func uiLegacyIntentSummary(s mozi.UIIntentConfig) string {
	var parts []string
	if s.PrimaryView != "" {
		parts = append(parts, "primary_view="+s.PrimaryView)
	}
	if len(s.Surfaces) > 0 {
		parts = append(parts, fmt.Sprintf("%d surfaces", len(s.Surfaces)))
	}
	if s.ListIntent != "" {
		parts = append(parts, "list_intent="+s.ListIntent)
	}
	if s.FormIntent != "" {
		parts = append(parts, "form_intent="+s.FormIntent)
	}
	if s.DetailIntent != "" {
		parts = append(parts, "detail_intent="+s.DetailIntent)
	}
	if len(s.PrimaryActions) > 0 {
		parts = append(parts, fmt.Sprintf("%d primary_actions", len(s.PrimaryActions)))
	}
	if len(s.InteractionNotes) > 0 {
		parts = append(parts, fmt.Sprintf("%d interaction_notes", len(s.InteractionNotes)))
	}
	if len(s.SurfaceNotes) > 0 {
		parts = append(parts, fmt.Sprintf("%d surface_notes", len(s.SurfaceNotes)))
	}
	return strings.Join(parts, "; ")
}

func uiSurfaceSummary(s mozi.UISurfaceIntentConfig) string {
	var parts []string
	if s.Role != "" {
		parts = append(parts, "role="+s.Role)
	}
	if len(s.EnabledTasks) > 0 {
		parts = append(parts, fmt.Sprintf("%d enabled_tasks", len(s.EnabledTasks)))
	}
	if len(s.Views) > 0 {
		parts = append(parts, fmt.Sprintf("%d views", len(s.Views)))
	}
	if len(s.Actions) > 0 {
		parts = append(parts, fmt.Sprintf("%d actions", len(s.Actions)))
	}
	if len(s.Constraints) > 0 {
		parts = append(parts, fmt.Sprintf("%d constraints", len(s.Constraints)))
	}
	return strings.Join(parts, "; ")
}

func changedUISurfaces(from, to map[string]mozi.UISurfaceIntentConfig) []string {
	seen := make(map[string]bool)
	for surface := range from {
		seen[surface] = true
	}
	for surface := range to {
		seen[surface] = true
	}

	var changed []string
	for surface := range seen {
		if !reflect.DeepEqual(from[surface], to[surface]) {
			changed = append(changed, surface)
		}
	}
	sort.Strings(changed)
	return changed
}

func legacyUIIntentChanged(from, to mozi.UIIntentConfig) bool {
	return !equalStrings(from.Surfaces, to.Surfaces) ||
		from.PrimaryView != to.PrimaryView ||
		!equalStrings(from.PrimaryActions, to.PrimaryActions) ||
		from.ListIntent != to.ListIntent ||
		from.FormIntent != to.FormIntent ||
		from.DetailIntent != to.DetailIntent ||
		from.EmptyState != to.EmptyState ||
		!equalStrings(from.InteractionNotes, to.InteractionNotes) ||
		!equalStrings(from.SurfaceNotes, to.SurfaceNotes)
}

func uiAffectsSurface(changes []FieldChange, surface string) bool {
	for _, change := range changes {
		if change.Category != "ui_intent" {
			continue
		}
		if change.Name == "shared" || change.Name == "legacy" || change.Name == "ui_intent" || change.Name == "surfaces."+surface {
			return true
		}
	}
	return false
}

func apiIntentSummary(s mozi.APIIntentConfig) string {
	var parts []string
	if s.Exposure != "" {
		parts = append(parts, "exposure="+s.Exposure)
	}
	if s.Auth != "" {
		parts = append(parts, "auth="+s.Auth)
	}
	if s.BasePath != "" {
		parts = append(parts, "base_path="+s.BasePath)
	}
	if len(s.Consumers) > 0 {
		parts = append(parts, fmt.Sprintf("%d consumers", len(s.Consumers)))
	}
	if len(s.Operations) > 0 {
		parts = append(parts, fmt.Sprintf("%d operations", len(s.Operations)))
	}
	if len(s.ErrorCases) > 0 {
		parts = append(parts, fmt.Sprintf("%d error_cases", len(s.ErrorCases)))
	}
	if s.Versioning != "" {
		parts = append(parts, "versioning="+s.Versioning)
	}
	return strings.Join(parts, "; ")
}
