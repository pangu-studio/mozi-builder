package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/parser"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// Store wraps the database connection and provides CRUD operations.
// ============================================================================

// Store provides CRUD operations for the design database.
type Store struct {
	DB *sql.DB
}

func (s *Store) ListErrorCodes() ([]mozi.ErrorCodeIR, error) {
	rows, err := s.DB.Query(`SELECT code, domain, http_status, category, message, consumer_facing, retryable, details_schema, i18n_key, deprecated FROM error_codes ORDER BY domain, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []mozi.ErrorCodeIR
	for rows.Next() {
		var item mozi.ErrorCodeIR
		if err := rows.Scan(&item.Code, &item.Domain, &item.HTTPStatus, &item.Category, &item.Message, &item.ConsumerFacing, &item.Retryable, &item.DetailsSchema, &item.I18nKey, &item.Deprecated); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) UpsertErrorCode(item mozi.ErrorCodeIR) error {
	_, err := s.DB.Exec(`INSERT INTO error_codes (code, domain, http_status, category, message, consumer_facing, retryable, details_schema, i18n_key, deprecated) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (code) DO UPDATE SET domain=EXCLUDED.domain,http_status=EXCLUDED.http_status,category=EXCLUDED.category,message=EXCLUDED.message,consumer_facing=EXCLUDED.consumer_facing,retryable=EXCLUDED.retryable,details_schema=EXCLUDED.details_schema,i18n_key=EXCLUDED.i18n_key,deprecated=EXCLUDED.deprecated,updated_at=CURRENT_TIMESTAMP`, item.Code, item.Domain, item.HTTPStatus, item.Category, item.Message, item.ConsumerFacing, item.Retryable, item.DetailsSchema, item.I18nKey, item.Deprecated)
	return err
}

func (s *Store) DeleteErrorCode(code string) error {
	_, err := s.DB.Exec(`DELETE FROM error_codes WHERE code=$1`, code)
	return err
}

// NewStore creates a new Store.
func NewStore(db *sql.DB) *Store {
	return &Store{DB: db}
}

// ============================================================================
// Module CRUD
// ============================================================================

// UpsertModule inserts or updates a module.
func (s *Store) UpsertModule(mod *mozi.ModuleIR) error {
	_, err := s.DB.Exec(`
		INSERT INTO modules (id, label, description, icon, api_prefix)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			label = EXCLUDED.label,
			description = EXCLUDED.description,
			icon = EXCLUDED.icon,
			api_prefix = EXCLUDED.api_prefix,
			updated_at = CURRENT_TIMESTAMP
	`, mod.Name, mod.Label, mod.Description, mod.Icon, mod.APIPrefix)
	return err
}

// GetModule returns a module by name.
func (s *Store) GetModule(name string) (*mozi.ModuleIR, error) {
	row := s.DB.QueryRow(`
		SELECT id, label, description, icon, api_prefix FROM modules WHERE id = $1
	`, name)

	mod := &mozi.ModuleIR{}
	var desc, icon sql.NullString
	err := row.Scan(&mod.Name, &mod.Label, &desc, &icon, &mod.APIPrefix)
	if err != nil {
		return nil, err
	}
	mod.Description = desc.String
	mod.Icon = icon.String
	return mod, nil
}

// ListModules returns all modules.
func (s *Store) ListModules() ([]*mozi.ModuleIR, error) {
	rows, err := s.DB.Query(`
		SELECT id, label, description, icon, api_prefix FROM modules ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []*mozi.ModuleIR
	for rows.Next() {
		mod := &mozi.ModuleIR{}
		var desc, icon sql.NullString
		if err := rows.Scan(&mod.Name, &mod.Label, &desc, &icon, &mod.APIPrefix); err != nil {
			return nil, err
		}
		mod.Description = desc.String
		mod.Icon = icon.String
		mods = append(mods, mod)
	}
	return mods, rows.Err()
}

// DeleteModule deletes a module and all its models (cascade).
func (s *Store) DeleteModule(name string) error {
	_, err := s.DB.Exec(`DELETE FROM modules WHERE id = $1`, name)
	return err
}

// ============================================================================
// Model CRUD
// ============================================================================

// UpsertModel inserts or updates a model record.
func (s *Store) UpsertModel(modelID, moduleID, label, description, tableName string) error {
	_, err := s.DB.Exec(`
		INSERT INTO models (id, module_id, label, description, table_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			module_id = EXCLUDED.module_id,
			label = EXCLUDED.label,
			description = EXCLUDED.description,
			table_name = EXCLUDED.table_name,
			updated_at = CURRENT_TIMESTAMP
	`, modelID, moduleID, label, description, tableName)
	return err
}

// GetModel returns a model by ID.
func (s *Store) GetModel(modelID string) (modelIDOut, moduleID, label, description, tableName string, currentVersion string, err error) {
	row := s.DB.QueryRow(`
		SELECT id, module_id, label, description, table_name, current_version FROM models WHERE id = $1
	`, modelID)
	var desc sql.NullString
	err = row.Scan(&modelIDOut, &moduleID, &label, &desc, &tableName, &currentVersion)
	description = desc.String
	return
}

// ListModelsByModule returns all model IDs in a module.
func (s *Store) ListModelsByModule(moduleID string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT id FROM models WHERE module_id = $1 ORDER BY id`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IncrementModelVersion generates a new timestamp-based version and updates the model.
func (s *Store) IncrementModelVersion(modelID string) (string, error) {
	newVersion := time.Now().Format("20060102150405")
	// Check for collision — if this timestamp already exists for this model, append counter
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*) FROM model_versions WHERE model_id = $1 AND version = $2
	`, modelID, newVersion).Scan(&count)
	if err == nil && count > 0 {
		newVersion = fmt.Sprintf("%s_%d", newVersion, count+1)
	}
	_, err = s.DB.Exec(`
		UPDATE models SET current_version = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, newVersion, modelID)
	return newVersion, err
}

// DeleteModel deletes a model.
func (s *Store) DeleteModel(modelID string) error {
	_, err := s.DB.Exec(`DELETE FROM models WHERE id = $1`, modelID)
	return err
}

// ============================================================================
// Version CRUD
// ============================================================================

// CreateVersion creates a new version record with a YAML snapshot.
func (s *Store) CreateVersion(modelID string, version string, yamlSnapshot, changeSummary, createdBy string) error {
	_, err := s.DB.Exec(`
		INSERT INTO model_versions (model_id, version, yaml_snapshot, change_summary, created_by)
		VALUES ($1, $2, $3, $4, $5)
	`, modelID, version, yamlSnapshot, changeSummary, createdBy)
	return err
}

// GetVersionYAML returns the YAML snapshot for a specific model version.
func (s *Store) GetVersionYAML(modelID string, version string) (string, error) {
	var yaml string
	err := s.DB.QueryRow(`
		SELECT yaml_snapshot FROM model_versions WHERE model_id = $1 AND version = $2
	`, modelID, version).Scan(&yaml)
	return yaml, err
}

// GetLatestVersion returns the latest version string for a model.
func (s *Store) GetLatestVersion(modelID string) (string, error) {
	var v sql.NullString
	err := s.DB.QueryRow(`
		SELECT version FROM model_versions WHERE model_id = $1 ORDER BY id DESC LIMIT 1
	`, modelID).Scan(&v)
	if err != nil {
		return "", err
	}
	if v.Valid {
		return v.String, nil
	}
	return "1", nil
}

// ListVersions returns version history for a model.
func (s *Store) ListVersions(modelID string) ([]VersionInfo, error) {
	rows, err := s.DB.Query(`
		SELECT version, change_summary, created_by, created_at, diff_summary
		FROM model_versions WHERE model_id = $1 ORDER BY created_at DESC
	`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []VersionInfo
	for rows.Next() {
		var v VersionInfo
		var createdAt time.Time
		var diffSummary sql.NullString
		if err := rows.Scan(&v.Version, &v.ChangeSummary, &v.CreatedBy, &createdAt, &diffSummary); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt.Format(time.RFC3339)
		v.DiffSummary = diffSummary.String
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// VersionInfo holds version metadata.
type VersionInfo struct {
	Version       string `json:"version"`
	ChangeSummary string `json:"change_summary"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	// DiffSummary holds the cached JSON diff snapshot (differ.DiffSummary) for
	// this version vs its predecessor. Not serialized directly — the service
	// layer parses and attaches it to the API response. Empty when not yet
	// computed (lazily backfilled on first history read).
	DiffSummary string `json:"-"`
}

// PreviousVersion returns the immediate predecessor version of `version`
// (the next older one by insertion order), or "" if `version` is the first.
func (s *Store) PreviousVersion(modelID string, version string) (string, error) {
	var v sql.NullString
	err := s.DB.QueryRow(`
		SELECT version FROM model_versions
		WHERE model_id = $1
		  AND id < (SELECT id FROM model_versions WHERE model_id = $1 AND version = $2)
		ORDER BY id DESC LIMIT 1
	`, modelID, version).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v.String, nil
}

// UpdateVersionDiffSummary persists the JSON-encoded differ.DiffSummary for a
// version. Passing diffJSON == "" stores NULL, marking the summary as not computed.
func (s *Store) UpdateVersionDiffSummary(modelID string, version string, diffJSON string) error {
	var arg any
	if diffJSON == "" {
		arg = nil
	} else {
		arg = diffJSON
	}
	_, err := s.DB.Exec(`
		UPDATE model_versions SET diff_summary = $1 WHERE model_id = $2 AND version = $3
	`, arg, modelID, version)
	return err
}

// ============================================================================
// API endpoint override CRUD
// ============================================================================

// APIEndpointOverride stores human curation for an OpenAPI-derived endpoint.
type APIEndpointOverride struct {
	EndpointID  string `json:"endpoint_id"`
	ModuleID    string `json:"module_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	UpdatedBy   string `json:"updated_by"`
	UpdatedAt   string `json:"updated_at"`
}

// UpsertAPIEndpointOverride inserts or updates endpoint curation metadata.
func (s *Store) UpsertAPIEndpointOverride(override APIEndpointOverride) error {
	if err := s.ensureAPIEndpointOverridesTable(); err != nil {
		return err
	}
	_, err := s.DB.Exec(`
		INSERT INTO api_endpoint_overrides (endpoint_id, module_id, display_name, description, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint_id) DO UPDATE SET
			module_id = EXCLUDED.module_id,
			display_name = EXCLUDED.display_name,
			description = EXCLUDED.description,
			updated_by = EXCLUDED.updated_by,
			updated_at = CURRENT_TIMESTAMP
	`, override.EndpointID, override.ModuleID, override.DisplayName, override.Description, override.UpdatedBy)
	return err
}

// ListAPIEndpointOverrides returns all endpoint curation records.
func (s *Store) ListAPIEndpointOverrides() (map[string]APIEndpointOverride, error) {
	if err := s.ensureAPIEndpointOverridesTable(); err != nil {
		return nil, err
	}
	rows, err := s.DB.Query(`
		SELECT endpoint_id, module_id, display_name, description, updated_by, updated_at
		FROM api_endpoint_overrides
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := map[string]APIEndpointOverride{}
	for rows.Next() {
		var item APIEndpointOverride
		var updatedAt time.Time
		if err := rows.Scan(&item.EndpointID, &item.ModuleID, &item.DisplayName, &item.Description, &item.UpdatedBy, &updatedAt); err != nil {
			return nil, err
		}
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		overrides[item.EndpointID] = item
	}
	return overrides, rows.Err()
}

func (s *Store) ensureAPIEndpointOverridesTable() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS api_endpoint_overrides (
			endpoint_id   TEXT PRIMARY KEY,
			module_id     TEXT DEFAULT '',
			display_name  TEXT DEFAULT '',
			description   TEXT DEFAULT '',
			updated_by    TEXT DEFAULT '',
			updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// ============================================================================
// Design dictionary CRUD
// ============================================================================

// DesignDictionaryItem stores one business-maintained option in the design DB.
type DesignDictionaryItem struct {
	DictionaryID string   `json:"dictionary_id"`
	Value        string   `json:"value"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Aliases      []string `json:"aliases"`
	SortOrder    int      `json:"sort_order"`
	Enabled      bool     `json:"enabled"`
	UpdatedAt    string   `json:"updated_at"`
}

// ListDesignDictionaryItems returns all items in one dictionary.
func (s *Store) ListDesignDictionaryItems(dictionaryID string, includeDisabled bool) ([]DesignDictionaryItem, error) {
	if err := s.ensureDesignDictionariesTable(); err != nil {
		return nil, err
	}
	query := `
		SELECT dictionary_id, value, label, description, aliases, sort_order, enabled, updated_at
		FROM design_dictionary_items
		WHERE dictionary_id = $1
	`
	if !includeDisabled {
		query += ` AND enabled = TRUE`
	}
	query += ` ORDER BY sort_order, value`

	rows, err := s.DB.Query(query, dictionaryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DesignDictionaryItem
	for rows.Next() {
		var item DesignDictionaryItem
		var aliasesJSON string
		var updatedAt time.Time
		if err := rows.Scan(
			&item.DictionaryID,
			&item.Value,
			&item.Label,
			&item.Description,
			&aliasesJSON,
			&item.SortOrder,
			&item.Enabled,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if aliasesJSON != "" {
			_ = json.Unmarshal([]byte(aliasesJSON), &item.Aliases)
		}
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpsertDesignDictionaryItem inserts or updates a dictionary item.
func (s *Store) UpsertDesignDictionaryItem(item DesignDictionaryItem) error {
	if err := s.ensureDesignDictionariesTable(); err != nil {
		return err
	}
	aliasesJSON := mustMarshalJSON(item.Aliases)
	_, err := s.DB.Exec(`
		INSERT INTO design_dictionary_items
			(dictionary_id, value, label, description, aliases, sort_order, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (dictionary_id, value) DO UPDATE SET
			label = EXCLUDED.label,
			description = EXCLUDED.description,
			aliases = EXCLUDED.aliases,
			sort_order = EXCLUDED.sort_order,
			enabled = EXCLUDED.enabled,
			updated_at = CURRENT_TIMESTAMP
	`, item.DictionaryID, item.Value, item.Label, item.Description, aliasesJSON, item.SortOrder, item.Enabled)
	return err
}

// DeleteDesignDictionaryItem deletes a dictionary item.
func (s *Store) DeleteDesignDictionaryItem(dictionaryID, value string) error {
	if err := s.ensureDesignDictionariesTable(); err != nil {
		return err
	}
	_, err := s.DB.Exec(`
		DELETE FROM design_dictionary_items WHERE dictionary_id = $1 AND value = $2
	`, dictionaryID, value)
	return err
}

func (s *Store) ensureDesignDictionariesTable() error {
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS design_dictionaries (
			id          TEXT PRIMARY KEY,
			label       TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS design_dictionary_items (
			dictionary_id TEXT NOT NULL REFERENCES design_dictionaries(id) ON DELETE CASCADE,
			value         TEXT NOT NULL,
			label         TEXT NOT NULL,
			description   TEXT DEFAULT '',
			aliases       TEXT DEFAULT '[]',
			sort_order    INT DEFAULT 0,
			enabled       BOOLEAN DEFAULT TRUE,
			created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (dictionary_id, value)
		);

		CREATE INDEX IF NOT EXISTS idx_design_dictionary_items_dictionary_id
			ON design_dictionary_items(dictionary_id, sort_order, value);

		INSERT INTO design_dictionaries (id, label, description)
		VALUES ('api_consumers', 'API 调用方', '按当前业务维护 API 意图中的调用方选项')
		ON CONFLICT (id) DO NOTHING;
	`)
	return err
}

// ============================================================================
// Field CRUD
// ============================================================================

// SaveFields replaces all fields for a model version.
func (s *Store) SaveFields(modelID string, version string, fields []mozi.FieldIR) error {
	// Delete existing fields for this version
	_, err := s.DB.Exec(`DELETE FROM model_fields WHERE model_id = $1 AND version = $2`, modelID, version)
	if err != nil {
		return err
	}

	for i, f := range fields {
		enumJSON := mustMarshalJSON(f.EnumValues)
		defaultVal := ""
		if f.Default != nil {
			defaultVal = *f.Default
		}
		_, err := s.DB.Exec(`
			INSERT INTO model_fields (model_id, version, name, label, type, required, unique_flag,
				sensitive, default_val, enum_values, form_type, searchable, listable, editable,
				sort_order, is_primary, generated, auto_now_add, auto_now)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		`, modelID, version, f.Name, f.Label, string(f.Type), f.Required, f.Unique,
			f.Sensitive, defaultVal, enumJSON, f.FormType, f.Searchable, f.Listable, f.Editable,
			i, f.Primary, string(f.Generated), f.AutoNowAdd, f.AutoNow)
		if err != nil {
			return fmt.Errorf("save field %s: %w", f.Name, err)
		}
	}
	return nil
}

// GetFields returns all fields for a model version.
func (s *Store) GetFields(modelID string, version string) ([]mozi.FieldIR, error) {
	rows, err := s.DB.Query(`
		SELECT name, label, type, required, unique_flag, sensitive, default_val, enum_values,
			form_type, searchable, listable, editable, sort_order, is_primary, generated,
			auto_now_add, auto_now
		FROM model_fields WHERE model_id = $1 AND version = $2 ORDER BY sort_order
	`, modelID, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []mozi.FieldIR
	for rows.Next() {
		var f mozi.FieldIR
		var typeStr, defaultVal, enumJSON, generated, formType sql.NullString
		var sortOrder int
		err := rows.Scan(&f.Name, &f.Label, &typeStr, &f.Required, &f.Unique,
			&f.Sensitive, &defaultVal, &enumJSON, &formType, &f.Searchable, &f.Listable,
			&f.Editable, &sortOrder, &f.Primary, &generated, &f.AutoNowAdd, &f.AutoNow)
		if err != nil {
			return nil, err
		}
		f.Type = mozi.FieldType(typeStr.String)
		f.FormType = formType.String
		f.Generated = mozi.GeneratedType(generated.String)
		if defaultVal.Valid && defaultVal.String != "" {
			dv := defaultVal.String
			f.Default = &dv
		}
		if enumJSON.Valid && enumJSON.String != "" {
			json.Unmarshal([]byte(enumJSON.String), &f.EnumValues)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// ============================================================================
// Relation CRUD
// ============================================================================

// SaveRelations replaces all relations for a model version.
func (s *Store) SaveRelations(modelID string, version string, relations []mozi.RelationIR) error {
	_, err := s.DB.Exec(`DELETE FROM model_relations WHERE model_id = $1 AND version = $2`, modelID, version)
	if err != nil {
		return err
	}

	for _, r := range relations {
		_, err := s.DB.Exec(`
			INSERT INTO model_relations (model_id, version, name, label, type, target_module, target_model,
				back_ref, cascade, required, unique_flag)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, modelID, version, r.Name, r.Label, string(r.Type), r.TargetModule, r.TargetModel,
			r.BackRef, r.Cascade, r.Required, r.Unique)
		if err != nil {
			return fmt.Errorf("save relation %s: %w", r.Name, err)
		}
	}
	return nil
}

// GetRelations returns all relations for a model version.
func (s *Store) GetRelations(modelID string, version string) ([]mozi.RelationIR, error) {
	rows, err := s.DB.Query(`
		SELECT name, label, type, target_module, target_model, back_ref, cascade, required, unique_flag
		FROM model_relations WHERE model_id = $1 AND version = $2 ORDER BY id
	`, modelID, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []mozi.RelationIR
	for rows.Next() {
		var r mozi.RelationIR
		var typeStr string
		err := rows.Scan(&r.Name, &r.Label, &typeStr, &r.TargetModule, &r.TargetModel,
			&r.BackRef, &r.Cascade, &r.Required, &r.Unique)
		if err != nil {
			return nil, err
		}
		r.Type = mozi.RelationType(typeStr)
		r.Target = r.TargetModule + "/" + r.TargetModel
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// ============================================================================
// Admin Config CRUD
// ============================================================================

// SaveAdmin saves admin config for a model version.
func (s *Store) SaveAdmin(modelID string, version string, admin mozi.AdminConfig) error {
	_, err := s.DB.Exec(`DELETE FROM model_admin WHERE model_id = $1 AND version = $2`, modelID, version)
	if err != nil {
		return err
	}

	listCols := mustMarshalJSON(admin.ListColumns)
	searchFields := mustMarshalJSON(admin.SearchFields)

	_, err = s.DB.Exec(`
		INSERT INTO model_admin (model_id, version, list_columns, search_fields, default_sort, default_order, page_size)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, modelID, version, listCols, searchFields, admin.DefaultSort, admin.DefaultOrder, admin.PageSize)
	return err
}

// GetAdmin returns admin config for a model version.
func (s *Store) GetAdmin(modelID string, version string) (*mozi.AdminConfig, error) {
	row := s.DB.QueryRow(`
		SELECT list_columns, search_fields, default_sort, default_order, page_size
		FROM model_admin WHERE model_id = $1 AND version = $2
	`, modelID, version)

	var admin mozi.AdminConfig
	var listCols, searchFields string
	err := row.Scan(&listCols, &searchFields, &admin.DefaultSort, &admin.DefaultOrder, &admin.PageSize)
	if err == sql.ErrNoRows {
		return &mozi.AdminConfig{PageSize: 20, DefaultOrder: "desc", DefaultSort: "created_at"}, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(listCols), &admin.ListColumns)
	json.Unmarshal([]byte(searchFields), &admin.SearchFields)
	return &admin, nil
}

// ============================================================================
// High-level load: DB → ModelIR
// ============================================================================

// LoadModel loads a complete ModelIR from the database at its current version.
func (s *Store) LoadModel(modelID string) (*mozi.ModelIR, error) {
	modelIDOut, moduleID, label, desc, tableName, version, err := s.GetModel(modelID)
	if err != nil {
		return nil, fmt.Errorf("get model %s: %w", modelID, err)
	}

	if snapshot, err := s.GetVersionYAML(modelIDOut, version); err == nil && strings.TrimSpace(snapshot) != "" {
		if model, err := parser.ParseFileFromContent([]byte(snapshot), moduleID); err == nil {
			if model.Name == "" {
				model.Name = modelIDOut
			}
			return model, nil
		}
	}

	fields, err := s.GetFields(modelIDOut, version)
	if err != nil {
		return nil, fmt.Errorf("get fields: %w", err)
	}
	relations, err := s.GetRelations(modelIDOut, version)
	if err != nil {
		return nil, fmt.Errorf("get relations: %w", err)
	}
	admin, err := s.GetAdmin(modelIDOut, version)
	if err != nil {
		return nil, fmt.Errorf("get admin: %w", err)
	}

	return &mozi.ModelIR{
		SchemaVersion: mozi.CurrentSchemaVersion,
		Module:        moduleID,
		Name:          modelIDOut,
		Label:         label,
		Description:   desc,
		Table:         tableName,
		Fields:        fields,
		Relations:     relations,
		Admin:         *admin,
	}, nil
}

// LoadModelVersion loads a complete ModelIR at a specific version from the database.
// It prefers the YAML snapshot (which includes semantics/ui_intent/api_intent),
// and falls back to structured tables only when no snapshot is available.
// This is the single source of truth for historical model versions — CLI and API
// should both use this function to avoid divergent diff behavior.
func (s *Store) LoadModelVersion(modelID string, version string, fallbackModule string) (*mozi.ModelIR, error) {
	// Prefer YAML snapshot — contains full ModelIR including semantics/ui_intent/api_intent
	if snapshot, err := s.GetVersionYAML(modelID, version); err == nil && strings.TrimSpace(snapshot) != "" {
		if model, err := parser.ParseFileFromContent([]byte(snapshot), fallbackModule); err == nil {
			if model.Name == "" {
				model.Name = modelID
			}
			return model, nil
		}
	}

	// Fallback to structured tables (fields/relations/admin only — semantics/ui_intent/api_intent
	// are stored only in the YAML snapshot, not in separate tables)
	fields, err := s.GetFields(modelID, version)
	if err != nil {
		return nil, fmt.Errorf("get fields at version %s: %w", version, err)
	}
	relations, err := s.GetRelations(modelID, version)
	if err != nil {
		return nil, fmt.Errorf("get relations at version %s: %w", version, err)
	}
	admin, err := s.GetAdmin(modelID, version)
	if err != nil {
		return nil, fmt.Errorf("get admin at version %s: %w", version, err)
	}
	if admin == nil {
		admin = &mozi.AdminConfig{}
	}

	return &mozi.ModelIR{
		SchemaVersion: mozi.CurrentSchemaVersion,
		Module:        fallbackModule,
		Name:          modelID,
		Fields:        fields,
		Relations:     relations,
		Admin:         *admin,
	}, nil
}

// LoadProject loads the entire project (all modules with all models) from the database.
func (s *Store) LoadProject() (*mozi.ProjectIR, error) {
	mods, err := s.ListModules()
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}

	project := &mozi.ProjectIR{SchemaVersion: mozi.CurrentSchemaVersion, Name: "memflow-cloud"}
	project.ErrorCodes, _ = s.ListErrorCodes()

	for _, mod := range mods {
		modelIDs, err := s.ListModelsByModule(mod.Name)
		if err != nil {
			return nil, fmt.Errorf("list models for module %s: %w", mod.Name, err)
		}

		for _, mid := range modelIDs {
			m, err := s.LoadModel(mid)
			if err != nil {
				return nil, fmt.Errorf("load model %s: %w", mid, err)
			}
			mod.Models = append(mod.Models, m)
		}

		project.Modules = append(project.Modules, mod)
	}

	return project, nil
}

// ============================================================================
// High-level save: ModelIR → DB
// ============================================================================

// SaveModel persists a complete model IR to the database.
// It upserts the module, model, creates a new version, and saves all fields/relations/admin.
func (s *Store) SaveModel(model *mozi.ModelIR, changeSummary, createdBy string) error {
	// Upsert module — preserve existing metadata when the module already exists
	mod := &mozi.ModuleIR{
		Name:      model.Module,
		Label:     model.Module,
		APIPrefix: model.Module,
	}
	if existing, err := s.GetModule(model.Module); err == nil {
		// Module already exists: keep user-configured label, description, icon, api_prefix
		mod.Label = existing.Label
		mod.Description = existing.Description
		mod.Icon = existing.Icon
		mod.APIPrefix = existing.APIPrefix
	}
	if err := s.UpsertModule(mod); err != nil {
		return fmt.Errorf("upsert module: %w", err)
	}

	// Upsert model
	if err := s.UpsertModel(model.Name, model.Module, model.Label, model.Description, model.Table); err != nil {
		return fmt.Errorf("upsert model: %w", err)
	}

	// Bump version
	version, err := s.IncrementModelVersion(model.Name)
	if err != nil {
		return fmt.Errorf("increment version: %w", err)
	}

	if err := s.CreateVersion(model.Name, version, marshalModelSnapshot(model), changeSummary, createdBy); err != nil {
		return fmt.Errorf("create version: %w", err)
	}

	// Save fields
	if err := s.SaveFields(model.Name, version, model.Fields); err != nil {
		return fmt.Errorf("save fields: %w", err)
	}

	// Save relations
	if err := s.SaveRelations(model.Name, version, model.Relations); err != nil {
		return fmt.Errorf("save relations: %w", err)
	}

	// Save admin
	if err := s.SaveAdmin(model.Name, version, model.Admin); err != nil {
		return fmt.Errorf("save admin: %w", err)
	}

	return nil
}

// UpdateVersionYAML updates the YAML snapshot for a specific version.
func (s *Store) UpdateVersionYAML(modelID string, version string, yaml string) error {
	_, err := s.DB.Exec(`
		UPDATE model_versions SET yaml_snapshot = $1 WHERE model_id = $2 AND version = $3
	`, yaml, modelID, version)
	return err
}

// ============================================================================
// Helpers
// ============================================================================

func mustMarshalJSON(v any) string {
	if v == nil {
		return "[]"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	s := string(data)
	if s == "null" {
		return "[]"
	}
	return s
}

func marshalModelSnapshot(model *mozi.ModelIR) string {
	data, err := yaml.Marshal(model)
	if err != nil {
		return ""
	}
	return string(data)
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
