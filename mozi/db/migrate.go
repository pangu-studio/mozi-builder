package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Migrate creates the design database schema if it doesn't exist,
// and applies incremental migrations to keep the schema up to date.
func Migrate(db *sql.DB) error {
	if err := migrateV1(db); err != nil {
		return err
	}
	if err := migrateV2TimestampVersion(db); err != nil {
		return err
	}
	if err := migrateV3DesignDictionaries(db); err != nil {
		return err
	}
	if err := migrateV4RelationLabels(db); err != nil {
		return err
	}
	if err := migrateV5ModelVersionDiffSummary(db); err != nil {
		return err
	}
	return nil
}

// migrateV1 creates the initial schema.
func migrateV1(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS modules (
		id          TEXT PRIMARY KEY,
		label       TEXT NOT NULL,
		description TEXT DEFAULT '',
		icon        TEXT DEFAULT '',
		api_prefix  TEXT NOT NULL,
		sort_order  INT DEFAULT 0,
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS models (
		id              TEXT PRIMARY KEY,
		module_id       TEXT NOT NULL REFERENCES modules(id),
		label           TEXT NOT NULL,
		description     TEXT DEFAULT '',
		table_name      TEXT NOT NULL,
		current_version INT NOT NULL DEFAULT 1,
		created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS model_versions (
		id             SERIAL PRIMARY KEY,
		model_id       TEXT NOT NULL REFERENCES models(id),
		version        INT NOT NULL,
		yaml_snapshot  TEXT NOT NULL,
		change_summary TEXT DEFAULT '',
		created_by     TEXT DEFAULT '',
		created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(model_id, version)
	);

	CREATE TABLE IF NOT EXISTS model_fields (
		id           SERIAL PRIMARY KEY,
		model_id     TEXT NOT NULL REFERENCES models(id),
		version      INT NOT NULL,
		name         TEXT NOT NULL,
		label        TEXT NOT NULL,
		type         TEXT NOT NULL,
		required     BOOLEAN DEFAULT FALSE,
		unique_flag  BOOLEAN DEFAULT FALSE,
		sensitive    BOOLEAN DEFAULT FALSE,
		default_val  TEXT DEFAULT '',
		enum_values  TEXT DEFAULT '',
		form_type    TEXT DEFAULT 'text',
		searchable   BOOLEAN DEFAULT FALSE,
		listable     BOOLEAN DEFAULT TRUE,
		editable     BOOLEAN DEFAULT TRUE,
		sort_order   INT DEFAULT 0,
		is_primary   BOOLEAN DEFAULT FALSE,
		generated    TEXT DEFAULT '',
		auto_now_add BOOLEAN DEFAULT FALSE,
		auto_now     BOOLEAN DEFAULT FALSE,
		created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS model_relations (
		id            SERIAL PRIMARY KEY,
		model_id      TEXT NOT NULL REFERENCES models(id),
		version       INT NOT NULL,
		name          TEXT NOT NULL,
		label         TEXT DEFAULT '',
		type          TEXT NOT NULL,
		target_module TEXT NOT NULL,
		target_model  TEXT NOT NULL,
		back_ref      TEXT DEFAULT '',
		cascade       BOOLEAN DEFAULT FALSE,
		required      BOOLEAN DEFAULT FALSE,
		unique_flag   BOOLEAN DEFAULT FALSE,
		created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS model_admin (
		id            SERIAL PRIMARY KEY,
		model_id      TEXT NOT NULL REFERENCES models(id),
		version       INT NOT NULL,
		list_columns  TEXT DEFAULT '',
		search_fields TEXT DEFAULT '',
		default_sort  TEXT DEFAULT 'created_at',
		default_order TEXT DEFAULT 'desc',
		page_size     INT DEFAULT 20
	);

	CREATE TABLE IF NOT EXISTS api_endpoint_overrides (
		endpoint_id   TEXT PRIMARY KEY,
		module_id     TEXT DEFAULT '',
		display_name  TEXT DEFAULT '',
		description   TEXT DEFAULT '',
		updated_by    TEXT DEFAULT '',
		updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_model_fields_model_id ON model_fields(model_id);
	CREATE INDEX IF NOT EXISTS idx_model_relations_model_id ON model_relations(model_id);
	CREATE INDEX IF NOT EXISTS idx_model_versions_model_id ON model_versions(model_id);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("execute migration v1: %w", err)
	}
	return nil
}

// migrateV4RelationLabels stores the business predicate shown on ER edges.
func migrateV4RelationLabels(db *sql.DB) error {
	if _, err := db.Exec(`ALTER TABLE model_relations ADD COLUMN IF NOT EXISTS label TEXT DEFAULT ''`); err != nil {
		return fmt.Errorf("add model_relations.label: %w", err)
	}
	return nil
}

// migrateV5ModelVersionDiffSummary adds a JSONB column caching each version's
// diff against its predecessor, so version history can render per-version
// change summaries without recomputing on every read. Rows written before this
// migration have NULL here and are backfilled lazily on first history read.
func migrateV5ModelVersionDiffSummary(db *sql.DB) error {
	if _, err := db.Exec(`ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS diff_summary JSONB`); err != nil {
		return fmt.Errorf("add model_versions.diff_summary: %w", err)
	}
	return nil
}

// migrateV3DesignDictionaries adds per-design-database dictionaries for
// business-specific platform options such as API consumers.
func migrateV3DesignDictionaries(db *sql.DB) error {
	schema := `
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
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("execute migration v3 design dictionaries: %w", err)
	}

	if _, err := db.Exec(`
		INSERT INTO design_dictionaries (id, label, description)
		VALUES ('api_consumers', 'API 调用方', '按当前业务维护 API 意图中的调用方选项')
		ON CONFLICT (id) DO UPDATE SET
			label = EXCLUDED.label,
			description = EXCLUDED.description,
			updated_at = CURRENT_TIMESTAMP
	`); err != nil {
		return fmt.Errorf("seed api_consumers dictionary: %w", err)
	}

	defaultConsumers := []struct {
		value     string
		label     string
		aliases   string
		sortOrder int
	}{
		{"miniapp", "微信小程序", `["小程序","mini_program","wechat_miniapp"]`, 10},
		{"desktop", "桌面端（Tauri）", `["桌面端","桌面端 (Tauri)","tauri"]`, 20},
		{"admin", "管理后台", `["后台","admin_console"]`, 30},
		{"third_party", "第三方集成", `["第三方","partner"]`, 40},
		{"server_job", "服务端任务", `["定时任务"]`, 50},
	}
	for _, item := range defaultConsumers {
		if _, err := db.Exec(`
			INSERT INTO design_dictionary_items (dictionary_id, value, label, aliases, sort_order, enabled)
			VALUES ('api_consumers', $1, $2, $3, $4, TRUE)
			ON CONFLICT (dictionary_id, value) DO NOTHING
		`, item.value, item.label, item.aliases, item.sortOrder); err != nil {
			return fmt.Errorf("seed api_consumers item %s: %w", item.value, err)
		}
	}
	return nil
}

// migrateV2TimestampVersion converts version columns from INT to TEXT (timestamp format YYYYMMDDHHmmss).
func migrateV2TimestampVersion(db *sql.DB) error {
	// Check if migration is needed — if current_version column is already TEXT, skip
	var colType string
	err := db.QueryRow(`
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'models' AND column_name = 'current_version'
	`).Scan(&colType)
	if err != nil {
		// Table might not exist yet — fine
		return nil
	}
	if colType == "text" || colType == "character varying" {
		return nil // already migrated
	}

	// Step 1: Add temporary TEXT columns
	if _, err := db.Exec(`ALTER TABLE models ADD COLUMN IF NOT EXISTS current_version_text TEXT`); err != nil {
		return fmt.Errorf("add models.current_version_text: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_versions ADD COLUMN IF NOT EXISTS version_text TEXT`); err != nil {
		return fmt.Errorf("add model_versions.version_text: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_fields ADD COLUMN IF NOT EXISTS version_text TEXT`); err != nil {
		return fmt.Errorf("add model_fields.version_text: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_relations ADD COLUMN IF NOT EXISTS version_text TEXT`); err != nil {
		return fmt.Errorf("add model_relations.version_text: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_admin ADD COLUMN IF NOT EXISTS version_text TEXT`); err != nil {
		return fmt.Errorf("add model_admin.version_text: %w", err)
	}

	// Step 2: Convert existing version data to timestamp strings
	// For model_versions, use the created_at timestamp to generate version strings
	rows, err := db.Query(`
		SELECT mv.id, mv.created_at FROM model_versions mv ORDER BY mv.model_id, mv.version
	`)
	if err != nil {
		return fmt.Errorf("query model_versions for migration: %w", err)
	}
	defer rows.Close()

	type verUpdate struct {
		id      int
		verText string
	}
	var updates []verUpdate
	// Track per-second counts to handle collisions
	seen := make(map[string]int)

	for rows.Next() {
		var id int
		var createdAt interface{}
		if err := rows.Scan(&id, &createdAt); err != nil {
			return fmt.Errorf("scan model_versions row: %w", err)
		}
		ts := timestampFromCreatedAt(createdAt)
		seen[ts]++
		if seen[ts] > 1 {
			ts = fmt.Sprintf("%s_%d", ts, seen[ts])
		}
		updates = append(updates, verUpdate{id: id, verText: ts})
	}
	rows.Close()

	for _, u := range updates {
		if _, err := db.Exec(`UPDATE model_versions SET version_text = $1 WHERE id = $2`, u.verText, u.id); err != nil {
			return fmt.Errorf("update model_versions.version_text: %w", err)
		}
	}

	// Sync version_text to related tables based on model_versions join
	if _, err := db.Exec(`
		UPDATE model_fields mf SET version_text = mv.version_text
		FROM model_versions mv
		WHERE mf.model_id = mv.model_id AND mf.version = mv.version
	`); err != nil {
		return fmt.Errorf("sync model_fields.version_text: %w", err)
	}
	if _, err := db.Exec(`
		UPDATE model_relations mr SET version_text = mv.version_text
		FROM model_versions mv
		WHERE mr.model_id = mv.model_id AND mr.version = mv.version
	`); err != nil {
		return fmt.Errorf("sync model_relations.version_text: %w", err)
	}
	if _, err := db.Exec(`
		UPDATE model_admin ma SET version_text = mv.version_text
		FROM model_versions mv
		WHERE ma.model_id = mv.model_id AND ma.version = mv.version
	`); err != nil {
		return fmt.Errorf("sync model_admin.version_text: %w", err)
	}

	// Update models.current_version_text to the latest version for each model
	if _, err := db.Exec(`
		UPDATE models m SET current_version_text = (
			SELECT version_text FROM model_versions mv
			WHERE mv.model_id = m.id
			ORDER BY mv.id DESC LIMIT 1
		)
	`); err != nil {
		return fmt.Errorf("sync models.current_version_text: %w", err)
	}

	// Set default for any models without versions
	if _, err := db.Exec(`
		UPDATE models SET current_version_text = '1' WHERE current_version_text IS NULL
	`); err != nil {
		return fmt.Errorf("default models.current_version_text: %w", err)
	}

	// Step 3: Drop old INT columns and rename TEXT columns
	if _, err := db.Exec(`ALTER TABLE models DROP COLUMN current_version`); err != nil {
		return fmt.Errorf("drop models.current_version: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE models RENAME COLUMN current_version_text TO current_version`); err != nil {
		return fmt.Errorf("rename models.current_version_text: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE model_versions DROP COLUMN version`); err != nil {
		return fmt.Errorf("drop model_versions.version: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_versions RENAME COLUMN version_text TO version`); err != nil {
		return fmt.Errorf("rename model_versions.version_text: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE model_fields DROP COLUMN version`); err != nil {
		return fmt.Errorf("drop model_fields.version: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_fields RENAME COLUMN version_text TO version`); err != nil {
		return fmt.Errorf("rename model_fields.version_text: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE model_relations DROP COLUMN version`); err != nil {
		return fmt.Errorf("drop model_relations.version: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_relations RENAME COLUMN version_text TO version`); err != nil {
		return fmt.Errorf("rename model_relations.version_text: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE model_admin DROP COLUMN version`); err != nil {
		return fmt.Errorf("drop model_admin.version: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE model_admin RENAME COLUMN version_text TO version`); err != nil {
		return fmt.Errorf("rename model_admin.version_text: %w", err)
	}

	// Recreate unique constraint on model_versions
	if _, err := db.Exec(`
		ALTER TABLE model_versions ADD CONSTRAINT model_versions_model_id_version_key UNIQUE (model_id, version)
	`); err != nil {
		// Constraint might already exist — ignore
		_ = err
	}

	return nil
}

// timestampFromCreatedAt converts a created_at value to YYYYMMDDHHmmss format.
func timestampFromCreatedAt(createdAt interface{}) string {
	switch t := createdAt.(type) {
	case time.Time:
		return t.Format("20060102150405")
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			parsed, err = time.Parse("2006-01-02T15:04:05Z", t)
			if err != nil {
				return t
			}
		}
		return parsed.Format("20060102150405")
	default:
		return fmt.Sprintf("%v", createdAt)
	}
}
