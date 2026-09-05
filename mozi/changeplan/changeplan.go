// Package changeplan assembles AI Coding change-plan contracts from model IRs
// and structured diffs. It is pure logic: data loading (design DB, manifest)
// stays with the caller — devplatform (v1) or the v2 platform design store.
// The logic was extracted verbatim from devplatform/service.go.
package changeplan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
	"github.com/pangu-studio/mozi-builder/mozi/migration"
)

// Status indicates whether a change plan has been applied to code.
type Status string

const (
	Pending Status = "pending" // model changed, code not yet synced
	Applied Status = "applied" // model changes already synced to code
	NoDiff  Status = "no_diff" // model has no version diff
)

// Result describes a model change as an AI Coding task instead of a
// template overwrite operation.
type Result struct {
	ModelRef         string                `json:"model_ref"`
	Status           Status                `json:"status"`
	Intent           string                `json:"intent"`
	ModuleIcon       string                `json:"module_icon,omitempty"`
	ModelIcon        string                `json:"model_icon,omitempty"`
	Semantics        mozi.SemanticConfig   `json:"semantics"`
	UIIntent         mozi.UIIntentConfig   `json:"ui_intent"`
	APIIntent        mozi.APIIntentConfig  `json:"api_intent"`
	Diff             *differ.DiffResult    `json:"diff"`
	AffectedFiles    []differ.AffectedFile `json:"affected_files"`
	Contracts        []string              `json:"contracts"`
	Tasks            []Task                `json:"tasks"`
	Checks           []string              `json:"checks"`
	Migration        migration.Plan        `json:"migration"`
	RequiresApproval bool                  `json:"requires_approval"`
	Prompt           string                `json:"prompt"`
}

