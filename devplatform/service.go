// Package devplatform provides the HTTP API layer for the visual development platform.
// It wraps internal/mozi core capabilities (parser, generator, differ) and exposes
// them as Gin-compatible services consumed by the admin frontend.
package devplatform

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/pangu-sutido/mozi-builder/mozi"
	moziapply "github.com/pangu-sutido/mozi-builder/mozi/apply"
	"github.com/pangu-sutido/mozi-builder/mozi/db"
	"github.com/pangu-sutido/mozi-builder/mozi/differ"
	"github.com/pangu-sutido/mozi-builder/mozi/generator"
	"github.com/pangu-sutido/mozi-builder/mozi/manifest"
	"github.com/pangu-sutido/mozi-builder/mozi/parser"

	"gopkg.in/yaml.v3"
)

// Service wraps mozi core capabilities for the dev platform HTTP API.
type Service struct {
	Store  *db.Store
	Engine *generator.Engine
}

// NewService creates a new dev platform service.
func NewService(store *db.Store, engine *generator.Engine) *Service {
	return &Service{Store: store, Engine: engine}
}

// APIEndpointOverrideInput is the editable curation payload for an OpenAPI endpoint.
type APIEndpointOverrideInput struct {
	EndpointID  string `json:"endpoint_id"`
	ModuleID    string `json:"module_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// DesignDictionaryItemInput is the editable payload for one design dictionary item.
type DesignDictionaryItemInput struct {
	Value       string   `json:"value"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	SortOrder   int      `json:"sort_order"`
	Enabled     *bool    `json:"enabled"`
}

// ============================================================================
// Model CRUD
// ============================================================================

// ModelSummary is a lightweight representation of a model for list views.
type ModelSummary struct {
	Module      string `json:"module"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Table       string `json:"table"`
	FieldCount  int    `json:"field_count"`
	RelCount    int    `json:"rel_count"`
	Version     string `json:"version"`
	SyncStatus  string `json:"sync_status"` // "synced", "modified", "new"
}

// ModuleSummary is a lightweight module representation.
type ModuleSummary struct {
	Name        string         `json:"name"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	APIPrefix   string         `json:"api_prefix"`
	ModelCount  int            `json:"model_count"`
	Models      []ModelSummary `json:"models"`
}

// ListModules returns all modules with their models.
func (s *Service) ListModules(ctx context.Context) ([]ModuleSummary, error) {
	project, err := s.Store.LoadProject()
	if err != nil {
		return nil, fmt.Errorf("load project: %w", err)
	}
	var genManifest *manifest.Manifest
	if projectRoot, err := moziapply.FindProjectRoot(); err == nil {
		genManifest, _ = manifest.Load(projectRoot)
	}

	var modules []ModuleSummary
	for _, mod := range project.Modules {
		ms := ModuleSummary{
			Name:        mod.Name,
			Label:       mod.Label,
			Description: mod.Description,
			Icon:        mod.Icon,
			APIPrefix:   mod.APIPrefix,
			ModelCount:  len(mod.Models),
		}
		for _, m := range mod.Models {
			_, _, _, _, _, version, _ := s.Store.GetModel(m.Name)
			syncStatus := "synced"
			if genManifest != nil && genManifest.NeedsRegen(m.Module+"/"+m.Name, version) {
				syncStatus = "modified"
			}
			ms.Models = append(ms.Models, ModelSummary{
				Module:      m.Module,
				Name:        m.Name,
				Label:       m.Label,
				Description: m.Description,
				Table:       m.Table,
				FieldCount:  len(m.Fields),
				RelCount:    len(m.Relations),
				Version:     version,
				SyncStatus:  syncStatus,
			})
		}
		modules = append(modules, ms)
	}
	return modules, nil
}

