package parser

import (
	"fmt"

	"github.com/pangu-sutido/mozi-builder/mozi"
)

// ValidationError represents a single validation issue.
type ValidationError struct {
	Model   string `json:"model"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s.%s] %s", e.Model, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Model, e.Message)
}

// ValidationResult holds the result of validating one or more models.
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors,omitempty"`
	Warnings []*ValidationError `json:"warnings,omitempty"`
}

// Validate checks a single ModelIR for semantic correctness.
func Validate(model *mozi.ModelIR) *ValidationResult {
	result := &ValidationResult{Valid: true}
	add := func(field, msg string) {
		result.Errors = append(result.Errors, &ValidationError{
			Model: model.Module + "/" + model.Name, Field: field, Message: msg,
		})
		result.Valid = false
	}
	warn := func(field, msg string) {
		result.Warnings = append(result.Warnings, &ValidationError{
			Model: model.Module + "/" + model.Name, Field: field, Message: msg,
		})
	}

	// Model-level checks
	if model.Name == "" {
		add("", "model name is required")
	}
	if model.Module == "" {
		add("", "module is required")
	}

	// Field checks
	seenFields := make(map[string]bool)
	hasPrimary := false
	for _, f := range model.Fields {
		if f.Name == "" {
			add("", "field name is required")
			continue
		}
		if seenFields[f.Name] {
			add(f.Name, "duplicate field name")
		}
		seenFields[f.Name] = true

		if f.Primary {
			hasPrimary = true
		}

		if !isValidFieldType(f.Type) {
			add(f.Name, fmt.Sprintf("invalid field type: %s", f.Type))
		}

		if f.Type == mozi.FieldTypeEnum && len(f.EnumValues) == 0 {
			add(f.Name, "enum type requires enum_values")
		}

		if f.Generated != "" && !isValidGeneratedType(f.Generated) {
			add(f.Name, fmt.Sprintf("invalid generated type: %s (must be uuid, auto, or manual)", f.Generated))
		}

		if f.FormType != "" && !isValidFormType(f.FormType) {
			warn(f.Name, fmt.Sprintf("unknown form_type: %s", f.FormType))
		}
	}

	if !hasPrimary {
		add("", "model must have a primary key field")
	}

	// Relation checks
	seenRelations := make(map[string]bool)
	for _, r := range model.Relations {
		if r.Name == "" {
			add("", "relation name is required")
			continue
		}
		if seenRelations[r.Name] {
			add(r.Name, "duplicate relation name")
		}
		seenRelations[r.Name] = true

		if r.Target == "" {
			add(r.Name, "relation target is required")
		}
		if r.Label == "" {
			add(r.Name, "relation label is required; provide a business predicate such as 包含、归属于、创建、产生、记录 or 表示")
		}
		if !isValidRelationType(r.Type) {
			add(r.Name, fmt.Sprintf("invalid relation type: %s", r.Type))
		}
	}

	return result
}

// ValidateProject validates an entire ProjectIR including all modules and models.
func ValidateProject(project *mozi.ProjectIR) *ValidationResult {
	result := &ValidationResult{Valid: true}

	for _, mod := range project.Modules {
		// Validate module
		if mod.Name == "" {
			result.Errors = append(result.Errors, &ValidationError{
				Model: "project", Message: "module name is required",
			})
			result.Valid = false
		}

		// Validate all models in module
		for _, m := range mod.Models {
			mr := Validate(m)
			result.Errors = append(result.Errors, mr.Errors...)
			result.Warnings = append(result.Warnings, mr.Warnings...)
			if !mr.Valid {
				result.Valid = false
			}
		}

		// Cross-model relation checks within module
		modelNames := make(map[string]bool)
		for _, m := range mod.Models {
			modelNames[m.Name] = true
		}
		for _, m := range mod.Models {
			for _, r := range m.Relations {
				if r.TargetModel != "" && !modelNames[r.TargetModel] {
					// Check if target is in another module
					found := false
					for _, otherMod := range project.Modules {
						if otherMod.Name == r.TargetModule {
							for _, om := range otherMod.Models {
								if om.Name == r.TargetModel {
									found = true
									break
								}
							}
						}
					}
					if !found {
						result.Warnings = append(result.Warnings, &ValidationError{
							Model: m.Module + "/" + m.Name,
							Field: r.Name,
							Message: fmt.Sprintf("relation target '%s' (%s/%s) not found",
								r.Target, r.TargetModule, r.TargetModel),
						})
					}
				}
			}
		}
	}

	return result
}

// ValidateAll validates all models, including cross-model reference checks.
// (deprecated: use ValidateProject for module-aware validation)
func ValidateAll(models []*mozi.ModelIR) *ValidationResult {
	result := &ValidationResult{Valid: true}
	modelKeys := make(map[string]bool)

	for _, m := range models {
		key := m.Module + "/" + m.Name
		modelKeys[key] = true
		modelKeys[m.Name] = true
		mr := Validate(m)
		result.Errors = append(result.Errors, mr.Errors...)
		result.Warnings = append(result.Warnings, mr.Warnings...)
		if !mr.Valid {
			result.Valid = false
		}
	}

	// Cross-model relation checks
	for _, m := range models {
		for _, r := range m.Relations {
			targetKey := r.TargetModule + "/" + r.TargetModel
			if !modelKeys[targetKey] && !modelKeys[r.TargetModel] {
				result.Warnings = append(result.Warnings, &ValidationError{
					Model:   m.Module + "/" + m.Name,
					Field:   r.Name,
					Message: fmt.Sprintf("relation target '%s' not found in any module", r.Target),
				})
			}
		}
	}

	return result
}

func isValidFieldType(ft mozi.FieldType) bool {
	switch ft {
	case mozi.FieldTypeString, mozi.FieldTypeInt, mozi.FieldTypeFloat,
		mozi.FieldTypeBool, mozi.FieldTypeTime, mozi.FieldTypeText,
		mozi.FieldTypeEnum, mozi.FieldTypeJSON:
		return true
	}
	return false
}

func isValidGeneratedType(gt mozi.GeneratedType) bool {
	switch gt {
	case mozi.GeneratedUUID, mozi.GeneratedAuto, mozi.GeneratedManual:
		return true
	}
	return false
}

func isValidRelationType(rt mozi.RelationType) bool {
	switch rt {
	case mozi.RelationHasOne, mozi.RelationHasMany,
		mozi.RelationBelongsTo, mozi.RelationManyToMany:
		return true
	}
	return false
}

func isValidFormType(ft string) bool {
	switch ft {
	case "text", "email", "password", "number", "select",
		"switch", "date", "textarea", "richtext", "upload":
		return true
	}
	return false
}
