package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/jobs"
	"github.com/zeromicro/go-zero/rest/router"
)

// TestPromotion covers promote/rollback orchestration: protected
// confirmation, supersede semantics, rollback target selection, and Dkron
// task synchronization from the frozen snapshot.
func TestPromotion(t *testing.T) {
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
	db := isolatedDB(t, cfg.Platform, "platform")
	designDB := isolatedDB(t, cfg.Design, "design")

	var dkronCreates []string
	dkronMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(404)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		dkronCreates = append(dkronCreates, body.Name)
		w.WriteHeader(201)
	}))
	defer dkronMock.Close()
	dkron := &jobs.DkronClient{BaseURL: dkronMock.URL}

	a := API{DB: db, Design: designDB, Dkron: dkron, FireURL: "http://platform/api/v2/dkron/fire", FireKey: "k"}
	mux := router.NewRouter()
	for _, r := range a.Routes() {
		if err = mux.Handle(r.Method, r.Path, http.HandlerFunc(r.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	ctx := t.Context()
	if err = CreateUser(ctx, db, "pro@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	if err = CreateUser(ctx, db, "prov@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "pro@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	tokenV, err := login(ctx, db, "prov@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, tok string, body any, status int) map[string]any {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v2/"+path, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d expected %d: %s", method, path, w.Code, status, w.Body.String())
		}
		result := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	p := call("POST", "projects", token, map[string]string{"slug": "pro-proj", "name": "Pro"}, 201)["id"].(string)
	var viewer string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='prov@test.local'`).Scan(&viewer); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+p+"/members", token, map[string]string{"user_id": viewer, "role": "viewer"}, 201)
	dev := call("POST", "projects/"+p+"/environments", token, map[string]string{"slug": "dev", "name": "开发", "kind": "development"}, 201)["id"].(string)
	prod := call("POST", "projects/"+p+"/environments", token, map[string]string{"slug": "prod", "name": "生产", "kind": "production"}, 201)["id"].(string)

	jobDoc := map[string]any{"schema_version": 1, "module": "content", "job": "DigestJob", "label": "摘要", "schedule": "0 3 * * *", "executor": map[string]any{"kind": "http", "method": "POST", "path": "/jobs/x"}, "timeout_seconds": 60, "retry": map[string]any{"max_attempts": 1, "backoff_seconds": 60}}
	call("POST", "projects/"+p+"/design/jobs", token, map[string]any{"document": jobDoc}, 201)
	r1 := call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r1", "code_ref": "sha-1"}, 201)

	// Promote r1 to dev: ready, and the snapshot job synced to Dkron.
	rec := call("POST", "projects/"+p+"/environments/"+dev+"/promote", token, map[string]any{"release_id": r1["id"]}, 200)
	if rec["state"] != "ready" || rec["action"] != "promote" {
		t.Fatalf("record: %v", rec)
	}
	if len(dkronCreates) != 1 || dkronCreates[0] != "content-digestjob" {
		t.Fatalf("dkron creates: %v", dkronCreates)
	}

	// Protected environment requires confirmation, enforced server-side.
	call("POST", "projects/"+p+"/environments/"+prod+"/promote", token, map[string]any{"release_id": r1["id"]}, 409)
	call("POST", "projects/"+p+"/environments/"+prod+"/promote", token, map[string]any{"release_id": r1["id"], "confirm": true}, 200)

	// Evolve the job, create r2, promote: r1 becomes superseded on dev.
	latest := call("GET", "projects/"+p+"/design/jobs/content/DigestJob", token, nil, 200)
	jobDoc["label"] = "摘要 v2"
	call("PUT", "projects/"+p+"/design/jobs/content/DigestJob", token, map[string]any{"version": latest["version"], "document": jobDoc}, 200)
	r2 := call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r2", "code_ref": "sha-2"}, 201)
	call("POST", "projects/"+p+"/environments/"+dev+"/promote", token, map[string]any{"release_id": r2["id"]}, 200)
	prov := call("GET", "projects/"+p+"/releases/"+r1["id"].(string)+"/provenance", token, nil, 200)
	superseded := false
	for _, e := range prov["environment_releases"].([]any) {
		m := e.(map[string]any)
		if m["environment_id"] == dev && m["state"] == "superseded" {
			superseded = true
		}
	}
	if !superseded {
		t.Fatalf("r1 on dev must be superseded: %v", prov["environment_releases"])
	}

	// Rollback dev: re-applies r1 as action=rollback and re-syncs its job.
	rb := call("POST", "projects/"+p+"/environments/"+dev+"/rollback", token, map[string]any{}, 200)
	if rb["action"] != "rollback" || rb["release_id"] != r1["id"] || rb["state"] != "ready" {
		t.Fatalf("rollback: %v", rb)
	}

	// Production has only one ready release: no rollback target.
	call("POST", "projects/"+p+"/environments/"+prod+"/rollback", token, map[string]any{"confirm": true}, 409)

	// Roles and existence.
	call("POST", "projects/"+p+"/environments/"+dev+"/promote", tokenV, map[string]any{"release_id": r1["id"]}, 403)
	call("POST", "projects/"+p+"/environments/"+dev+"/promote", token, map[string]any{"release_id": "missing"}, 404)
}
