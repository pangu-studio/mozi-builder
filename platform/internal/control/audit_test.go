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

// TestAuditSearch covers the three audit search endpoints: cursor
// pagination, filters, and isolation.
func TestAuditSearch(t *testing.T) {
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
	executor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer executor.Close()
	jobStore := &jobs.Store{DB: db}
	a := API{DB: db, Design: designDB, Jobs: jobStore, Dispatcher: &jobs.Dispatcher{Store: *jobStore, GatewayURL: executor.URL}}
	mux := router.NewRouter()
	for _, r := range a.Routes() {
		if err = mux.Handle(r.Method, r.Path, http.HandlerFunc(r.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	ctx := t.Context()
	if err = CreateUser(ctx, db, "aud@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "aud@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, tok string, body any, status int) map[string]any {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v2/"+path, strings.NewReader(string(data)))
		if tok != "" {
			r.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d expected %d: %s", method, path, w.Code, status, w.Body.String())
		}
		result := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	p := call("POST", "projects", token, map[string]string{"slug": "aud-proj", "name": "Aud"}, 201)["id"].(string)

	// Generate audit events: project creation + two releases + a job fire.
	modelDoc := map[string]any{"schema_version": 1, "module": "content", "model": "Card", "label": "卡片", "table": "cards", "fields": []map[string]any{{"name": "id", "type": "string", "label": "ID", "primary": true, "generated": "uuid"}}}
	call("POST", "projects/"+p+"/design/models", token, map[string]any{"document": modelDoc}, 201)
	jobDoc := map[string]any{"schema_version": 1, "module": "content", "job": "DigestJob", "label": "摘要", "schedule": "0 3 * * *", "executor": map[string]any{"kind": "http", "method": "POST", "path": "/jobs/x"}, "timeout_seconds": 60, "retry": map[string]any{"max_attempts": 1, "backoff_seconds": 60}}
	call("POST", "projects/"+p+"/design/jobs", token, map[string]any{"document": jobDoc}, 201)
	call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r1", "code_ref": "sha-1"}, 201)
	call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r2", "code_ref": "sha-2"}, 201)
	call("POST", "projects/"+p+"/jobs/content/DigestJob/fire", token, nil, 202)

	// Audit events: filter by action, then paginate.
	page1 := call("GET", "projects/"+p+"/audit?action=release.create&limit=1", token, nil, 200)
	items1 := page1["items"].([]any)
	if len(items1) != 1 || page1["next_cursor"] == "" {
		t.Fatalf("page1: %v", page1)
	}
	page2 := call("GET", "projects/"+p+"/audit?action=release.create&limit=1&cursor="+page1["next_cursor"].(string), token, nil, 200)
	items2 := page2["items"].([]any)
	if len(items2) != 1 {
		t.Fatalf("page2: %v", page2)
	}
	if items1[0].(map[string]any)["id"] == items2[0].(map[string]any)["id"] {
		t.Fatal("cursor must not repeat rows")
	}
	call("GET", "projects/"+p+"/audit?cursor=bogus", token, nil, 400)

	// Design changes per kind, across collections.
	models := call("GET", "projects/"+p+"/design-changes?kind=models", token, nil, 200)
	if len(models["items"].([]any)) != 1 {
		t.Fatalf("models changes: %v", models)
	}
	jobsPage := call("GET", "projects/"+p+"/design-changes?kind=jobs", token, nil, 200)
	if len(jobsPage["items"].([]any)) != 1 {
		t.Fatalf("jobs changes: %v", jobsPage)
	}
	call("GET", "projects/"+p+"/design-changes", token, nil, 400)

	// Executions with state filter.
	execs := call("GET", "projects/"+p+"/executions?job=DigestJob&state=running", token, nil, 200)
	_ = execs
	all := call("GET", "projects/"+p+"/executions", token, nil, 200)
	if len(all["items"].([]any)) != 1 {
		t.Fatalf("executions: %v", all)
	}

	// Isolation: another project's audit contains only its own rows.
	other := call("POST", "projects", token, map[string]string{"slug": "aud-other", "name": "Other"}, 201)["id"].(string)
	empty := call("GET", "projects/"+other+"/audit", token, nil, 200)
	for _, item := range empty["items"].([]any) {
		m := item.(map[string]any)
		if m["project_id"] != other {
			t.Fatalf("cross-project leak: %v", m)
		}
		if m["action"] == "release.create" {
			t.Fatalf("foreign release visible: %v", m)
		}
	}
}