// Task is one actionable item in the AI Coding plan.
type Task struct {
	Area        string   `json:"area"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// Input carries everything needed to assemble a change plan. Previous may be
// nil when no earlier version exists; migration advice then treats the model
// as newly created.
type Input struct {
	Model         *mozi.ModelIR
	Previous      *mozi.ModelIR
	Diff          *differ.DiffResult
	AffectedFiles []differ.AffectedFile
	Status        Status
	ModuleIcon    string
}

// Build assembles the full change-plan contract.
func Build(in Input) *Result {
	model := in.Model
	previous := in.Previous
	if previous == nil {
		previous = &mozi.ModelIR{Module: model.Module, Name: model.Name, Table: model.Table}
	}
	result := &Result{
		ModelRef:      model.Module + "/" + model.Name,
		Status:        in.Status,
		Intent:        buildIntent(in.Diff, in.Status),
		ModuleIcon:    strings.TrimSpace(in.ModuleIcon),
		ModelIcon:     strings.TrimSpace(model.Display.Icon),
		Semantics:     model.Semantics,
		UIIntent:      model.UIIntent,
		APIIntent:     model.APIIntent,
		Diff:          in.Diff,
		AffectedFiles: in.AffectedFiles,
		Contracts:     buildContracts(in.Status),
		Tasks:         buildTasks(model, in.Diff, in.AffectedFiles, in.Status),
		Checks:        buildChecks(in.Diff, in.Status),
		Migration:     migration.Advise(previous, model, in.Diff),
	}
	result.RequiresApproval = result.Migration.HasDangerous || hasBreakingChange(in.Diff)
	result.Prompt = buildPrompt(result)
	return result
}

func hasBreakingChange(diff *differ.DiffResult) bool {
	if diff == nil {
		return false
	}
	for _, change := range diff.Changes {
		if change.Compatibility == differ.CompatibilityBreaking {
			return true
		}
	}
	return false
}

func buildIntent(diff *differ.DiffResult, status Status) string {
	switch status {
	case NoDiff:
		return "No model changes detected. Keep code unchanged unless manual cleanup is explicitly requested."
	case Applied:
		return fmt.Sprintf("Model %s v%s changes have already been applied to the repository. No further code changes needed.", diff.ModelRef, diff.ToVersion)
	case Pending:
		// fallthrough to diff-based intent
	}

	if diff == nil || !diff.HasChanges {
		return "No model changes detected. Keep code unchanged unless manual cleanup is explicitly requested."
	}

	var parts []string
	added := countChanges(diff.Changes, differ.ChangeAdded)
	modified := countChanges(diff.Changes, differ.ChangeModified)
	removed := countChanges(diff.Changes, differ.ChangeRemoved)
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d additions", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modifications", modified))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removals", removed))
	}
	return fmt.Sprintf("Apply %s from %s %s to %s as an incremental code patch.", strings.Join(parts, ", "), diff.ModelRef, diff.FromVersion, diff.ToVersion)
}

func buildContracts(status Status) []string {
	if status == Applied || status == NoDiff {
		return []string{"No pending model changes — skip code generation."}
	}
	return []string{
		"Treat the design DB model as the source of truth; YAML snapshots are exchange artifacts.",
		"Generate a minimal patch against the current repository instead of overwriting handwritten files.",
		"Preserve existing custom business logic, UI behavior, comments, imports, and local formatting.",
		"Use existing module layout, API helpers, stores, route style, and component patterns.",
		"Treat API intent as a first-class contract for routes, DTOs, auth, errors, docs, and compatibility.",
		"Every new or modified HTTP endpoint must carry swaggo annotations; @Tags starts with the owning module label (e.g. 计费) and may append the model label (e.g. 会员方案); @Router must match the actual registered path (admin CRUD: {admin_api_base}/{module_api_prefix}/{plural}); regenerate docs (swag init) afterwards so the dev platform API workbench stays in sync.",
		"Keep deterministic generated artifacts small; put business behavior in normal application code.",
		"Do not modify unrelated modules or files unless the model relationship requires it.",
	}
}

func buildTasks(model *mozi.ModelIR, diff *differ.DiffResult, affectedFiles []differ.AffectedFile, status Status) []Task {
	if status == Applied {
		return []Task{
			{
				Area:        "review",
				Description: fmt.Sprintf("Model %s v%s already synced. Run mozi sync if the manifest is stale, or skip.", diff.ModelRef, diff.ToVersion),
			},
		}
	}

	if diff == nil || !diff.HasChanges {
		return []Task{
			{
				Area:        "review",
				Description: "Confirm the model has no pending changes and avoid unnecessary code churn.",
			},
		}
	}

	var tasks []Task
	if hasCategory(diff, "field") || hasCategory(diff, "relation") {
		tasks = append(tasks, Task{
			Area:        "backend",
			Description: "Update ent schema, request/response DTOs, handler binding, and service persistence behavior to match the model changes.",
			Files:       filesWithPrefix(affectedFiles, "ent/schema/", "internal/model/", "internal/handler/", "internal/service/"),
		})
	}
	if hasCategory(diff, "field") || hasCategory(diff, "admin") {
		tasks = append(tasks, Task{
			Area:        "frontend",
			Description: "Update API types, store usage, list columns, search filters, and form fields using existing admin UI patterns.",
			Files:       filesWithPrefix(affectedFiles, "admin/src/api/", "admin/src/pages/", "admin/src/stores/"),
		})
	}
	if hasCategory(diff, "relation") {
		tasks = append(tasks, Task{
			Area:        "relationship",
			Description: "Check relation target modules and update selectors, joins, eager loading, and display labels only where the current code needs them.",
		})
	}
	if hasCategory(diff, "admin") {
		tasks = append(tasks, Task{
			Area:        "admin-config",
			Description: "Apply list/search/sort/page-size behavior without rewriting unrelated page logic.",
		})
	}
	if hasCategory(diff, "meta") || hasCategory(diff, "ui_intent") || model.Display.Icon != "" {
		tasks = append(tasks, Task{
			Area:        "navigation",
			Description: fmt.Sprintf("When creating or updating menus, routes, breadcrumbs, cards, or other model entry points, use the model icon %q and the module icon configured in the design DB.", model.Display.Icon),
			Files:       filesWithPrefix(affectedFiles, "admin/src/", "../memflow-desktop/src/", "../memflow-miniapp/src/"),
		})
	}
	if hasCategory(diff, "semantics") {
		tasks = append(tasks, Task{
			Area:        "semantics",
			Description: "Translate business purpose, user value, permissions, lifecycle, and business rules into service behavior and tests where needed.",
			Files:       filesWithPrefix(affectedFiles, "internal/model/", "internal/handler/", "internal/service/"),
		})
		if len(model.Semantics.PermissionRules) > 0 {
			tasks = append(tasks, Task{Area: "permissions", Description: "Generate permission constants and wire server-side enforcement; client checks remain presentation-only.", Files: []string{"internal/permissions/generated.go"}})
		}
	}
	if hasCategory(diff, "ui_intent") {
		tasks = append(tasks, Task{
			Area:        "ui-intent",
			Description: "Update all configured UI surfaces as applicable (admin, desktop, miniapp, cli, and any custom surfaces registered in the ui_surfaces dictionary), including list, form, detail, empty state, primary actions, navigation, and surface-specific behavior.",
			Files:       filesWithPrefix(affectedFiles, "admin/src/", "../memflow-desktop/", "../memflow-miniapp/src/"),
		})
	}
	if hasCategory(diff, "api_intent") {
		tasks = append(tasks, Task{
			Area:        "api-contract",
			Description: "Update public/internal API routes, DTOs, auth behavior, validation, error responses, idempotency, versioning, and OpenAPI docs to match the API intent.",
			Files:       filesWithPrefix(affectedFiles, "internal/handler/", "internal/service/", "internal/model/", "docs/"),
		})
		if len(model.APIIntent.TestContracts) > 0 {
			tasks = append(tasks, Task{Area: "contract-tests", Description: "Generate and run framework-independent HTTP contract cases.", Files: []string{"contracts/bruno/"}})
		}
		tasks = append(tasks, Task{Area: "sdk", Description: "Regenerate the TypeScript SDK from the reviewed OpenAPI document.", Files: []string{"sdk/typescript/client.ts"}})
	}
	if modelHasI18n(model) {
		tasks = append(tasks, Task{Area: "i18n", Description: "Export the source catalog and validate missing, stale, and placeholder-mismatched translations.", Files: []string{"locales/source.json"}})
	}

	tasks = append(tasks, Task{
		Area:        "snapshot",
		Description: fmt.Sprintf("After verification, export the %s model snapshot so models/ stays aligned with the design DB, then run mozi sync.", model.Module),
		Files:       []string{fmt.Sprintf("models/%s/%s.yaml", model.Module, toSnake(model.Name))},
	})
	return tasks
}

func modelHasI18n(model *mozi.ModelIR) bool {
	for _, field := range model.Fields {
		if field.I18nKey != "" {
			return true
		}
	}
	return false
}

func buildChecks(diff *differ.DiffResult, status Status) []string {
	checks := []string{
		"mozi validate",
		"mozi lint --strict",
	}
	if status == Applied {
		checks = append(checks,
			fmt.Sprintf("mozi diff --model %s", diff.ModelRef),
			fmt.Sprintf("mozi sync --model %s  # if manifest is stale, re-sync", diff.ModelRef),
			"# No code changes needed - model already synced",
		)
		return checks
	}
	if diff != nil && diff.HasChanges {
		checks = append(checks,
			fmt.Sprintf("mozi diff --model %s", diff.ModelRef),
			"make generate",
			"cd admin && npx tsc --noEmit",
			"GOCACHE=/private/tmp/memflow-go-build-cache go test ./...",
			fmt.Sprintf("mozi export --module %s", strings.SplitN(diff.ModelRef, "/", 2)[0]),
			fmt.Sprintf("mozi sync --model %s", diff.ModelRef),
		)
	}
	return checks
}

func buildPrompt(plan *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Change plan for %s [status: %s]\n\n", plan.ModelRef, plan.Status)
	fmt.Fprintf(&b, "Intent: %s\n\n", plan.Intent)
	writeIconPromptSection(&b, plan)
	writeSemanticPromptSection(&b, plan)
	if plan.Diff != nil && len(plan.Diff.Changes) > 0 {
		b.WriteString("Model changes:\n")
		for _, change := range plan.Diff.Changes {
			fmt.Fprintf(&b, "- [%s] %s\n", change.Compatibility, change.Detail)
		}
		b.WriteString("\n")
	}
	if len(plan.AffectedFiles) > 0 {
		b.WriteString("Likely affected files:\n")
		for _, file := range plan.AffectedFiles {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", file.Evidence, file.Path, file.Description)
		}
		b.WriteString("\n")
	}
	if len(plan.Migration.Steps) > 0 {
		b.WriteString("Database migration advice (review only; never execute automatically):\n")
		for _, step := range plan.Migration.Steps {
			fmt.Fprintf(&b, "- [%s] %s", step.Risk, step.Description)
			if step.SQL != "" {
				fmt.Fprintf(&b, " — `%s`", step.SQL)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Contracts:\n")
	for _, contract := range plan.Contracts {
		fmt.Fprintf(&b, "- %s\n", contract)
	}
	b.WriteString("\nTasks:\n")
	for _, task := range plan.Tasks {
		fmt.Fprintf(&b, "- [%s] %s\n", task.Area, task.Description)
	}
	b.WriteString("\nVerification:\n")
	for _, check := range plan.Checks {
		fmt.Fprintf(&b, "- %s\n", check)
	}
	return b.String()
}

func writeIconPromptSection(b *strings.Builder, plan *Result) {
	if plan.ModuleIcon == "" && plan.ModelIcon == "" {
		return
	}
	b.WriteString("Icon contract:\n")
	if plan.ModuleIcon != "" {
		fmt.Fprintf(b, "- Module icon: %s\n", plan.ModuleIcon)
	}
	if plan.ModelIcon != "" {
		fmt.Fprintf(b, "- Model icon: %s\n", plan.ModelIcon)
	}
	b.WriteString("- Use these icon names when generating or updating menus, navigation entries, dashboards, cards, and other model-specific UI entry points.\n\n")
}

func writeSemanticPromptSection(b *strings.Builder, plan *Result) {
	b.WriteString("Product semantics, UI intent, and API intent are part of the model contract. Preserve existing code, but let these fields guide business behavior, page structure, copy, empty states, API behavior, documentation, and tests.\n")
	b.WriteString("\nSemantics:\n")
	if plan.Semantics.Purpose != "" {
		fmt.Fprintf(b, "- Purpose: %s\n", plan.Semantics.Purpose)
	}
	if len(plan.Semantics.Audience) > 0 {
		fmt.Fprintf(b, "- Audience: %s\n", strings.Join(plan.Semantics.Audience, ", "))
	}
	if plan.Semantics.UserValue != "" {
		fmt.Fprintf(b, "- User value: %s\n", plan.Semantics.UserValue)
	}
	for _, rule := range plan.Semantics.BusinessRules {
		fmt.Fprintf(b, "- Business rule: %s\n", rule)
	}
	for _, permission := range plan.Semantics.Permissions {
		fmt.Fprintf(b, "- Permission: %s\n", permission)
	}
	for _, lifecycle := range plan.Semantics.Lifecycle {
		fmt.Fprintf(b, "- Lifecycle: %s\n", lifecycle)
	}
	b.WriteString("\nUI intent:\n")
	if plan.UIIntent.ProductGoal != "" {
		fmt.Fprintf(b, "- Product goal: %s\n", plan.UIIntent.ProductGoal)
	}
	for _, task := range plan.UIIntent.UserTasks {
		if task.Key != "" || task.Label != "" {
			fmt.Fprintf(b, "- User task: %s (%s, priority: %s)\n", task.Key, task.Label, task.Priority)
		}
	}
	if len(plan.UIIntent.Shared.PrimaryEntities) > 0 {
		fmt.Fprintf(b, "- Shared primary entities: %s\n", strings.Join(plan.UIIntent.Shared.PrimaryEntities, ", "))
	}
	if len(plan.UIIntent.Shared.PrimaryActions) > 0 {
		fmt.Fprintf(b, "- Shared primary actions: %s\n", strings.Join(plan.UIIntent.Shared.PrimaryActions, ", "))
	}
	if plan.UIIntent.Shared.EmptyState != "" {
		fmt.Fprintf(b, "- Shared empty state: %s\n", plan.UIIntent.Shared.EmptyState)
	}
	for _, term := range sortedStringKeys(plan.UIIntent.Shared.Terminology) {
		label := plan.UIIntent.Shared.Terminology[term]
		fmt.Fprintf(b, "- Terminology: %s = %s\n", term, label)
	}
	for _, surface := range sortedSurfaceKeys(plan.UIIntent.SurfacesConfig) {
		cfg := plan.UIIntent.SurfacesConfig[surface]
		fmt.Fprintf(b, "- Surface %s role: %s\n", surface, cfg.Role)
		if len(cfg.EnabledTasks) > 0 {
			fmt.Fprintf(b, "- Surface %s enabled tasks: %s\n", surface, strings.Join(cfg.EnabledTasks, ", "))
		}
		for _, view := range sortedViewKeys(cfg.Views) {
			viewCfg := cfg.Views[view]
			fmt.Fprintf(b, "- Surface %s view %s: %s", surface, view, viewCfg.Intent)
			if viewCfg.Density != "" {
				fmt.Fprintf(b, " (density: %s)", viewCfg.Density)
			}
			if len(viewCfg.Fields) > 0 {
				fmt.Fprintf(b, " fields: %s", strings.Join(viewCfg.Fields, ", "))
			}
			b.WriteString("\n")
		}
		if len(cfg.Actions) > 0 {
			fmt.Fprintf(b, "- Surface %s actions: %s\n", surface, strings.Join(cfg.Actions, ", "))
		}
		for _, constraint := range cfg.Constraints {
			fmt.Fprintf(b, "- Surface %s constraint: %s\n", surface, constraint)
		}
	}
	if len(plan.UIIntent.Surfaces) > 0 {
		fmt.Fprintf(b, "- Legacy surfaces: %s\n", strings.Join(plan.UIIntent.Surfaces, ", "))
	}
	if plan.UIIntent.PrimaryView != "" {
		fmt.Fprintf(b, "- Primary view: %s\n", plan.UIIntent.PrimaryView)
	}
	for _, action := range plan.UIIntent.PrimaryActions {
		fmt.Fprintf(b, "- Primary action: %s\n", action)
	}
	if plan.UIIntent.ListIntent != "" {
		fmt.Fprintf(b, "- List intent: %s\n", plan.UIIntent.ListIntent)
	}
	if plan.UIIntent.FormIntent != "" {
		fmt.Fprintf(b, "- Form intent: %s\n", plan.UIIntent.FormIntent)
	}
	if plan.UIIntent.DetailIntent != "" {
		fmt.Fprintf(b, "- Detail intent: %s\n", plan.UIIntent.DetailIntent)
	}
	if plan.UIIntent.EmptyState != "" {
		fmt.Fprintf(b, "- Empty state: %s\n", plan.UIIntent.EmptyState)
	}
	for _, note := range plan.UIIntent.InteractionNotes {
		fmt.Fprintf(b, "- Interaction note: %s\n", note)
	}
	for _, note := range plan.UIIntent.SurfaceNotes {
		fmt.Fprintf(b, "- Surface note: %s\n", note)
	}
	b.WriteString("\nAPI intent:\n")
	if plan.APIIntent.Exposure != "" {
		fmt.Fprintf(b, "- Exposure: %s\n", plan.APIIntent.Exposure)
	}
	if len(plan.APIIntent.Consumers) > 0 {
		fmt.Fprintf(b, "- Consumers: %s\n", strings.Join(plan.APIIntent.Consumers, ", "))
	}
	if plan.APIIntent.Auth != "" {
		fmt.Fprintf(b, "- Auth: %s\n", plan.APIIntent.Auth)
	}
	if plan.APIIntent.BasePath != "" {
		fmt.Fprintf(b, "- Base path: %s\n", plan.APIIntent.BasePath)
	}
	for _, operation := range plan.APIIntent.Operations {
		fmt.Fprintf(b, "- Operation: %s\n", operation)
	}
	for _, note := range plan.APIIntent.RequestNotes {
		fmt.Fprintf(b, "- Request note: %s\n", note)
	}
	for _, note := range plan.APIIntent.ResponseNotes {
		fmt.Fprintf(b, "- Response note: %s\n", note)
	}
	for _, errorCase := range plan.APIIntent.ErrorCases {
		fmt.Fprintf(b, "- Error case: %s\n", errorCase)
	}
	if plan.APIIntent.Idempotency != "" {
		fmt.Fprintf(b, "- Idempotency: %s\n", plan.APIIntent.Idempotency)
	}
	if plan.APIIntent.RateLimit != "" {
		fmt.Fprintf(b, "- Rate limit: %s\n", plan.APIIntent.RateLimit)
	}
	if plan.APIIntent.Versioning != "" {
		fmt.Fprintf(b, "- Versioning: %s\n", plan.APIIntent.Versioning)
	}
	for _, note := range plan.APIIntent.CompatibilityNotes {
		fmt.Fprintf(b, "- Compatibility note: %s\n", note)
	}
	b.WriteString("\n")
}

func countChanges(changes []differ.FieldChange, typ differ.ChangeType) int {
	n := 0
	for _, change := range changes {
		if change.Type == typ {
			n++
		}
	}
	return n
}

func hasCategory(diff *differ.DiffResult, category string) bool {
	for _, change := range diff.Changes {
		if change.Category == category {
			return true
		}
	}
	return false
}

func filesWithPrefix(files []differ.AffectedFile, prefixes ...string) []string {
	var out []string
	for _, file := range files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(file.Path, prefix) {
				out = append(out, file.Path)
				break
			}
		}
	}
	return out
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSurfaceKeys(values map[string]mozi.UISurfaceIntentConfig) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedViewKeys(values map[string]mozi.UISurfaceViewConfig) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
