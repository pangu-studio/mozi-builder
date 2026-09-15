package design

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	mozijob "github.com/pangu-studio/mozi-builder/mozi/job"
)

type Job struct {
	Module    string          `json:"module"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Document  json.RawMessage `json:"document"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ValidateJob decodes and validates a JobIR document.
func ValidateJob(body json.RawMessage) (*mozi.JobIR, error) {
	var j mozi.JobIR
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&j) != nil || d.Decode(&struct{}{}) != io.EOF {
		return nil, ErrInvalid
	}
	if !moduleRE.MatchString(j.Module) || !nameRE.MatchString(j.Name) || strings.TrimSpace(j.Label) == "" || j.SchemaVersion > mozi.CurrentSchemaVersion {
		return nil, ErrInvalid
	}
	if !mozijob.Validate(&j).Valid {
		return nil, ErrInvalid
	}
	return &j, nil
}

func (s Store) ListJobs(ctx context.Context, project string) ([]Job, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT module,name,version,document,updated_at FROM design_jobs WHERE project_id=$1 AND NOT deleted ORDER BY module,name LIMIT 500`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Job{}
	for rows.Next() {
		var v Job
		if err = rows.Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s Store) GetJob(ctx context.Context, project, module, name string) (Job, error) {
	var v Job
	err := s.DB.QueryRowContext(ctx, `SELECT module,name,version,document,updated_at FROM design_jobs WHERE project_id=$1 AND module=$2 AND name=$3 AND NOT deleted`, project, module, name).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	return v, err
}
func (s Store) JobHistory(ctx context.Context, project, module, name string) ([]History, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT version,document,action,actor_id,created_at FROM design_job_history WHERE project_id=$1 AND module=$2 AND name=$3 ORDER BY created_at DESC,version LIMIT 100`, project, module, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []History{}
	for rows.Next() {
		var h History
		if err = rows.Scan(&h.Version, &h.Document, &h.Action, &h.ActorID, &h.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// SaveJob mirrors Save/SaveService: compare-and-swap version, project
// metadata from the authorized platform lookup, document and history in one
// transaction.
func (s Store) SaveJob(ctx context.Context, scope Scope, module, name, expected string, body json.RawMessage, action string) (Job, error) {
	if action != "deleted" {
		v, err := ValidateJob(body)
		if err != nil {
			return Job{}, err
		}
		if v.Module != module || v.Name != name {
			return Job{}, ErrInvalid
		}
	}
	if action != "created" && expected == "" {
		return Job{}, ErrConflict
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO design_projects(id,slug,name) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET slug=excluded.slug,name=excluded.name,updated_at=now()`, scope.ID, scope.Slug, scope.Name)
	if err != nil {
		return Job{}, err
	}
	version := rand.Text()
	var v Job
	if action == "created" {
		err = tx.QueryRowContext(ctx, `INSERT INTO design_jobs(project_id,module,name,version,document) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body)).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else if action == "updated" {
		err = tx.QueryRowContext(ctx, `UPDATE design_jobs SET version=$4,document=$5,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$6 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body), expected).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else if action == "deleted" {
		err = tx.QueryRowContext(ctx, `UPDATE design_jobs SET version=$4,deleted=true,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$5 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, expected).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else {
		return Job{}, ErrInvalid
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrConflict
	}
	if err != nil {
		return Job{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO design_job_history(project_id,module,name,version,document,action,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.ID, module, name, version, []byte(v.Document), action, scope.Actor, rand.Text())
	if err != nil {
		return Job{}, err
	}
	if err = tx.Commit(); err != nil {
		return Job{}, err
	}
	return v, nil
}
