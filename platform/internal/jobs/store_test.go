package jobs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/migrate"
)

func isolatedPlatformDB(t *testing.T) *sql.DB {
	t.Helper()
	env := os.Getenv("MOZI_INTEGRATION_ENV")
	if env == "" {
		t.Skip("MOZI_INTEGRATION_ENV required")
	}
	if err := config.LoadEnvFile(env); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadDatabases(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	pgxCfg, err := pgx.ParseConfig(cfg.Platform)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*pgxCfg)
	schema := "test_jobs_" + strings.ToLower(rand.Text())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(`CREATE SCHEMA ` + quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	pgxCfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*pgxCfg)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec(`DROP SCHEMA ` + quoted + ` CASCADE`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if _, err = migrate.Apply(context.Background(), db, "platform"); err != nil {
		t.Fatal(err)
	}
	return db
}

func createProject(t *testing.T, db *sql.DB) string {
	t.Helper()
	var userID string
	if err := db.QueryRow(`INSERT INTO users(id,email,display_name,password_hash) VALUES($1,$2,$3,$4) RETURNING id`, rand.Text(), strings.ToLower(rand.Text())+"@test.local", "tester", "x").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`INSERT INTO projects(id,slug,name,created_by) VALUES($1,$2,$3,$4) RETURNING id`, rand.Text(), "job-"+strings.ToLower(rand.Text())[:8], "Jobs", userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTriggerIndependenceAndCompletionIdempotency(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()

	// Every trigger gets an independent execution_id, attempt 1.
	e1, err := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	if e1.ExecutionID == e2.ExecutionID || e1.Attempt != 1 || e2.Attempt != 1 {
		t.Fatalf("triggers must be independent: %v %v", e1, e2)
	}

	// Completion is idempotent per (execution_id, attempt).
	if err = s.Complete(ctx, e1.ExecutionID, 1, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.Complete(ctx, e1.ExecutionID, 1, true, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate completion must be rejected: %v", err)
	}
	got, err := s.Get(ctx, e1.ExecutionID)
	if err != nil || got.State != Succeeded {
		t.Fatalf("state: %v %v", got.State, err)
	}
}

func TestRetrySharesExecutionID(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()

	e, err := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	// Running executions cannot be retried.
	if _, err = s.Retry(ctx, e.ExecutionID, 3); !errors.Is(err, ErrConflict) {
		t.Fatalf("retry while running: %v", err)
	}
	if err = s.Complete(ctx, e.ExecutionID, 1, false, "boom"); err != nil {
		t.Fatal(err)
	}
	// Same trigger retries share execution_id with incrementing attempt.
	r2, err := s.Retry(ctx, e.ExecutionID, 3)
	if err != nil || r2.ExecutionID != e.ExecutionID || r2.Attempt != 2 || r2.State != Running {
		t.Fatalf("retry: %+v %v", r2, err)
	}
	if err = s.Complete(ctx, e.ExecutionID, 2, false, "boom again"); err != nil {
		t.Fatal(err)
	}
	r3, err := s.Retry(ctx, e.ExecutionID, 3)
	if err != nil || r3.Attempt != 3 {
		t.Fatalf("retry 3: %+v %v", r3, err)
	}
	if err = s.Complete(ctx, e.ExecutionID, 3, false, "third"); err != nil {
		t.Fatal(err)
	}
	// Budget exhausted.
	if _, err = s.Retry(ctx, e.ExecutionID, 3); !errors.Is(err, ErrConflict) {
		t.Fatalf("retry budget: %v", err)
	}
}

func TestHeartbeatAndLostSweep(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()

	e, err := s.Trigger(ctx, project, "content", "LongJob", TriggerScheduled)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Heartbeat(ctx, e.ExecutionID, 1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, e.ExecutionID)
	if got.HeartbeatAt == nil {
		t.Fatal("heartbeat not recorded")
	}
	// Age the heartbeat past the grace window.
	if _, err = db.ExecContext(ctx, `UPDATE job_executions SET heartbeat_at=now()-interval '1 hour' WHERE execution_id=$1`, e.ExecutionID); err != nil {
		t.Fatal(err)
	}
	lost, err := s.SweepLost(ctx, 30*time.Second)
	if err != nil || lost != 1 {
		t.Fatalf("sweep: %d %v", lost, err)
	}
	got, _ = s.Get(ctx, e.ExecutionID)
	if got.State != Lost {
		t.Fatalf("state: %s", got.State)
	}
	// Lost executions may be retried like failed ones.
	r, err := s.Retry(ctx, e.ExecutionID, 2)
	if err != nil || r.Attempt != 2 || r.State != Running {
		t.Fatalf("retry from lost: %+v %v", r, err)
	}
}

func TestListRecentFirst(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	s := Store{DB: db}
	ctx := context.Background()
	first, _ := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerScheduled)
	second, _ := s.Trigger(ctx, project, "content", "DeckDigestJob", TriggerManual)
	list, err := s.List(ctx, project, "content", "DeckDigestJob", 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", len(list), err)
	}
	if list[0].ExecutionID != second.ExecutionID || list[1].ExecutionID != first.ExecutionID {
		t.Fatalf("order: %v", list)
	}
}
