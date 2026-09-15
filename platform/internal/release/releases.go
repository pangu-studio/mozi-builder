package release

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/pangu-studio/mozi-builder/platform/internal/design"
)

// Release is one row of the releases table: the frozen triple of design
// version snapshot, code reference, and audit metadata.
type Release struct {
	ID             string                       `json:"id"`
	ProjectID      string                       `json:"project_id"`
	Label          string                       `json:"label"`
	DesignVersions map[string]map[string]string `json:"design_versions"`
	CodeRef        string                       `json:"code_ref"`
	CreatedBy      string                       `json:"created_by"`
	CreatedAt      time.Time                    `json:"created_at"`
}

// EnvironmentRelease is one append-only promotion/rollback record.
type EnvironmentRelease struct {
	EnvironmentID string    `json:"environment_id"`
	ReleaseID     string    `json:"release_id"`
	Action        string    `json:"action"`
	State         string    `json:"state"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}

var ErrReleaseInvalid = errors.New("invalid release input")

// Catalog persists releases in the platform database and snapshots design
// versions from the design database. The two databases never share a
// transaction: the snapshot is a best-effort read committed before the
// release row is written.
type Catalog struct {
	Platform *sql.DB
	Design   *sql.DB
}

// Snapshot reads the current version tokens of all design collections.
func (c Catalog) Snapshot(ctx context.Context, project string) (map[string]map[string]string, error) {
	store := design.Store{DB: c.Design}
	out := map[string]map[string]string{"models": {}, "services": {}, "jobs": {}}
	models, err := store.List(ctx, project)
	if err != nil {
		return nil, err
	}
	for _, m := range models {
		out["models"][m.Module+"/"+m.Name] = m.Version
	}
	services, err := store.ListServices(ctx, project)
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		out["services"][s.Module+"/"+s.Name] = s.Version
	}
	jobs, err := store.ListJobs(ctx, project)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		out["jobs"][j.Module+"/"+j.Name] = j.Version
	}
	return out, nil
}

// Create freezes the design snapshot and writes the release row.
func (c Catalog) Create(ctx context.Context, project, label, codeRef, actor string) (Release, error) {
	if label == "" || codeRef == "" || len(label) > 200 {
		return Release{}, ErrReleaseInvalid
	}
	versions, err := c.Snapshot(ctx, project)
	if err != nil {
		return Release{}, err
	}
	raw, err := json.Marshal(versions)
	if err != nil {
		return Release{}, err
	}
	var r Release
	err = c.Platform.QueryRowContext(ctx, `INSERT INTO releases(id,project_id,label,design_versions,code_ref,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,project_id,label,code_ref,created_by,created_at`,
		rand.Text(), project, label, raw, codeRef, actor).Scan(&r.ID, &r.ProjectID, &r.Label, &r.CodeRef, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return Release{}, err
	}
	r.DesignVersions = versions
	return r, nil
}

// List returns releases of a project, newest first.
func (c Catalog) List(ctx context.Context, project string) ([]Release, error) {
	rows, err := c.Platform.QueryContext(ctx, `SELECT id,project_id,label,design_versions,code_ref,created_by,created_at FROM releases WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Release{}
	for rows.Next() {
		var r Release
		var raw json.RawMessage
		if err = rows.Scan(&r.ID, &r.ProjectID, &r.Label, &raw, &r.CodeRef, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.DesignVersions); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Provenance returns the release with its promotion/rollback history.
func (c Catalog) Provenance(ctx context.Context, project, releaseID string) (Release, []EnvironmentRelease, error) {
	var r Release
	var raw json.RawMessage
	err := c.Platform.QueryRowContext(ctx, `SELECT id,project_id,label,design_versions,code_ref,created_by,created_at FROM releases WHERE project_id=$1 AND id=$2`, project, releaseID).
		Scan(&r.ID, &r.ProjectID, &r.Label, &raw, &r.CodeRef, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return r, nil, err
	}
	if err = json.Unmarshal(raw, &r.DesignVersions); err != nil {
		return r, nil, err
	}
	rows, err := c.Platform.QueryContext(ctx, `SELECT environment_id,release_id,action,state,actor_id,created_at FROM environment_releases WHERE release_id=$1 ORDER BY created_at`, releaseID)
	if err != nil {
		return r, nil, err
	}
	defer rows.Close()
	history := []EnvironmentRelease{}
	for rows.Next() {
		var e EnvironmentRelease
		if err = rows.Scan(&e.EnvironmentID, &e.ReleaseID, &e.Action, &e.State, &e.ActorID, &e.CreatedAt); err != nil {
			return r, nil, err
		}
		history = append(history, e)
	}
	return r, history, rows.Err()
}