// CreateModule creates a business module.
func (s *Service) CreateModule(ctx context.Context, mod *mozi.ModuleIR) (*mozi.ModuleIR, error) {
	normalized, err := normalizeModuleInput(mod, "")
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.GetModule(normalized.Name); err == nil {
		return nil, fmt.Errorf("module %s already exists", normalized.Name)
	} else if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get module: %w", err)
	}
	if err := s.Store.UpsertModule(normalized); err != nil {
		return nil, fmt.Errorf("create module: %w", err)
	}
	return s.Store.GetModule(normalized.Name)
}

// UpdateModule updates module metadata. Renaming modules is intentionally not
// supported here because model refs, snapshots, and generated paths depend on it.
func (s *Service) UpdateModule(ctx context.Context, name string, mod *mozi.ModuleIR) (*mozi.ModuleIR, error) {
	existing, err := s.Store.GetModule(name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("module %s not found", name)
		}
		return nil, fmt.Errorf("get module: %w", err)
	}
	normalized, err := normalizeModuleInput(mod, name)
	if err != nil {
		return nil, err
	}
	existing.Label = normalized.Label
	existing.Description = normalized.Description
	existing.Icon = normalized.Icon
	existing.APIPrefix = normalized.APIPrefix
	if err := s.Store.UpsertModule(existing); err != nil {
		return nil, fmt.Errorf("update module: %w", err)
	}
	return s.Store.GetModule(name)
}

// DeleteModule deletes an empty module.
func (s *Service) DeleteModule(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("module name is required")
	}
	if _, err := s.Store.GetModule(name); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("module %s not found", name)
		}
		return fmt.Errorf("get module: %w", err)
	}
	models, err := s.Store.ListModelsByModule(name)
	if err != nil {
		return fmt.Errorf("list module models: %w", err)
	}
	if len(models) > 0 {
		return fmt.Errorf("module %s has %d model(s); delete or move them first", name, len(models))
	}
	if err := s.Store.DeleteModule(name); err != nil {
		return fmt.Errorf("delete module: %w", err)
	}
	return nil
}

// GetModel returns a single model with full details.
func (s *Service) GetModel(ctx context.Context, modelName string) (*mozi.ModelIR, error) {
	return s.Store.LoadModel(modelName)
}

