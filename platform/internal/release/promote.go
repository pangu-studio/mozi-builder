package release

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/platform/internal/jobs"
)

var (
	// ErrConfirmRequired rejects promotion into a protected environment
	// without explicit confirmation.
	ErrConfirmRequired = errors.New("protected environment requires confirmation")
	// ErrPromotionNotFound signals a missing environment or release.
	ErrPromotionNotFound = errors.New("environment or release not found")
	// ErrNoRollbackTarget signals no earlier ready release to roll back to.
	ErrNoRollbackTarget = errors.New("no earlier ready release")
)

// Promoter unfolds environment promotions and rollbacks into the fixed
// sequence: task synchronization (Dkron) from the frozen snapshot, then the
// state transition. Deploy/route steps run through the phase-4 operation
// pipeline where configured; the sequence fails into a resumable middle
// state. Database rollback is always a separate concern.
type Promoter struct {
	Platform *sql.DB
	Design   *sql.DB
	Dkron    *jobs.DkronClient
	FireURL  string // platform fire endpoint template base (project/module/job appended)
	FireKey  string
}

// Promote applies a release to an environment. Protected environments
// require confirm=true. On success the previously ready record is superseded.
func (p Promoter) Promote(ctx context.Context, environmentID, releaseID, actor string, confirm bool) (EnvironmentRelease, error) {
	return p.apply(ctx, environmentID, releaseID, "promote", actor, confirm)
}

// Rollback re-applies the previous ready release of the environment.
func (p Promoter) Rollback(ctx context.Context, environmentID, actor string, confirm bool) (EnvironmentRelease, error) {
	target, err := p.previousReady(ctx, environmentID)
	if err != nil {
		return EnvironmentRelease{}, err
	}
	return p.apply(ctx, environmentID, target, "rollback", actor, confirm)
}

func (p Promoter) apply(ctx context.Context, environmentID, releaseID, action, actor string, confirm bool) (EnvironmentRelease, error) {
	protected, project, err := p.environment(ctx, environmentID)
	if errors.Is(err, sql.ErrNoRows) {
		return EnvironmentRelease{}, ErrPromotionNotFound
	}
	if err != nil {
		return EnvironmentRelease{}, err
	}
	if protected && !confirm {
		return EnvironmentRelease{}, ErrConfirmRequired
	}
	rel, _, err := Catalog{Platform: p.Platform, Design: p.Design}.Provenance(ctx, project, releaseID)
	if errors.Is(err, sql.ErrNoRows) {
		return EnvironmentRelease{}, ErrPromotionNotFound
	}
	if err != nil {
		return EnvironmentRelease{}, err
	}

	record, err := p.insert(ctx, environmentID, releaseID, action, "applying", actor)
	if err != nil {
		return record, err
	}
	// Task synchronization: Dkron jobs come from the frozen snapshot, loaded
	// from design history at the snapshot tokens.
	if p.Dkron != nil {
		if err = p.syncTasks(ctx, project, rel); err != nil {
			_ = p.transition(ctx, environmentID, releaseID, action, record.CreatedAt, "failed")
			return record, fmt.Errorf("sync tasks: %w", err)
		}
	}
	if err = p.supersedePrevious(ctx, environmentID, record.CreatedAt); err != nil {
		_ = p.transition(ctx, environmentID, releaseID, action, record.CreatedAt, "failed")
		return record, err
	}
	err = p.transition(ctx, environmentID, releaseID, action, record.CreatedAt, "ready")
	record.State = "ready"
	return record, err
}

func (p Promoter) environment(ctx context.Context, id string) (protected bool, project string, err error) {
	err = p.Platform.QueryRowContext(ctx, `SELECT protected,project_id FROM environments WHERE id=$1`, id).Scan(&protected, &project)
	return
}

func (p Promoter) insert(ctx context.Context, environmentID, releaseID, action, state, actor string) (EnvironmentRelease, error) {
	var e EnvironmentRelease
	err := p.Platform.QueryRowContext(ctx, `INSERT INTO environment_releases(environment_id,release_id,action,state,actor_id) VALUES($1,$2,$3,$4,$5) RETURNING environment_id,release_id,action,state,actor_id,created_at`,
		environmentID, releaseID, action, state, actor).Scan(&e.EnvironmentID, &e.ReleaseID, &e.Action, &e.State, &e.ActorID, &e.CreatedAt)
	return e, err
}

func (p Promoter) transition(ctx context.Context, environmentID, releaseID, action string, createdAt any, state string) error {
	_, err := p.Platform.ExecContext(ctx, `UPDATE environment_releases SET state=$5 WHERE environment_id=$1 AND release_id=$2 AND action=$3 AND created_at=$4`, environmentID, releaseID, action, createdAt, state)
	return err
}

func (p Promoter) supersedePrevious(ctx context.Context, environmentID string, before any) error {
	_, err := p.Platform.ExecContext(ctx, `UPDATE environment_releases SET state='superseded' WHERE environment_id=$1 AND state='ready' AND created_at<$2`, environmentID, before)
	return err
}

// previousReady returns the latest ready release before the current one.
func (p Promoter) previousReady(ctx context.Context, environmentID string) (string, error) {
	var id string
	err := p.Platform.QueryRowContext(ctx, `SELECT release_id FROM environment_releases WHERE environment_id=$1 AND state IN ('ready','superseded') ORDER BY created_at DESC LIMIT 1 OFFSET 1`, environmentID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoRollbackTarget
	}
	return id, err
}

// syncTasks loads each snapshot job from design history at its frozen token
// and syncs it to Dkron.
func (p Promoter) syncTasks(ctx context.Context, project string, rel Release) error {
	for key, token := range rel.DesignVersions["jobs"] {
		module, name := splitRef(key)
		var raw json.RawMessage
		err := p.Design.QueryRowContext(ctx, `SELECT document FROM design_job_history WHERE project_id=$1 AND module=$2 AND name=$3 AND version=$4`, project, module, name, token).Scan(&raw)
		if err != nil {
			return fmt.Errorf("load job %s at %s: %w", key, token, err)
		}
		var job mozi.JobIR
		if err = json.Unmarshal(raw, &job); err != nil {
			return fmt.Errorf("parse job %s: %w", key, err)
		}
		fireURL := fmt.Sprintf("%s?project=%s&module=%s&job=%s", p.FireURL, project, module, name)
		if err = p.Dkron.SyncJob(ctx, &job, fireURL, p.FireKey); err != nil {
			return fmt.Errorf("sync job %s: %w", key, err)
		}
	}
	return nil
}

func splitRef(key string) (module, name string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
