// Package auditlog provides cursor-paginated search over the three
// append-only audit trails: platform audit_events, design *_history, and
// job_executions. Read-only; retention/archival is an operator concern.
package auditlog

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MaxLimit caps a single page.
const MaxLimit = 100

// ErrCursor marks a malformed pagination cursor.
var ErrCursor = errors.New("invalid cursor")

func encodeCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrCursor
	}
	t, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return time.Time{}, "", ErrCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return time.Time{}, "", ErrCursor
	}
	return ts, id, nil
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

// Event is one audit_events row.
type Event struct {
	ID           int64           `json:"id"`
	OccurredAt   time.Time       `json:"occurred_at"`
	ActorID      *string         `json:"actor_id,omitempty"`
	ProjectID    *string         `json:"project_id,omitempty"`
	RequestID    string          `json:"request_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Result       string          `json:"result"`
	Metadata     json.RawMessage `json:"metadata"`
}

// EventFilter narrows SearchEvents.
type EventFilter struct {
	Action string
	Actor  string
	From   *time.Time
	To     *time.Time
	Cursor string
	Limit  int
}

// SearchEvents returns one page of audit events newest-first plus the cursor
// for the next page (empty when the page is short).
func SearchEvents(ctx context.Context, db *sql.DB, project string, f EventFilter) ([]Event, string, error) {
	limit := clampLimit(f.Limit)
	query := `SELECT id,occurred_at,actor_id,project_id,request_id,action,resource_type,resource_id,result,metadata FROM audit_events WHERE project_id=$1`
	args := []any{project}
	n := 1
	if f.Action != "" {
		n++
		query += fmt.Sprintf(` AND action=$%d`, n)
		args = append(args, f.Action)
	}
	if f.Actor != "" {
		n++
		query += fmt.Sprintf(` AND actor_id=$%d`, n)
		args = append(args, f.Actor)
	}
	if f.From != nil {
		n++
		query += fmt.Sprintf(` AND occurred_at>=$%d`, n)
		args = append(args, *f.From)
	}
	if f.To != nil {
		n++
		query += fmt.Sprintf(` AND occurred_at<=$%d`, n)
		args = append(args, *f.To)
	}
	if f.Cursor != "" {
		ts, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		n++
		query += fmt.Sprintf(` AND (occurred_at,id)<($%d,$%d)`, n, n+1)
		n++
		var numeric int64
		if _, err = fmt.Sscan(id, &numeric); err != nil {
			return nil, "", ErrCursor
		}
		args = append(args, ts, numeric)
	}
	query += fmt.Sprintf(` ORDER BY occurred_at DESC,id DESC LIMIT %d`, limit+1)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.ID, &e.OccurredAt, &e.ActorID, &e.ProjectID, &e.RequestID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.Metadata); err != nil {
			return nil, "", err
		}
		items = append(items, e)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(last.OccurredAt, fmt.Sprint(last.ID))
		items = items[:limit]
	}
	return items, next, nil
}

// Change is one design history row from any of the three collections.
type Change struct {
	Module    string          `json:"module"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Document  json.RawMessage `json:"document"`
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id"`
	CreatedAt time.Time       `json:"created_at"`
}

var changeTables = map[string]string{
	"models":   "design_model_history",
	"services": "design_service_history",
	"jobs":     "design_job_history",
}

// SearchChanges returns one page of design changes for one collection kind.
func SearchChanges(ctx context.Context, db *sql.DB, project, kind, cursor string, limit int) ([]Change, string, error) {
	table, ok := changeTables[kind]
	if !ok {
		return nil, "", fmt.Errorf("unknown design collection %q", kind)
	}
	limit = clampLimit(limit)
	args := []any{project}
	query := `SELECT module,name,version,document,action,actor_id,created_at FROM ` + table + ` WHERE project_id=$1`
	if cursor != "" {
		ts, version, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query += ` AND (created_at,version)<($2,$3)`
		args = append(args, ts, version)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC,version DESC LIMIT %d`, limit+1)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []Change{}
	for rows.Next() {
		var c Change
		if err = rows.Scan(&c.Module, &c.Name, &c.Version, &c.Document, &c.Action, &c.ActorID, &c.CreatedAt); err != nil {
			return nil, "", err
		}
		items = append(items, c)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(last.CreatedAt, last.Version)
		items = items[:limit]
	}
	return items, next, nil
}

// ExecutionRow is the audit view of a job execution.
type ExecutionRow struct {
	ExecutionID string     `json:"execution_id"`
	Module      string     `json:"module"`
	Job         string     `json:"job"`
	Trigger     string     `json:"trigger"`
	Attempt     int        `json:"attempt"`
	State       string     `json:"state"`
	Error       string     `json:"error"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ExecutionFilter narrows SearchExecutions.
type ExecutionFilter struct {
	Job    string
	State  string
	Cursor string
	Limit  int
}

// SearchExecutions returns one page of job executions newest-first.
func SearchExecutions(ctx context.Context, db *sql.DB, project string, f ExecutionFilter) ([]ExecutionRow, string, error) {
	limit := clampLimit(f.Limit)
	query := `SELECT id,execution_id,module,job,trigger,attempt,state,error,started_at,finished_at,created_at FROM job_executions WHERE project_id=$1`
	args := []any{project}
	n := 1
	if f.Job != "" {
		n++
		query += fmt.Sprintf(` AND job=$%d`, n)
		args = append(args, f.Job)
	}
	if f.State != "" {
		n++
		query += fmt.Sprintf(` AND state=$%d`, n)
		args = append(args, f.State)
	}
	if f.Cursor != "" {
		ts, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		n++
		query += fmt.Sprintf(` AND (created_at,id)<($%d,$%d)`, n, n+1)
		n++
		args = append(args, ts, id)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC,id DESC LIMIT %d`, limit+1)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items := []ExecutionRow{}
	ids := []string{}
	for rows.Next() {
		var e ExecutionRow
		var id string
		if err = rows.Scan(&id, &e.ExecutionID, &e.Module, &e.Job, &e.Trigger, &e.Attempt, &e.State, &e.Error, &e.StartedAt, &e.FinishedAt, &e.CreatedAt); err != nil {
			return nil, "", err
		}
		items = append(items, e)
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		// job_executions.id is a random text PK used as the tiebreaker.
		next = encodeCursor(items[limit-1].CreatedAt, ids[limit-1])
		items = items[:limit]
	}
	return items, next, nil
}