// CreateModel creates a new model from YAML content.
func (s *Service) CreateModel(ctx context.Context, yamlContent string) (*mozi.ModelIR, error) {
	// Determine module from YAML content
	var raw struct {
		Module string `yaml:"module"`
		Model  string `yaml:"model"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if raw.Module == "" {
		return nil, fmt.Errorf("module is required in model YAML")
	}

	model, err := parser.ParseFileFromContent([]byte(yamlContent), raw.Module)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	if err := s.Store.SaveModel(model, "Created via dev platform", ""); err != nil {
		return nil, fmt.Errorf("save model: %w", err)
	}

	return model, nil
}

// CreateModelIR creates a new model from a structured ModelIR payload.
func (s *Service) CreateModelIR(ctx context.Context, model *mozi.ModelIR) (*mozi.ModelIR, error) {
	if model == nil {
		return nil, fmt.Errorf("model payload is required")
	}
	parser.NormalizeModel(model, model.Module)
	if err := s.Store.SaveModel(model, "Created via dev platform", ""); err != nil {
		return nil, fmt.Errorf("save model: %w", err)
	}
	return model, nil
}

// UpdateModel updates an existing model from YAML content (creates a new version).
func (s *Service) UpdateModel(ctx context.Context, modelName string, yamlContent string) (*mozi.ModelIR, error) {
	// Load existing model to get module
	existing, err := s.Store.LoadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("load existing model: %w", err)
	}

	model, err := parser.ParseFileFromContent([]byte(yamlContent), existing.Module)
	if err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}

	if err := s.Store.SaveModel(model, "Updated via dev platform", ""); err != nil {
		return nil, fmt.Errorf("save model: %w", err)
	}

	return model, nil
}

// UpdateModelIR updates an existing model from a structured ModelIR payload.
func (s *Service) UpdateModelIR(ctx context.Context, modelName string, model *mozi.ModelIR) (*mozi.ModelIR, error) {
	if model == nil {
		return nil, fmt.Errorf("model payload is required")
	}
	existing, err := s.Store.LoadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("load existing model: %w", err)
	}
	if model.Module == "" {
		model.Module = existing.Module
	}
	if model.Name == "" {
		model.Name = existing.Name
	}
	parser.NormalizeModel(model, existing.Module)
	if err := s.Store.SaveModel(model, "Updated via dev platform", ""); err != nil {
		return nil, fmt.Errorf("save model: %w", err)
	}
	return model, nil
}

// DeleteModel deletes a model.
func (s *Service) DeleteModel(ctx context.Context, modelName string) error {
	return s.Store.DeleteModel(modelName)
}

// ============================================================================
// ER Diagram
// ============================================================================

// ERDiagram returns a Mermaid ER diagram DSL string.
// If module is non-empty, only entities and relations for that module are included.
func (s *Service) ERDiagram(ctx context.Context, module string) (string, error) {
	project, err := s.Store.LoadProject()
	if err != nil {
		return "", fmt.Errorf("load project: %w", err)
	}
	if module != "" {
		project = filterProjectByModule(project, module)
	}
	return GenerateMermaidER(project), nil
}

// filterProjectByModule returns a new project containing only the specified module.
func filterProjectByModule(project *mozi.ProjectIR, moduleName string) *mozi.ProjectIR {
	// Find the target module
	var targetMod *mozi.ModuleIR
	for _, mod := range project.Modules {
		if mod.Name == moduleName {
			targetMod = mod
			break
		}
	}
	if targetMod == nil {
		return &mozi.ProjectIR{}
	}

	modelNames := make(map[string]bool, len(targetMod.Models))
	for _, m := range targetMod.Models {
		modelNames[m.Name] = true
	}

	filteredMod := *targetMod
	filteredMod.Models = make([]*mozi.ModelIR, 0, len(targetMod.Models))
	for _, m := range targetMod.Models {
		filteredModel := *m
		filteredModel.Relations = make([]mozi.RelationIR, 0, len(m.Relations))
		for _, r := range m.Relations {
			if modelNames[r.TargetModel] {
				filteredModel.Relations = append(filteredModel.Relations, r)
			}
		}
		filteredMod.Models = append(filteredMod.Models, &filteredModel)
	}

	return &mozi.ProjectIR{Modules: []*mozi.ModuleIR{&filteredMod}}
}

// ============================================================================
// Validation
// ============================================================================

// ValidateResult is the result of model validation.
type ValidateResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidateModel validates a model's YAML content.
func (s *Service) ValidateModel(ctx context.Context, modelName string) (*ValidateResult, error) {
	model, err := s.Store.LoadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	result := parser.Validate(model)
	vr := &ValidateResult{Valid: result.Valid}
	for _, e := range result.Errors {
		vr.Errors = append(vr.Errors, e.Error())
	}
	for _, w := range result.Warnings {
		vr.Warnings = append(vr.Warnings, w.Error())
	}
	return vr, nil
}

// ============================================================================
// Diff
// ============================================================================

// GetDiff returns a structured diff for a model.
func (s *Service) GetDiff(ctx context.Context, modelName string) (*differ.DiffResult, error) {
	model, err := s.Store.LoadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	_, _, _, _, _, currentVersion, err := s.Store.GetModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}

	// Compare with previous version — find the prior version from history
	versions, err := s.Store.ListVersions(modelName)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	var prevVersion string
	for i, v := range versions {
		if v.Version == currentVersion && i+1 < len(versions) {
			prevVersion = versions[i+1].Version
			break
		}
	}

	if prevVersion == "" {
		// No previous version — diff against empty model (new model)
		prevModel := &mozi.ModelIR{
			Module: model.Module,
			Name:   model.Name,
			Label:  model.Label,
			Admin:  mozi.AdminConfig{},
		}
		return differ.Compare(prevModel, model, prevVersion, currentVersion), nil
	}

	prevModel := s.loadModelVersionForDiff(modelName, prevVersion, model)

	return differ.Compare(prevModel, model, prevVersion, currentVersion), nil
}

// ============================================================================
// AI Change Plan
// ============================================================================

// ChangePlanStatus indicates whether a change plan has been applied to code.
type ChangePlanStatus string

const (
	ChangePlanPending ChangePlanStatus = "pending" // model changed, code not yet synced
	ChangePlanApplied ChangePlanStatus = "applied" // model changes already synced to code
	ChangePlanNoDiff  ChangePlanStatus = "no_diff" // model has no version diff
)

// ChangePlanResult describes a model change as an AI Coding task instead of a
// template overwrite operation.
type ChangePlanResult struct {
	ModelRef      string                `json:"model_ref"`
	Status        ChangePlanStatus      `json:"status"`
	Intent        string                `json:"intent"`
	Semantics     mozi.SemanticConfig   `json:"semantics"`
	UIIntent      mozi.UIIntentConfig   `json:"ui_intent"`
	APIIntent     mozi.APIIntentConfig  `json:"api_intent"`
	Diff          *differ.DiffResult    `json:"diff"`
	AffectedFiles []differ.AffectedFile `json:"affected_files"`
	Contracts     []string              `json:"contracts"`
	Tasks         []ChangePlanTask      `json:"tasks"`
	Checks        []string              `json:"checks"`
	Prompt        string                `json:"prompt"`
}

// ChangePlanTask is one actionable item in the AI Coding plan.
type ChangePlanTask struct {
	Area        string   `json:"area"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// ChangePlan returns a model-driven task contract that an AI Coding agent can
// apply as a small, reviewable diff against the current repository.
func (s *Service) ChangePlan(ctx context.Context, modelName string) (*ChangePlanResult, error) {
	modelID := modelNameFromRef(modelName)
	model, err := s.Store.LoadModel(modelID)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	diff, err := s.GetDiff(ctx, modelID)
	if err != nil {
		return nil, err
	}

	// Determine status from manifest
	status := ChangePlanPending
	if !diff.HasChanges {
		status = ChangePlanNoDiff
	} else if projectRoot, err := moziapply.FindProjectRoot(); err == nil {
		if genManifest, err := manifest.Load(projectRoot); err == nil {
			if !genManifest.NeedsRegen(diff.ModelRef, diff.ToVersion) {
				status = ChangePlanApplied
			}
		}
	}

	modelRef := model.Module + "/" + model.Name
	affectedFiles := diff.AffectedFiles()
	result := &ChangePlanResult{
		ModelRef:      modelRef,
		Status:        status,
		Intent:        buildChangeIntent(diff, status),
		Semantics:     model.Semantics,
		UIIntent:      model.UIIntent,
		APIIntent:     model.APIIntent,
		Diff:          diff,
		AffectedFiles: affectedFiles,
		Contracts:     buildContracts(status),
		Tasks:         buildChangeTasks(model, diff, affectedFiles, status),
		Checks:        buildChangeChecks(diff, status),
	}
	result.Prompt = buildChangePrompt(result)
	return result, nil
}

// SyncModel records the current model version in the manifest so that future
// change-plan requests will show status "applied" instead of "pending".
func (s *Service) SyncModel(ctx context.Context, module, name string) error {
	modelRef := module + "/" + name
	_, _, _, _, _, currentVersion, err := s.Store.GetModel(name)
	if err != nil {
		return fmt.Errorf("get model %s: %w", modelRef, err)
	}

	projectRoot, err := moziapply.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	mf, err := manifest.Load(projectRoot)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	mf.RecordGen(modelRef, currentVersion, nil)
	if err := mf.Save(); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	return nil
}

// SaveAPIEndpointOverride saves lightweight human curation for an OpenAPI endpoint.
func (s *Service) SaveAPIEndpointOverride(ctx context.Context, input APIEndpointOverrideInput) error {
	input.EndpointID = strings.TrimSpace(input.EndpointID)
	input.ModuleID = strings.TrimSpace(input.ModuleID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	if input.EndpointID == "" {
		return fmt.Errorf("endpoint_id is required")
	}
	if input.ModuleID != "" {
		if _, err := s.Store.GetModule(input.ModuleID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("module %s not found", input.ModuleID)
			}
			return fmt.Errorf("get module: %w", err)
		}
	}
	return s.Store.UpsertAPIEndpointOverride(db.APIEndpointOverride{
		EndpointID:  input.EndpointID,
		ModuleID:    input.ModuleID,
		DisplayName: input.DisplayName,
		Description: input.Description,
		UpdatedBy:   "dev-platform",
	})
}

// ListDesignDictionaryItems returns business-maintained dictionary items.
func (s *Service) ListDesignDictionaryItems(ctx context.Context, dictionaryID string, includeDisabled bool) ([]db.DesignDictionaryItem, error) {
	dictionaryID = strings.TrimSpace(dictionaryID)
	if dictionaryID == "" {
		return nil, fmt.Errorf("dictionary id is required")
	}
	return s.Store.ListDesignDictionaryItems(dictionaryID, includeDisabled)
}

// SaveDesignDictionaryItem saves one business-maintained dictionary option.
func (s *Service) SaveDesignDictionaryItem(ctx context.Context, dictionaryID string, input DesignDictionaryItemInput) error {
	dictionaryID = strings.TrimSpace(dictionaryID)
	value := strings.TrimSpace(input.Value)
	label := strings.TrimSpace(input.Label)
	if dictionaryID == "" {
		return fmt.Errorf("dictionary id is required")
	}
	if value == "" {
		return fmt.Errorf("dictionary item value is required")
	}
	if label == "" {
		label = value
	}
	aliases := make([]string, 0, len(input.Aliases))
	seenAliases := map[string]bool{}
	for _, alias := range input.Aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || seenAliases[alias] {
			continue
		}
		seenAliases[alias] = true
		aliases = append(aliases, alias)
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return s.Store.UpsertDesignDictionaryItem(db.DesignDictionaryItem{
		DictionaryID: dictionaryID,
		Value:        value,
		Label:        label,
		Description:  strings.TrimSpace(input.Description),
		Aliases:      aliases,
		SortOrder:    input.SortOrder,
		Enabled:      enabled,
	})
}

// DeleteDesignDictionaryItem deletes one business-maintained dictionary option.
func (s *Service) DeleteDesignDictionaryItem(ctx context.Context, dictionaryID, value string) error {
	dictionaryID = strings.TrimSpace(dictionaryID)
	value = strings.TrimSpace(value)
	if dictionaryID == "" {
		return fmt.Errorf("dictionary id is required")
	}
	if value == "" {
		return fmt.Errorf("dictionary item value is required")
	}
	return s.Store.DeleteDesignDictionaryItem(dictionaryID, value)
}

// ============================================================================
// Helpers
// ============================================================================

func normalizeModuleInput(mod *mozi.ModuleIR, fixedName string) (*mozi.ModuleIR, error) {
	if mod == nil {
		return nil, fmt.Errorf("module payload is required")
	}
	out := *mod
	if fixedName != "" {
		out.Name = fixedName
	}
	out.Name = strings.TrimSpace(out.Name)
	out.Label = strings.TrimSpace(out.Label)
	out.Description = strings.TrimSpace(out.Description)
	out.Icon = strings.TrimSpace(out.Icon)
	out.APIPrefix = strings.Trim(strings.TrimSpace(out.APIPrefix), "/")
	if out.Name == "" {
		return nil, fmt.Errorf("module name is required")
	}
	if !isStableIdentifier(out.Name) {
		return nil, fmt.Errorf("module name must use letters, numbers, underscore, or hyphen")
	}
	if out.Label == "" {
		out.Label = out.Name
	}
	if out.APIPrefix == "" {
		out.APIPrefix = out.Name
	}
	return &out, nil
}

func isStableIdentifier(value string) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// NewDevPlatformEngine creates a generator engine for dev platform use.
func NewDevPlatformEngine() *generator.Engine {
	tmplFS, _ := fs.Sub(mozi.EmbeddedTemplates, "templates")
	if tmplFS == nil {
		return generator.NewEngine(nil)
	}
	return generator.NewEngine(tmplFS)
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

func modelNameFromRef(ref string) string {
	if _, model, ok := strings.Cut(ref, "/"); ok {
		return model
	}
	return ref
}

func buildChangeIntent(diff *differ.DiffResult, status ChangePlanStatus) string {
	switch status {
	case ChangePlanNoDiff:
		return "No model changes detected. Keep code unchanged unless manual cleanup is explicitly requested."
	case ChangePlanApplied:
		return fmt.Sprintf("Model %s v%s changes have already been applied to the repository. No further code changes needed.", diff.ModelRef, diff.ToVersion)
	case ChangePlanPending:
		// fallthrough to diff-based intent
	}

	if diff == nil || !diff.HasChanges {
		return "No model changes detected. Keep code unchanged unless manual cleanup is explicitly requested."
	}

	var parts []string
	added := countPlanChanges(diff.Changes, differ.ChangeAdded)
	modified := countPlanChanges(diff.Changes, differ.ChangeModified)
	removed := countPlanChanges(diff.Changes, differ.ChangeRemoved)
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

func buildContracts(status ChangePlanStatus) []string {
	if status == ChangePlanApplied || status == ChangePlanNoDiff {
		return []string{"No pending model changes — skip code generation."}
	}
	return []string{
		"Treat the design DB model as the source of truth; YAML snapshots are exchange artifacts.",
		"Generate a minimal patch against the current repository instead of overwriting handwritten files.",
		"Preserve existing custom business logic, UI behavior, comments, imports, and local formatting.",
		"Use existing module layout, API helpers, stores, route style, and component patterns.",
		"Treat API intent as a first-class contract for routes, DTOs, auth, errors, docs, and compatibility.",
		"Keep deterministic generated artifacts small; put business behavior in normal application code.",
		"Do not modify unrelated modules or files unless the model relationship requires it.",
	}
}

func buildChangeTasks(model *mozi.ModelIR, diff *differ.DiffResult, affectedFiles []differ.AffectedFile, status ChangePlanStatus) []ChangePlanTask {
	if status == ChangePlanApplied {
		return []ChangePlanTask{
			{
				Area:        "review",
				Description: fmt.Sprintf("Model %s v%s already synced. Run mozi sync if the manifest is stale, or skip.", diff.ModelRef, diff.ToVersion),
			},
		}
	}

	if diff == nil || !diff.HasChanges {
		return []ChangePlanTask{
			{
				Area:        "review",
				Description: "Confirm the model has no pending changes and avoid unnecessary code churn.",
			},
		}
	}

	var tasks []ChangePlanTask
	if hasPlanCategory(diff, "field") || hasPlanCategory(diff, "relation") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "backend",
			Description: "Update ent schema, request/response DTOs, handler binding, and service persistence behavior to match the model changes.",
			Files:       filesWithPrefix(affectedFiles, "ent/schema/", "internal/model/", "internal/handler/", "internal/service/"),
		})
	}
	if hasPlanCategory(diff, "field") || hasPlanCategory(diff, "admin") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "frontend",
			Description: "Update API types, store usage, list columns, search filters, and form fields using existing admin UI patterns.",
			Files:       filesWithPrefix(affectedFiles, "admin/src/api/", "admin/src/pages/", "admin/src/stores/"),
		})
	}
	if hasPlanCategory(diff, "relation") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "relationship",
			Description: "Check relation target modules and update selectors, joins, eager loading, and display labels only where the current code needs them.",
		})
	}
	if hasPlanCategory(diff, "admin") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "admin-config",
			Description: "Apply list/search/sort/page-size behavior without rewriting unrelated page logic.",
		})
	}
	if hasPlanCategory(diff, "semantics") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "semantics",
			Description: "Translate business purpose, user value, permissions, lifecycle, and business rules into service behavior and tests where needed.",
			Files:       filesWithPrefix(affectedFiles, "internal/model/", "internal/handler/", "internal/service/"),
		})
	}
	if hasPlanCategory(diff, "ui_intent") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "ui-intent",
			Description: "Update admin, desktop, and mini program UI surfaces as applicable, including list, form, detail, empty state, primary actions, navigation, and surface-specific behavior.",
			Files:       filesWithPrefix(affectedFiles, "admin/src/", "../memflow-desktop/src/", "../memflow-miniapp/src/"),
		})
	}
	if hasPlanCategory(diff, "api_intent") {
		tasks = append(tasks, ChangePlanTask{
			Area:        "api-contract",
			Description: "Update public/internal API routes, DTOs, auth behavior, validation, error responses, idempotency, versioning, and OpenAPI docs to match the API intent.",
			Files:       filesWithPrefix(affectedFiles, "internal/handler/", "internal/service/", "internal/model/", "docs/"),
		})
	}

	tasks = append(tasks, ChangePlanTask{
		Area:        "snapshot",
		Description: fmt.Sprintf("After verification, export the %s model snapshot so models/ stays aligned with the design DB, then run mozi sync.", model.Module),
		Files:       []string{fmt.Sprintf("models/%s/%s.yaml", model.Module, toSnake(model.Name))},
	})
	return tasks
}

