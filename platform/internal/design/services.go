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
	moziservice "github.com/pangu-studio/mozi-builder/mozi/service"
)

type Service struct {
	Module    string          `json:"module"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Document  json.RawMessage `json:"document"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ValidateService decodes and validates a ServiceIR document. References to
// models are checked for format only; cross-model resolution stays at lint.
func ValidateService(body json.RawMessage) (*mozi.ServiceIR, error) {
	var svc mozi.ServiceIR
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&svc) != nil || d.Decode(&struct{}{}) != io.EOF {
		return nil, ErrInvalid
	}
	if !moduleRE.MatchString(svc.Module) || !nameRE.MatchString(svc.Name) || strings.TrimSpace(svc.Label) == "" || svc.SchemaVersion > mozi.CurrentSchemaVersion {
		return nil, ErrInvalid
	}
	if !moziservice.Validate(&svc).Valid {
		return nil, ErrInvalid
	}
	return &svc, nil
}

func (s Store) ListServices(ctx context.Context, project string) ([]Service, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT module,name,version,document,updated_at FROM design_services WHERE project_id=$1 AND NOT deleted ORDER BY module,name LIMIT 500`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Service{}
	for rows.Next() {
		var v Service
		if err = rows.Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s Store) GetService(ctx context.Context, project, module, name string) (Service, error) {
	var v Service
	err := s.DB.QueryRowContext(ctx, `SELECT module,name,version,document,updated_at FROM design_services WHERE project_id=$1 AND module=$2 AND name=$3 AND NOT deleted`, project, module, name).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	return v, err
}
func (s Store) ServiceHistory(ctx context.Context, project, module, name string) ([]History, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT version,document,action,actor_id,created_at FROM design_service_history WHERE project_id=$1 AND module=$2 AND name=$3 ORDER BY created_at DESC,version LIMIT 100`, project, module, name)
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

// SaveService mirrors Save: compare-and-swap version, project metadata from
// the authorized platform lookup, document and history in one transaction.
func (s Store) SaveService(ctx context.Context, scope Scope, module, name, expected string, body json.RawMessage, action string) (Service, error) {
	if action != "deleted" {
		v, err := ValidateService(body)
		if err != nil {
			return Service{}, err
		}
		if v.Module != module || v.Name != name {
			return Service{}, ErrInvalid
		}
	}
	if action != "created" && expected == "" {
		return Service{}, ErrConflict
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Service{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO design_projects(id,slug,name) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET slug=excluded.slug,name=excluded.name,updated_at=now()`, scope.ID, scope.Slug, scope.Name)
	if err != nil {
		return Service{}, err
	}
	version := rand.Text()
	var v Service
	if action == "created" {
		err = tx.QueryRowContext(ctx, `INSERT INTO design_services(project_id,module,name,version,document) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body)).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else if action == "updated" {
		err = tx.QueryRowContext(ctx, `UPDATE design_services SET version=$4,document=$5,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$6 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body), expected).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else if action == "deleted" {
		err = tx.QueryRowContext(ctx, `UPDATE design_services SET version=$4,deleted=true,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$5 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, expected).Scan(&v.Module, &v.Name, &v.Version, &v.Document, &v.UpdatedAt)
	} else {
		return Service{}, ErrInvalid
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, ErrConflict
	}
	if err != nil {
		return Service{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO design_service_history(project_id,module,name,version,document,action,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.ID, module, name, version, []byte(v.Document), action, scope.Actor, rand.Text())
	if err != nil {
		return Service{}, err
	}
	if err = tx.Commit(); err != nil {
		return Service{}, err
	}
	return v, nil
}
