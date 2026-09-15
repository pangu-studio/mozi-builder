package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/platform/internal/design"
	"github.com/pangu-studio/mozi-builder/platform/internal/jobs"
	"github.com/zeromicro/go-zero/rest"
)

func (a API) jobRoutes() []rest.Route {
	return []rest.Route{
		{Method: "POST", Path: "/api/v2/dkron/fire", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/jobs/heartbeat", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects/:project/jobs/:module/:job/fire", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/jobs/:module/:job/executions", Handler: a.handle},
	}
}

// fireKeyOK checks the shared secret carried by scheduler callbacks and
// executor heartbeat reports. Business data stays member-authorized; these
// two channels are service-to-service on the internal network. Dkron 4.1.3
// does not deliver custom executor headers (verified against the acceptance
// Dkron), so the scheduler callback also accepts the key as a query
// parameter; heartbeat stays header-only.
func (a API) fireKeyOK(r *http.Request) bool {
	if a.FireKey == "" {
		return false
	}
	if r.Header.Get("X-Mozi-Fire-Key") == a.FireKey {
		return true
	}
	return r.URL.Path == "/api/v2/dkron/fire" && r.URL.Query().Get("fire_key") == a.FireKey
}

func (a API) loadJob(r *http.Request, project, module, name string) (*mozi.JobIR, error) {
	if a.Design == nil {
		return nil, errNotConfigured
	}
	row, err := design.Store{DB: a.Design}.GetJob(r.Context(), project, module, name)
	if err != nil {
		return nil, err
	}
	var job mozi.JobIR
	if err = json.Unmarshal(row.Document, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

var errNotConfigured = errors.New("jobs not configured")

// dispatchAsync fires the attempt in the background; the attempt's own
// timeout governs the executor call.
func (a API) dispatchAsync(e jobs.Execution, j *mozi.JobIR) {
	if a.Dispatcher == nil {
		return
	}
	go func() { _ = a.Dispatcher.Dispatch(context.Background(), e, j) }()
}

// handleDkronFire is the scheduler callback: every Dkron trigger becomes a
// fresh execution with an independent execution_id. Disabled jobs are
// rejected — Dkron must never run-trigger them.
func (a API) handleDkronFire(w http.ResponseWriter, r *http.Request) {
	if !a.fireKeyOK(r) {
		fail(w, 403)
		return
	}
	if a.Jobs == nil {
		fail(w, 503)
		return
	}
	q := r.URL.Query()
	project, module, name := q.Get("project"), q.Get("module"), q.Get("job")
	if project == "" || module == "" || name == "" {
		fail(w, 400)
		return
	}
	job, err := a.loadJob(r, project, module, name)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if !job.IsEnabled() {
		reply(w, 409, map[string]string{"error": "job_disabled"})
		return
	}
	execution, err := a.Jobs.Trigger(r.Context(), project, module, name, jobs.TriggerScheduled)
	if err != nil {
		dbError(w, err)
		return
	}
	a.dispatchAsync(execution, job)
	reply(w, 202, execution)
}

// handleJobHeartbeat records executor liveness for long-running attempts.
func (a API) handleJobHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !a.fireKeyOK(r) {
		fail(w, 403)
		return
	}
	if a.Jobs == nil {
		fail(w, 503)
		return
	}
	executionID := r.Header.Get(jobs.HeaderExecutionID)
	attempt := 0
	if v := r.Header.Get(jobs.HeaderAttempt); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			attempt = n
		}
	}
	if executionID == "" || attempt < 1 {
		fail(w, 400)
		return
	}
	if err := a.Jobs.Heartbeat(r.Context(), executionID, attempt); errors.Is(err, jobs.ErrConflict) {
		reply(w, 409, map[string]string{"error": "attempt_not_running"})
		return
	} else if err != nil {
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}

// handleProjectJob covers manual fire and execution listing for members.
func (a API) handleProjectJob(w http.ResponseWriter, r *http.Request, u User, parts []string) {
	// projects/:id/jobs/:module/:name/(fire|executions)
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "jobs" {
		fail(w, 404)
		return
	}
	project, module, name, action := parts[1], parts[3], parts[4], parts[5]
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
	if a.Jobs == nil {
		fail(w, 503)
		return
	}
	if action == "executions" && r.Method == "GET" {
		list, err := a.Jobs.List(r.Context(), project, module, name, 100)
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, list)
		return
	}
	if action != "fire" || r.Method != "POST" {
		fail(w, 404)
		return
	}
	if role == "viewer" {
		fail(w, 403)
		return
	}
	job, err := a.loadJob(r, project, module, name)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	// Manual execution is ad-hoc and allowed even for disabled jobs.
	execution, err := a.Jobs.Trigger(r.Context(), project, module, name, jobs.TriggerManual)
	if err != nil {
		dbError(w, err)
		return
	}
	a.dispatchAsync(execution, job)
	reply(w, 202, execution)
}

func parsePositiveInt(v string) (int, error) {
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0, errors.New("invalid number")
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 {
		return 0, errors.New("not positive")
	}
	return n, nil
}

