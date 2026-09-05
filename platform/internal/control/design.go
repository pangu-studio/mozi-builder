package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/pangu-studio/mozi-builder/platform/internal/design"
	"github.com/zeromicro/go-zero/rest"
)

// designCollection abstracts the models and services design tables behind one
// HTTP contract: project-scoped auth, opaque optimistic versions, append-only
// history, and identical status codes (404/403/428/409/400).
type designCollection struct {
	list    func(ctx context.Context, project string) (any, error)
	get     func(ctx context.Context, project, module, name string) (any, error)
	history func(ctx context.Context, project, module, name string) (any, error)
	save    func(ctx context.Context, scope design.Scope, module, name, expected string, body json.RawMessage, action string) (any, error)
	ident   func(body json.RawMessage) (module, name string, err error)
}

func designCollections(store design.Store) map[string]designCollection {
	return map[string]designCollection{
		"models": {
			list: func(ctx context.Context, p string) (any, error) { return store.List(ctx, p) },
			get:  func(ctx context.Context, p, m, n string) (any, error) { return store.Get(ctx, p, m, n) },
			history: func(ctx context.Context, p, m, n string) (any, error) {
				return store.History(ctx, p, m, n)
			},
			save: func(ctx context.Context, s design.Scope, m, n, e string, b json.RawMessage, a string) (any, error) {
				return store.Save(ctx, s, m, n, e, b, a)
			},
			ident: func(body json.RawMessage) (string, string, error) {
				v, err := design.Validate(body)
				if err != nil {
					return "", "", err
				}
				return v.Module, v.Name, nil
			},
		},
		"services": {
			list: func(ctx context.Context, p string) (any, error) { return store.ListServices(ctx, p) },
			get:  func(ctx context.Context, p, m, n string) (any, error) { return store.GetService(ctx, p, m, n) },
			history: func(ctx context.Context, p, m, n string) (any, error) {
				return store.ServiceHistory(ctx, p, m, n)
			},
			save: func(ctx context.Context, s design.Scope, m, n, e string, b json.RawMessage, a string) (any, error) {
				return store.SaveService(ctx, s, m, n, e, b, a)
			},
			ident: func(body json.RawMessage) (string, string, error) {
				v, err := design.ValidateService(body)
				if err != nil {
					return "", "", err
				}
				return v.Module, v.Name, nil
			},
		},
	}
}

func (a API) designRoutes() []rest.Route {
	var routes []rest.Route
	for _, kind := range []string{"models", "services"} {
		root := "/api/v2/projects/:project/design/" + kind
		routes = append(routes, rest.Route{Method: "GET", Path: root, Handler: a.handle}, rest.Route{Method: "POST", Path: root, Handler: a.handle}, rest.Route{Method: "GET", Path: root + "/:module/:name", Handler: a.handle}, rest.Route{Method: "PUT", Path: root + "/:module/:name", Handler: a.handle}, rest.Route{Method: "DELETE", Path: root + "/:module/:name", Handler: a.handle}, rest.Route{Method: "GET", Path: root + "/:module/:name/history", Handler: a.handle})
	}
	return routes
}
func (a API) handleDesign(w http.ResponseWriter, r *http.Request, u User, path string) {
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "projects" || parts[2] != "design" {
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
	collection, ok := designCollections(design.Store{DB: a.Design})[parts[3]]
	if !ok {
		fail(w, 404)
		return
	}
	module, name := "", ""
	if len(parts) >= 6 {
		module, name = parts[4], parts[5]
	}
	if r.Method == "GET" {
		var result any
		switch len(parts) {
		case 4:
			result, err = collection.list(r.Context(), scope.ID)
		case 6:
			result, err = collection.get(r.Context(), scope.ID, module, name)
		case 7:
			if parts[6] != "history" {
				fail(w, 404)
				return
			}
			result, err = collection.history(r.Context(), scope.ID, module, name)
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
		module, name, err = collection.ident(input.Document)
		if err != nil {
			fail(w, 400)
			return
		}
	}
	if r.Method == "DELETE" {
		action = "deleted"
	}
	if action != "created" && input.Version == "" {
		reply(w, 428, map[string]string{"error": "version_required"})
		return
	}
	result, err := collection.save(r.Context(), scope, module, name, input.Version, input.Document, action)
	if errors.Is(err, design.ErrConflict) {
		reply(w, 409, map[string]string{"error": parts[3][:len(parts[3])-1] + "_conflict"})
		return
	}
	if errors.Is(err, design.ErrInvalid) {
		reply(w, 400, map[string]string{"error": "invalid_" + parts[3][:len(parts[3])-1]})
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
