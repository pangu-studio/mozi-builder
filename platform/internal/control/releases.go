package control

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/pangu-studio/mozi-builder/platform/internal/release"
	"github.com/zeromicro/go-zero/rest"
)

func (a API) releaseRoutes() []rest.Route {
	return []rest.Route{
		{Method: "GET", Path: "/api/v2/projects/:project/releases", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects/:project/releases", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/releases/:release/provenance", Handler: a.handle},
	}
}

// handleReleases covers release creation, listing, and provenance. The
// membership row lock is held through the operation, mirroring the design
// endpoints. GET is open to all members; creation excludes viewer.
func (a API) handleReleases(w http.ResponseWriter, r *http.Request, u User, parts []string) {
	// projects/:id/releases or projects/:id/releases/:rid/provenance
	if len(parts) != 3 && !(len(parts) == 5 && parts[4] == "provenance") {
		fail(w, 404)
		return
	}
	project := parts[1]
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRowContext(r.Context(), `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 FOR SHARE`, project, u.ID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if r.Method == "POST" && role == "viewer" {
		fail(w, 403)
		return
	}
	if a.Design == nil {
		fail(w, 503)
		return
	}
	catalog := release.Catalog{Platform: a.DB, Design: a.Design}

	if r.Method == "GET" && len(parts) == 3 {
		list, err := catalog.List(r.Context(), project)
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, list)
		return
	}
	if r.Method == "GET" {
		item, history, err := catalog.Provenance(r.Context(), project, parts[3])
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, 404)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, map[string]any{"release": item, "environment_releases": history})
		return
	}
	if r.Method != "POST" {
		fail(w, 404)
		return
	}
	var input struct {
		Label   string `json:"label"`
		CodeRef string `json:"code_ref"`
	}
	if !decode(w, r, &input) {
		return
	}
	created, err := catalog.Create(r.Context(), project, input.Label, input.CodeRef, u.ID)
	if errors.Is(err, release.ErrReleaseInvalid) {
		fail(w, 400)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if err = audit(r, tx, u.ID, project, "release.create", created.ID, "succeeded"); err != nil {
		dbError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	reply(w, 201, created)
}
