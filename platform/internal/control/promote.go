package control

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/pangu-studio/mozi-builder/platform/internal/release"
	"github.com/zeromicro/go-zero/rest"
)

func (a API) promotionRoutes() []rest.Route {
	return []rest.Route{
		{Method: "POST", Path: "/api/v2/projects/:project/environments/:environment/promote", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects/:project/environments/:environment/rollback", Handler: a.handle},
	}
}

// handlePromotion covers promote and rollback. Members except viewer may
// promote; protected environments additionally require explicit confirm.
func (a API) handlePromotion(w http.ResponseWriter, r *http.Request, u User, parts []string) {
	// projects/:id/environments/:eid/(promote|rollback)
	if len(parts) != 5 || parts[0] != "projects" || parts[2] != "environments" {
		fail(w, 404)
		return
	}
	project, environmentID, action := parts[1], parts[3], parts[4]
	var role string
	err := a.DB.QueryRowContext(r.Context(), `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2`, project, u.ID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if role == "viewer" {
		fail(w, 403)
		return
	}
	if a.Design == nil {
		fail(w, 503)
		return
	}
	var input struct {
		ReleaseID string `json:"release_id"`
		Confirm   bool   `json:"confirm"`
	}
	if !decode(w, r, &input) {
		return
	}
	promoter := release.Promoter{
		Platform: a.DB,
		Design:   a.Design,
		Dkron:    a.Dkron,
		FireURL:  a.FireURL,
		FireKey:  a.FireKey,
	}
	var record release.EnvironmentRelease
	if action == "promote" {
		if input.ReleaseID == "" {
			fail(w, 400)
			return
		}
		record, err = promoter.Promote(r.Context(), environmentID, input.ReleaseID, u.ID, input.Confirm)
	} else if action == "rollback" {
		record, err = promoter.Rollback(r.Context(), environmentID, u.ID, input.Confirm)
	} else {
		fail(w, 404)
		return
	}
	if errors.Is(err, release.ErrConfirmRequired) {
		reply(w, 409, map[string]string{"error": "confirm_required"})
		return
	}
	if errors.Is(err, release.ErrPromotionNotFound) {
		fail(w, 404)
		return
	}
	if errors.Is(err, release.ErrNoRollbackTarget) {
		reply(w, 409, map[string]string{"error": "no_rollback_target"})
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	reply(w, 200, record)
}
