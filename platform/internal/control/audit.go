package control

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/pangu-studio/mozi-builder/platform/internal/auditlog"
	"github.com/zeromicro/go-zero/rest"
)

func (a API) auditRoutes() []rest.Route {
	return []rest.Route{
		{Method: "GET", Path: "/api/v2/projects/:project/audit", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/design-changes", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/executions", Handler: a.handle},
	}
}

// handleAuditSearch serves the three audit search endpoints. All members may
// read; non-members get 404. Cursors are opaque and page newest-first.
func (a API) handleAuditSearch(w http.ResponseWriter, r *http.Request, u User, parts []string, resource string) {
	if len(parts) != 3 || parts[0] != "projects" {
		fail(w, 404)
		return
	}
	project := parts[1]
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
	q := r.URL.Query()
	limit := 0
	if v := q.Get("limit"); v != "" {
		if n, e := parsePositiveInt(v); e == nil {
			limit = n
		}
	}
	cursor := q.Get("cursor")

	switch resource {
	case "audit":
		filter := auditlog.EventFilter{Action: q.Get("action"), Actor: q.Get("actor"), Cursor: cursor, Limit: limit}
		from, err := parseTimeParam(q.Get("from"))
		if err != nil {
			fail(w, 400)
			return
		}
		to, err := parseTimeParam(q.Get("to"))
		if err != nil {
			fail(w, 400)
			return
		}
		filter.From, filter.To = from, to
		items, next, err := auditlog.SearchEvents(r.Context(), a.DB, project, filter)
		if errors.Is(err, auditlog.ErrCursor) {
			fail(w, 400)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "next_cursor": next})
	case "design-changes":
		if a.Design == nil {
			fail(w, 503)
			return
		}
		kind := q.Get("kind")
		if kind != "models" && kind != "services" && kind != "jobs" {
			fail(w, 400)
			return
		}
		items, next, err := auditlog.SearchChanges(r.Context(), a.Design, project, kind, cursor, limit)
		if errors.Is(err, auditlog.ErrCursor) {
			fail(w, 400)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "next_cursor": next})
	case "executions":
		items, next, err := auditlog.SearchExecutions(r.Context(), a.DB, project, auditlog.ExecutionFilter{Job: q.Get("job"), State: q.Get("state"), Cursor: cursor, Limit: limit})
		if errors.Is(err, auditlog.ErrCursor) {
			fail(w, 400)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items, "next_cursor": next})
	default:
		fail(w, 404)
	}
}

func parseTimeParam(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	ts, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}
