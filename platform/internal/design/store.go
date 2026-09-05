package design

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/parser"
)

var ErrConflict = errors.New("model version conflict")
var ErrInvalid = errors.New("invalid model")
var moduleRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var nameRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{0,62}$`)
var tableRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Model struct {
	Module    string          `json:"module"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Document  json.RawMessage `json:"document"`
	UpdatedAt time.Time       `json:"updated_at"`
}
type History struct {
	Version   string          `json:"version"`
	Document  json.RawMessage `json:"document"`
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id"`
	CreatedAt time.Time       `json:"created_at"`
}
type Store struct{ DB *sql.DB }
type Scope struct{ ID, Slug, Name, Actor string }

func Validate(body json.RawMessage) (*mozi.ModelIR, error) {
	var model mozi.ModelIR
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&model) != nil || d.Decode(&struct{}{}) != io.EOF {
		return nil, ErrInvalid
	}
	if !moduleRE.MatchString(model.Module) || !nameRE.MatchString(model.Name) || !tableRE.MatchString(model.Table) || strings.TrimSpace(model.Label) == "" || model.SchemaVersion > mozi.CurrentSchemaVersion {
		return nil, ErrInvalid
	}
	if !parser.Validate(&model).Valid {
		return nil, ErrInvalid
	}
	return &model, nil
}
func (s Store) List(ctx context.Context, project string) ([]Model, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT module,name,version,document,updated_at FROM design_models WHERE project_id=$1 AND NOT deleted ORDER BY module,name LIMIT 500`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Model{}
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.Module, &m.Name, &m.Version, &m.Document, &m.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (s Store) Get(ctx context.Context, project, module, name string) (Model, error) {
	var m Model
	err := s.DB.QueryRowContext(ctx, `SELECT module,name,version,document,updated_at FROM design_models WHERE project_id=$1 AND module=$2 AND name=$3 AND NOT deleted`, project, module, name).Scan(&m.Module, &m.Name, &m.Version, &m.Document, &m.UpdatedAt)
	return m, err
}
func (s Store) History(ctx context.Context, project, module, name string) ([]History, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT version,document,action,actor_id,created_at FROM design_model_history WHERE project_id=$1 AND module=$2 AND name=$3 ORDER BY created_at DESC,version LIMIT 100`, project, module, name)
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

// Save uses a compare-and-swap version. No update or delete may omit its version.
// Project metadata comes from an authorized platform lookup, never the request body.
func (s Store) Save(ctx context.Context, scope Scope, module, name, expected string, body json.RawMessage, action string) (Model, error) {
	if action != "deleted" {
		m, err := Validate(body)
		if err != nil {
			return Model{}, err
		}
		if m.Module != module || m.Name != name {
			return Model{}, ErrInvalid
		}
	}
	if action != "created" && expected == "" {
		return Model{}, ErrConflict
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Model{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO design_projects(id,slug,name) VALUES($1,$2,$3) ON CONFLICT(id) DO UPDATE SET slug=excluded.slug,name=excluded.name,updated_at=now()`, scope.ID, scope.Slug, scope.Name)
	if err != nil {
		return Model{}, err
	}
	version := rand.Text()
	var m Model
	if action == "created" {
		err = tx.QueryRowContext(ctx, `INSERT INTO design_models(project_id,module,name,version,document) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body)).Scan(&m.Module, &m.Name, &m.Version, &m.Document, &m.UpdatedAt)
	} else if action == "updated" {
		err = tx.QueryRowContext(ctx, `UPDATE design_models SET version=$4,document=$5,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$6 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, []byte(body), expected).Scan(&m.Module, &m.Name, &m.Version, &m.Document, &m.UpdatedAt)
	} else if action == "deleted" {
		err = tx.QueryRowContext(ctx, `UPDATE design_models SET version=$4,deleted=true,updated_at=now() WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$5 AND NOT deleted RETURNING module,name,version,document,updated_at`, scope.ID, module, name, version, expected).Scan(&m.Module, &m.Name, &m.Version, &m.Document, &m.UpdatedAt)
	} else {
		return Model{}, ErrInvalid
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, ErrConflict
	}
	if err != nil {
		return Model{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO design_model_history(project_id,module,name,version,document,action,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, scope.ID, module, name, version, []byte(m.Document), action, scope.Actor, rand.Text())
	if err != nil {
		return Model{}, err
	}
	if err = tx.Commit(); err != nil {
		return Model{}, err
	}
	return m, nil
}
