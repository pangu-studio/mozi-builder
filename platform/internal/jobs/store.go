// Package jobs implements the phase-5 business task protocol: every trigger
// gets an independent execution_id, retries of the same trigger share it with
// an incrementing attempt, completion is idempotent per (execution_id,
// attempt), and long-running jobs report heartbeats. See docs/v2/jobs.md.
package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"
)

var (
	// ErrConflict signals an idempotency or state guard rejection: the
	// execution is already finished, the attempt is stale, or the retry
	// budget is exhausted.
	ErrConflict = errors.New("job execution conflict")
	// ErrNotFound is returned for unknown execution IDs.
	ErrNotFound = errors.New("job execution not found")
)

// Trigger kinds persisted on the execution row.
const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
	TriggerRetry     = "retry"
)

// Execution states.
const (
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
	Lost      = "lost"
)

// Execution is one job_executions row.
type Execution struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Module      string     `json:"module"`
	Job         string     `json:"job"`
	ExecutionID string     `json:"execution_id"`
	Trigger     string     `json:"trigger"`
	Attempt     int        `json:"attempt"`
	State       string     `json:"state"`
	Error       string     `json:"error"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	HeartbeatAt *time.Time `json:"heartbeat_at,omitempty"`
}

// Store persists executions in the platform database.
type Store struct{ DB *sql.DB }

func scanExecution(row interface{ Scan(...any) error }) (Execution, error) {
	var e Execution
	err := row.Scan(&e.ID, &e.ProjectID, &e.Module, &e.Job, &e.ExecutionID, &e.Trigger, &e.Attempt, &e.State, &e.Error, &e.StartedAt, &e.FinishedAt, &e.HeartbeatAt)
	return e, err
}

const executionColumns = `id,project_id,module,job,execution_id,trigger,attempt,state,error,started_at,finished_at,heartbeat_at`

// Trigger starts a fresh execution: independent execution_id, attempt 1.
// Disabled jobs may still be triggered manually (ad-hoc execution); the
// scheduling layer never triggers disabled jobs.
func (s Store) Trigger(ctx context.Context, project, module, job, trigger string) (Execution, error) {
	return scanExecution(s.DB.QueryRowContext(ctx, `INSERT INTO job_executions(id,project_id,module,job,execution_id,trigger,attempt,state) VALUES($1,$2,$3,$4,$5,$6,1,$7) RETURNING `+executionColumns,
		rand.Text(), project, module, job, rand.Text(), trigger, Running))
}

// Get loads an execution by its protocol ID.
func (s Store) Get(ctx context.Context, executionID string) (Execution, error) {
	e, err := scanExecution(s.DB.QueryRowContext(ctx, `SELECT `+executionColumns+` FROM job_executions WHERE execution_id=$1`, executionID))
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// Retry re-enters the same execution (shared execution_id) with the attempt
// incremented. Only failed/lost executions within the retry budget qualify.
func (s Store) Retry(ctx context.Context, executionID string, maxAttempts int) (Execution, error) {
	e, err := scanExecution(s.DB.QueryRowContext(ctx, `UPDATE job_executions SET attempt=attempt+1,state=$2,error='',finished_at=NULL,heartbeat_at=NULL,updated_at=now() WHERE execution_id=$1 AND state IN ($3,$4) AND attempt<$5 RETURNING `+executionColumns,
		executionID, Running, Failed, Lost, maxAttempts))
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrConflict
	}
	return e, err
}

// Complete finishes an attempt idempotently: only the running attempt may
// complete, and duplicate completions of the same (execution_id, attempt)
// are rejected instead of taking effect twice.
func (s Store) Complete(ctx context.Context, executionID string, attempt int, success bool, errText string) error {
	state := Succeeded
	if !success {
		state = Failed
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE job_executions SET state=$3,error=$4,finished_at=now(),updated_at=now() WHERE execution_id=$1 AND attempt=$2 AND state=$5`, executionID, attempt, state, errText, Running)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// Heartbeat records liveness for a running attempt.
func (s Store) Heartbeat(ctx context.Context, executionID string, attempt int) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE job_executions SET heartbeat_at=now(),updated_at=now() WHERE execution_id=$1 AND attempt=$2 AND state=$3`, executionID, attempt, Running)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// SweepLost marks running executions whose heartbeat is older than grace as
// lost. A timeout marks the attempt failed; it never terminates the business
// work itself.
func (s Store) SweepLost(ctx context.Context, grace time.Duration) (int, error) {
	res, err := s.DB.ExecContext(ctx, `UPDATE job_executions SET state=$2,finished_at=now(),updated_at=now() WHERE state=$1 AND heartbeat_at IS NOT NULL AND heartbeat_at<now()-$3::interval`, Running, Lost, grace.String())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// List returns recent executions of a job, newest first.
func (s Store) List(ctx context.Context, project, module, job string, limit int) ([]Execution, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+executionColumns+` FROM job_executions WHERE project_id=$1 AND module=$2 AND job=$3 ORDER BY created_at DESC LIMIT $4`, project, module, job, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Execution{}
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
