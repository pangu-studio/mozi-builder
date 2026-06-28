package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warning"
	LintInfo    LintSeverity = "info"
)

type LintIssue struct {
	Code     string       `json:"code"`
	Severity LintSeverity `json:"severity"`
	Model    string       `json:"model"`
	Field    string       `json:"field,omitempty"`
	Message  string       `json:"message"`
}

type LintResult struct {
	Valid  bool        `json:"valid"`
	Issues []LintIssue `json:"issues"`
}

type LintOptions struct{ Strict bool }

// LintProject performs project-wide design checks with stable rule codes.
func LintProject(project *mozi.ProjectIR, opts LintOptions) *LintResult {
	result := &LintResult{Valid: true, Issues: []LintIssue{}}
	add := func(code string, severity LintSeverity, model, field, message string) {
		if opts.Strict && severity == LintWarning {
			severity = LintError
		}
		result.Issues = append(result.Issues, LintIssue{code, severity, model, field, message})
		if severity == LintError {
			result.Valid = false
		}
	}
	if project.SchemaVersion != mozi.CurrentSchemaVersion {
		add("unsupported-schema-version", LintError, "project", "schema_version", fmt.Sprintf("schema version %d is not supported; current version is %d", project.SchemaVersion, mozi.CurrentSchemaVersion))
	}
	errorCodes := map[string]bool{}
	validCategories := map[string]bool{"resource": true, "validation": true, "permission": true, "business": true, "system": true, "rate_limit": true, "auth": true}
	for _, item := range project.ErrorCodes {
		if item.Code == "" {
			add("invalid-error-code", LintError, "project", "error_codes", "error code is required")
			continue
		}
		if errorCodes[item.Code] {
			add("duplicate-error-code", LintError, "project", item.Code, "error code is duplicated")
		}
		errorCodes[item.Code] = true
		if item.HTTPStatus < 400 || item.HTTPStatus > 599 {
			add("invalid-error-status", LintError, "project", item.Code, "http_status must be between 400 and 599")
		}
		if !validCategories[item.Category] {
			add("invalid-error-category", LintError, "project", item.Code, "unknown error category")
		}
		if item.ConsumerFacing && strings.TrimSpace(item.Message) == "" {
			add("missing-error-message", LintError, "project", item.Code, "consumer-facing error requires a message")
		}
	}
	modelRefs := map[string]*mozi.ModelIR{}
	modules := map[string]bool{}
	for _, mod := range project.Modules {
		if modules[mod.Name] {
			add("duplicate-module", LintError, "project", mod.Name, "module name is duplicated")
		}
		modules[mod.Name] = true
		for _, model := range mod.Models {
			ref := mod.Name + "/" + model.Name
			if _, ok := modelRefs[ref]; ok {
				add("duplicate-model", LintError, ref, "", "model identifier is duplicated")
			}
			modelRefs[ref] = model
		}
	}
	for ref, model := range modelRefs {
		if model.SchemaVersion != mozi.CurrentSchemaVersion {
			add("unsupported-schema-version", LintError, ref, "schema_version", fmt.Sprintf("schema version %d is not supported", model.SchemaVersion))
		}
		if strings.TrimSpace(model.Description) == "" {
			add("missing-description", LintWarning, ref, "", "model description is missing")
		}
		if strings.TrimSpace(model.Semantics.Purpose) == "" {
			add("no-semantics", LintWarning, ref, "semantics", "model purpose is missing")
		}
		hasCreated, hasUpdated := false, false
		ids := map[string]bool{}
		i18nKeys := map[string]bool{}
		for _, field := range model.Fields {
			if strings.TrimSpace(field.Label) == "" {
				add("missing-label", LintWarning, ref, field.Name, "field label is missing")
			}
			if field.ID != "" && ids[field.ID] {
				add("duplicate-stable-id", LintError, ref, field.Name, "field stable id is duplicated")
			}
			if field.ID != "" {
				ids[field.ID] = true
			}
			if field.I18nKey != "" && i18nKeys[field.I18nKey] {
				add("duplicate-i18n-key", LintError, ref, field.Name, "i18n key is duplicated in model")
			}
			if field.I18nKey != "" {
				i18nKeys[field.I18nKey] = true
			}
			hasCreated = hasCreated || field.Name == "created_at"
			hasUpdated = hasUpdated || field.Name == "updated_at"
		}
		if !hasCreated || !hasUpdated {
			add("missing-timestamps", LintInfo, ref, "", "model does not define both created_at and updated_at")
		}
		for _, code := range model.APIIntent.ErrorCodes {
			if !errorCodes[code] {
				add("unknown-error-code", LintError, ref, "api_intent.error_codes", fmt.Sprintf("error code %q is not registered", code))
			}
		}
		for i, rule := range model.Semantics.PermissionRules {
			field := fmt.Sprintf("semantics.permission_rules[%d]", i)
			if rule.Principal == "" || rule.Resource == "" || rule.Action == "" {
				add("invalid-permission-rule", LintError, ref, field, "principal, resource, and action are required")
			}
			if rule.Effect != "" && rule.Effect != "allow" && rule.Effect != "deny" {
				add("invalid-permission-effect", LintError, ref, field, "effect must be allow or deny")
			}
			if rule.Scope != "" && !map[string]bool{"own": true, "group": true, "tenant": true, "all": true}[rule.Scope] {
				add("invalid-permission-scope", LintError, ref, field, "scope must be own, group, tenant, or all")
			}
			if rule.Scope == "own" && rule.OwnerField == "" {
				add("missing-owner-field", LintError, ref, field, "own scope requires owner_field")
			}
			if rule.Scope == "tenant" && rule.TenantField == "" {
				add("missing-tenant-field", LintError, ref, field, "tenant scope requires tenant_field")
			}
		}
		contractNames := map[string]bool{}
		for i, contract := range model.APIIntent.TestContracts {
			field := fmt.Sprintf("api_intent.test_contracts[%d]", i)
			if contract.Name == "" || contract.OperationID == "" {
				add("invalid-test-contract", LintError, ref, field, "name and operation_id are required")
			}
			if contractNames[contract.Name] {
				add("duplicate-test-contract", LintError, ref, field, "test contract name is duplicated")
			}
			contractNames[contract.Name] = true
			if contract.Expect.Status < 100 || contract.Expect.Status > 599 {
				add("invalid-test-status", LintError, ref, field, "expected status must be between 100 and 599")
			}
			if contract.Expect.ErrorCode != "" && !errorCodes[contract.Expect.ErrorCode] {
				add("unknown-error-code", LintError, ref, field, fmt.Sprintf("error code %q is not registered", contract.Expect.ErrorCode))
			}
		}
		for _, relation := range model.Relations {
			target := relation.TargetModule + "/" + relation.TargetModel
			targetModel, ok := modelRefs[target]
			if !ok {
				add("orphan-relation", LintError, ref, relation.Name, fmt.Sprintf("relation target %q does not exist", target))
				continue
			}
			if relation.BackRef != "" && !hasRelationNamed(targetModel, relation.BackRef) {
				add("invalid-back-ref", LintError, ref, relation.Name, fmt.Sprintf("target %s has no relation named %q", target, relation.BackRef))
			}
		}
	}
	sort.Slice(result.Issues, func(i, j int) bool {
		a, b := result.Issues[i], result.Issues[j]
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Field < b.Field
	})
	return result
}

func hasRelationNamed(model *mozi.ModelIR, name string) bool {
	for _, relation := range model.Relations {
		if relation.Name == name {
			return true
		}
	}
	return false
}