func (s *Service) loadModelVersionForDiff(modelName string, version string, current *mozi.ModelIR) *mozi.ModelIR {
	prev, err := s.Store.LoadModelVersion(modelName, version, current.Module)
	if err != nil {
		// Fallback: return minimal model with current identity
		return &mozi.ModelIR{
			Module: current.Module,
			Name:   current.Name,
			Label:  current.Label,
			Admin:  mozi.AdminConfig{},
		}
	}
	// Ensure model identity metadata matches current
	if prev.Label == "" {
		prev.Label = current.Label
	}
	prev.Name = current.Name
	prev.Module = current.Module
	return prev
}

func buildChangeChecks(diff *differ.DiffResult, status ChangePlanStatus) []string {
	checks := []string{
		"mozi validate",
	}
	if status == ChangePlanApplied {
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

func buildChangePrompt(plan *ChangePlanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Change plan for %s [status: %s]\n\n", plan.ModelRef, plan.Status)
	fmt.Fprintf(&b, "Intent: %s\n\n", plan.Intent)
	writeSemanticPromptSection(&b, plan)
	if plan.Diff != nil && len(plan.Diff.Changes) > 0 {
		b.WriteString("Model changes:\n")
		for _, change := range plan.Diff.Changes {
			fmt.Fprintf(&b, "- %s\n", change.Detail)
		}
		b.WriteString("\n")
	}
	if len(plan.AffectedFiles) > 0 {
		b.WriteString("Likely affected files:\n")
		for _, file := range plan.AffectedFiles {
			fmt.Fprintf(&b, "- %s: %s\n", file.Path, file.Description)
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

func writeSemanticPromptSection(b *strings.Builder, plan *ChangePlanResult) {
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

func countPlanChanges(changes []differ.FieldChange, typ differ.ChangeType) int {
	n := 0
	for _, change := range changes {
		if change.Type == typ {
			n++
		}
	}
	return n
}

func hasPlanCategory(diff *differ.DiffResult, category string) bool {
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
