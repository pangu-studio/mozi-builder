package release

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// OperationRow is a release_operations row as loaded for execution.
type OperationRow struct {
	ID        string
	ProjectID string
	Resource  string
	Kind      string
	Desired   json.RawMessage
	State     State
	Attempts  int
}

// Controller claims and executes release operations with lease-based crash
// recovery. Adapter writes are idempotent; execution IDs and business
// completion are separate concerns.
type Controller struct {
	DB       *sql.DB
	Etcd     EtcdAdapter
	Apisix   ApisixAdapter
	Now      func() time.Time
	LeaseTTL time.Duration
}

func (c *Controller) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Controller) leaseTTL() time.Duration {
	if c.LeaseTTL > 0 {
		return c.LeaseTTL
	}
	return 5 * time.Minute
}

// ExpireStale returns Applying operations whose claim lease has expired back
// to Pending so another controller can resume them.
func (c *Controller) ExpireStale(ctx context.Context) (int, error) {
	res, err := c.DB.ExecContext(ctx, `UPDATE release_operations SET state=$1,updated_at=now() WHERE state=$2 AND updated_at<$3`, Pending, Applying, c.now().Add(-c.leaseTTL()))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ClaimNext atomically claims the oldest Pending operation. SKIP LOCKED lets
// multiple controllers work without double-claiming.
func (c *Controller) ClaimNext(ctx context.Context) (*OperationRow, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var op OperationRow
	err = tx.QueryRowContext(ctx, `SELECT id,project_id,resource,kind,desired,state,attempts FROM release_operations WHERE state=$1 ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, Pending).
		Scan(&op.ID, &op.ProjectID, &op.Resource, &op.Kind, &op.Desired, &op.State, &op.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = c.transition(ctx, tx, &op, EventClaim, ""); err != nil {
		return nil, err
	}
	return &op, tx.Commit()
}

// RunOnce claims one operation, executes it through the adapters, verifies
// with a read-back, and records the outcome. Returns false when no operation
// is pending.
func (c *Controller) RunOnce(ctx context.Context) (bool, error) {
	op, err := c.ClaimNext(ctx)
	if err != nil || op == nil {
		return op != nil, err
	}
	desired, err := ParseDesired(op.Kind, op.Desired)
	if err != nil {
		return true, c.finish(ctx, op, EventFatal, Observed{}, err)
	}
	if err = Execute(ctx, op.Kind, desired, c.Etcd, c.Apisix); err != nil {
		return true, c.finish(ctx, op, EventFatal, Observed{}, err)
	}
	observed, err := ReadBack(ctx, desired, c.Etcd, c.Apisix)
	if err != nil {
		// Registry unreachable: retry, never treat as removal.
		return true, c.finish(ctx, op, EventReadbackMismatch, Observed{}, nil)
	}
	if desired.Matches(observed) {
		return true, c.finish(ctx, op, EventReadbackMatch, observed, nil)
	}
	return true, c.finish(ctx, op, EventReadbackMismatch, observed, nil)
}

// DriftCheck transitions a Ready operation to Drifted when the observed state
// no longer matches the desired state. Platform-external changes are only
// detected, never reverted automatically.
func (c *Controller) DriftCheck(ctx context.Context, op *OperationRow) error {
	desired, err := ParseDesired(op.Kind, op.Desired)
	if err != nil {
		return err
	}
	observed, err := ReadBack(ctx, desired, c.Etcd, c.Apisix)
	if err != nil {
		return nil // unreachable registry is not drift
	}
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = c.recordReadback(ctx, tx, op.ID, observed, desired.Matches(observed)); err != nil {
		return err
	}
	if !desired.Matches(observed) {
		if err = c.transition(ctx, tx, op, EventReadbackMismatch, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Controller) finish(ctx context.Context, op *OperationRow, event Event, observed Observed, cause error) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if observed.RPCNodes != nil || observed.HTTPNodes != nil || observed.RouteLive {
		if err = c.recordReadback(ctx, tx, op.ID, observed, event == EventReadbackMatch); err != nil {
			return err
		}
	}
	lastErr := ""
	if cause != nil {
		lastErr = cause.Error()
	}
	if err = c.transition(ctx, tx, op, event, lastErr); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Controller) transition(ctx context.Context, tx *sql.Tx, op *OperationRow, event Event, lastErr string) error {
	next, err := Transition(Operation{State: op.State, Attempts: op.Attempts}, event)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE release_operations SET state=$2,attempts=$3,last_error=$4,updated_at=now() WHERE id=$1 AND state=$5`, op.ID, next.State, next.Attempts, lastErr, op.State)
	if err != nil {
		return err
	}
	op.State, op.Attempts = next.State, next.Attempts
	return nil
}

func (c *Controller) recordReadback(ctx context.Context, tx *sql.Tx, operationID string, observed Observed, match bool) error {
	raw, err := json.Marshal(observed)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO release_readbacks(id,operation_id,observed,match) VALUES($1,$2,$3,$4)`, rand.Text(), operationID, raw, match)
	return err
}

// CreateOperation inserts a Pending operation. A duplicate idempotency key
// returns the existing operation ID without re-executing.
func CreateOperation(ctx context.Context, db *sql.DB, op OperationRow, idempotencyKey, actorID string) (string, bool, error) {
	var id string
	err := db.QueryRowContext(ctx, `INSERT INTO release_operations(id,project_id,resource,kind,desired,idempotency_key,state,actor_id,request_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(idempotency_key) DO NOTHING RETURNING id`,
		op.ID, op.ProjectID, op.Resource, op.Kind, op.Desired, idempotencyKey, Pending, actorID, rand.Text()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		if e := db.QueryRowContext(ctx, `SELECT id FROM release_operations WHERE idempotency_key=$1`, idempotencyKey).Scan(&id); e != nil {
			return "", false, e
		}
		return id, false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
