package control

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/pangu-studio/mozi-builder/platform/internal/design"
	"github.com/zeromicro/go-zero/rest"
)

func (a API) designRoutes() []rest.Route {
	root := "/api/v2/projects/:project/design/models"
	return []rest.Route{{Method: "GET", Path: root, Handler: a.handle}, {Method: "POST", Path: root, Handler: a.handle}, {Method: "GET", Path: root + "/:module/:name", Handler: a.handle}, {Method: "PUT", Path: root + "/:module/:name", Handler: a.handle}, {Method: "DELETE", Path: root + "/:module/:name", Handler: a.handle}, {Method: "GET", Path: root + "/:module/:name/history", Handler: a.handle}}
}
func (a API) handleDesign(w http.ResponseWriter, r *http.Request, u User, path string) {
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "projects" || parts[2] != "design" || parts[3] != "models" {
		fail(w, 404)
		return
	}
	// Hold the membership row lock through the design operation, so a role change
	// cannot race an authorized write. The databases still have separate commits.
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	var scope design.Scope
	scope.ID = parts[1]
	scope.Actor = u.ID
	var role string
	err = tx.QueryRowContext(r.Context(), `SELECT p.slug,p.name,m.role FROM projects p JOIN project_members m ON m.project_id=p.id WHERE p.id=$1 AND m.user_id=$2 FOR SHARE OF m`, scope.ID, u.ID).Scan(&scope.Slug, &scope.Name, &role)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if !Allowed(role, false, false) || (r.Method != "GET" && role == "viewer") {
		fail(w, 403)
		return
	}
	if a.Design == nil {
		fail(w, 503)
		return
	}
	store := design.Store{DB: a.Design}
	module, name := "", ""
	if len(parts) >= 6 {
		module, name = parts[4], parts[5]
	}
	if r.Method == "GET" {
		var result any
		switch len(parts) {
		case 4:
			result, err = store.List(r.Context(), scope.ID)
		case 6:
			result, err = store.Get(r.Context(), scope.ID, module, name)
		case 7:
			if parts[6] != "history" {
				fail(w, 404)
				return
			}
			result, err = store.History(r.Context(), scope.ID, module, name)
		default:
			fail(w, 404)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, result)
		return
	}
	var input struct {
		Version  string          `json:"version"`
		Document json.RawMessage `json:"document"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		fail(w, 400)
		return
	}
	action := "updated"
	if r.Method == "POST" {
		action = "created"
		m, e := design.Validate(input.Document)
		if e != nil {
			fail(w, 400)
			return
		}
		module, name = m.Module, m.Name
	}
	if r.Method == "DELETE" {
		action = "deleted"
	}
	if action != "created" && input.Version == "" {
		reply(w, 428, map[string]string{"error": "version_required"})
		return
	}
	result, err := store.Save(r.Context(), scope, module, name, input.Version, input.Document, action)
	if errors.Is(err, design.ErrConflict) {
		reply(w, 409, map[string]string{"error": "model_conflict"})
		return
	}
	if errors.Is(err, design.ErrInvalid) {
		reply(w, 400, map[string]string{"error": "invalid_model"})
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	status := 200
	if action == "created" {
		status = 201
	}
	reply(w, status, result)
}
