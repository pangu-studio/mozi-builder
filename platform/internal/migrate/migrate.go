package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed design/*.sql platform/*.sql
var migrations embed.FS

type Applied struct {
	Version  int
	Name     string
	Checksum string
}

func Apply(ctx context.Context, db *sql.DB, target string) ([]Applied, error) {
	if target != "design" && target != "platform" {
		return nil, fmt.Errorf("unknown migration target %q", target)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, "mozi-v2-"+target+"-migrations"); err != nil {
		return nil, fmt.Errorf("lock migrations: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(hashtext($1))`, "mozi-v2-"+target+"-migrations")
	}()

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        checksum TEXT NOT NULL,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )`); err != nil {
		return nil, fmt.Errorf("create migration ledger: %w", err)
	}

	entries, err := load(target)
	if err != nil {
		return nil, err
	}
	current, err := applied(ctx, conn)
	if err != nil {
		return nil, err
	}
	if err := validateApplied(entries, current, false); err != nil {
		return nil, err
	}
	var result []Applied
	for _, migration := range entries {
		if prior, ok := current[migration.Version]; ok {
			if prior.Name != migration.Name || prior.Checksum != migration.Checksum {
				return nil, fmt.Errorf("migration %04d changed after application", migration.Version)
			}
			continue
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin migration %04d: %w", migration.Version, err)
		}
		if _, err = tx.ExecContext(ctx, migration.SQL); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, checksum) VALUES ($1, $2, $3)`, migration.Version, migration.Name, migration.Checksum)
		}
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return nil, fmt.Errorf("apply migration %04d_%s: %w", migration.Version, migration.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit migration %04d: %w", migration.Version, err)
		}
		result = append(result, Applied{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
	}
	return result, nil
}

func Verify(ctx context.Context, db *sql.DB, target string) error {
	entries, err := load(target)
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()
	got := map[int]Applied{}
	for rows.Next() {
		var row Applied
		if err := rows.Scan(&row.Version, &row.Name, &row.Checksum); err != nil {
			return err
		}
		got[row.Version] = row
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return validateApplied(entries, got, true)
}

func validateApplied(entries []entry, got map[int]Applied, requireAll bool) error {
	expectedVersions := make(map[int]entry, len(entries))
	for _, expected := range entries {
		expectedVersions[expected.Version] = expected
		row, ok := got[expected.Version]
		if !ok {
			if requireAll {
				return fmt.Errorf("database requires migration %04d_%s", expected.Version, expected.Name)
			}
			continue
		}
		if row.Name != expected.Name || row.Checksum != expected.Checksum {
			return fmt.Errorf("database migration %04d does not match the binary", expected.Version)
		}
	}
	for version := range got {
		if _, ok := expectedVersions[version]; !ok {
			return fmt.Errorf("database contains migration %04d unknown to this binary", version)
		}
	}
	return nil
}

type entry struct {
	Applied
	SQL string
}

func load(target string) ([]entry, error) {
	files, err := fs.Glob(migrations, target+"/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var result []entry
	last := 0
	for _, path := range files {
		base := filepath.Base(path)
		parts := strings.SplitN(strings.TrimSuffix(base, ".sql"), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %s", base)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil || version <= last {
			return nil, fmt.Errorf("invalid migration version in %s", base)
		}
		body, err := migrations.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		result = append(result, entry{Applied: Applied{Version: version, Name: parts[1], Checksum: hex.EncodeToString(sum[:])}, SQL: string(body)})
		last = version
	}
	if len(result) == 0 {
		return nil, errors.New("no embedded migrations")
	}
	return result, nil
}

func applied(ctx context.Context, conn *sql.Conn) (map[int]Applied, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()
	result := map[int]Applied{}
	for rows.Next() {
		var row Applied
		if err := rows.Scan(&row.Version, &row.Name, &row.Checksum); err != nil {
			return nil, err
		}
		result[row.Version] = row
	}
	return result, rows.Err()
}
