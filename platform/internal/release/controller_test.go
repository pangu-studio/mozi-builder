package release

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
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
	schema := "test_rel_" + strings.ToLower(rand.Text())
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
	if err := db.QueryRow(`INSERT INTO projects(id,slug,name,created_by) VALUES($1,$2,$3,$4) RETURNING id`, rand.Text(), "rel-"+strings.ToLower(rand.Text())[:8], "Rel", userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func deployOp(project string) OperationRow {
	desired, _ := json.Marshal(Desired{
		RPCService: "content.rpc", RPCNodes: []string{"10.0.0.1:9000"},
		HTTPRoute: "content-api", HTTPURI: "/content/*", HTTPNodes: map[string]int{"10.0.0.1:8080": 1},
	})
	return OperationRow{ID: rand.Text(), ProjectID: project, Resource: "content/ContentService", Kind: "deploy", Desired: desired}
}

func opState(t *testing.T, db *sql.DB, id string) (State, int) {
	t.Helper()
	var s State
	var a int
	if err := db.QueryRow(`SELECT state,attempts FROM release_operations WHERE id=$1`, id).Scan(&s, &a); err != nil {
		t.Fatal(err)
	}
	return s, a
}

func TestRunOnceReadyAndReadback(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	fake := newFakeBackends()
	c := &Controller{DB: db, Etcd: fake, Apisix: fake}

	op := deployOp(project)
	id, created, err := CreateOperation(context.Background(), db, op, "key-ready", "tester")
	if err != nil || !created {
		t.Fatalf("create: %v %v", id, err)
	}
	// Duplicate idempotency key returns the same operation without a new row.
	dup, dupCreated, err := CreateOperation(context.Background(), db, op, "key-ready", "tester")
	if err != nil || dupCreated || dup != id {
		t.Fatalf("idempotent create: %s %v %v", dup, dupCreated, err)
	}

	worked, err := c.RunOnce(context.Background())
	if err != nil || !worked {
		t.Fatalf("run once: %v %v", worked, err)
	}
	state, _ := opState(t, db, id)
	if state != Ready {
		t.Fatalf("state: %s", state)
	}
	var matches int
	if err = db.QueryRow(`SELECT count(*) FROM release_readbacks WHERE operation_id=$1 AND match`, id).Scan(&matches); err != nil || matches != 1 {
		t.Fatalf("readbacks: %d %v", matches, err)
	}
	// Nothing left to claim.
	worked, err = c.RunOnce(context.Background())
	if err != nil || worked {
		t.Fatalf("expected no work: %v %v", worked, err)
	}
}

func TestMismatchRetriesToFailedWithExpiryRecovery(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	fake := newFakeBackends()
	c := &Controller{DB: db, Etcd: fake, Apisix: fake, LeaseTTL: time.Millisecond}

	op := deployOp(project)
	id, _, err := CreateOperation(context.Background(), db, op, "key-fail", "tester")
	if err != nil {
		t.Fatal(err)
	}
	// Read-back never matches: the registry reports different nodes than
	// what the controller just wrote.
	fake.diverge = true

	for i := 1; i <= MaxAttempts; i++ {
		worked, err := c.RunOnce(context.Background())
		if err != nil || !worked {
			t.Fatalf("round %d: %v %v", i, worked, err)
		}
		state, attempts := opState(t, db, id)
		if i == MaxAttempts {
			if state != Failed || attempts != MaxAttempts {
				t.Fatalf("final: %s %d", state, attempts)
			}
			break
		}
		if state != Applying || attempts != i {
			t.Fatalf("round %d: %s %d", i, state, attempts)
		}
		// Lease expires instantly: the operation is reclaimed from scratch.
		time.Sleep(2 * time.Millisecond)
		expired, err := c.ExpireStale(context.Background())
		if err != nil || expired != 1 {
			t.Fatalf("expire round %d: %d %v", i, expired, err)
		}
	}
}

func TestExecuteFailureIsFatal(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	fake := newFakeBackends()
	fake.execErr = errFakeBoom
	c := &Controller{DB: db, Etcd: fake, Apisix: fake}

	op := deployOp(project)
	id, _, err := CreateOperation(context.Background(), db, op, "key-fatal", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, _ := opState(t, db, id)
	if state != Failed {
		t.Fatalf("state: %s", state)
	}
}

func TestDisableOperation(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	fake := newFakeBackends()
	c := &Controller{DB: db, Etcd: fake, Apisix: fake}
	fake.live["content-api"] = true

	desired, _ := json.Marshal(Desired{HTTPRoute: "content-api", HTTPURI: "/content/*", Disabled: true})
	op := OperationRow{ID: rand.Text(), ProjectID: project, Resource: "content/ContentService", Kind: "disable", Desired: desired}
	id, _, err := CreateOperation(context.Background(), db, op, "key-disable", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, _ := opState(t, db, id)
	if state != Ready {
		t.Fatalf("state: %s", state)
	}
	if fake.live["content-api"] {
		t.Fatal("route still live")
	}
}

func TestDriftCheckMarksDrifted(t *testing.T) {
	db := isolatedPlatformDB(t)
	project := createProject(t, db)
	fake := newFakeBackends()
	c := &Controller{DB: db, Etcd: fake, Apisix: fake}

	op := deployOp(project)
	id, _, err := CreateOperation(context.Background(), db, op, "key-drift", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Platform-external change: someone removes a node behind our back.
	fake.httpNodes["content-api"] = map[string]int{}
	row := &OperationRow{ID: id, Kind: op.Kind, Desired: op.Desired, State: Ready}
	if err = c.DriftCheck(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	state, _ := opState(t, db, id)
	if state != Drifted {
		t.Fatalf("state: %s", state)
	}
}

var errFakeBoom = errors.New("boom")
