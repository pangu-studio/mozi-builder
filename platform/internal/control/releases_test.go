package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/zeromicro/go-zero/rest/router"
)

// TestReleases covers creation with frozen design snapshots, provenance, and
// role/isolation rules.
func TestReleases(t *testing.T) {
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
	a := API{DB: db, Design: designDB}
	mux := router.NewRouter()
	for _, r := range a.Routes() {
		if err = mux.Handle(r.Method, r.Path, http.HandlerFunc(r.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	ctx := t.Context()
	if err = CreateUser(ctx, db, "rel@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	if err = CreateUser(ctx, db, "relv@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "rel@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	tokenV, err := login(ctx, db, "relv@test.local", "test-password-123")
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
	callList := func(path, tok string, status int) []any {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v2/"+path, nil)
		r.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("GET %s: %d expected %d: %s", path, w.Code, status, w.Body.String())
		}
		result := []any{}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	p := call("POST", "projects", token, map[string]string{"slug": "rel-proj", "name": "Rel"}, 201)["id"].(string)
	var viewer string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='relv@test.local'`).Scan(&viewer); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+p+"/members", token, map[string]string{"user_id": viewer, "role": "viewer"}, 201)

	// One model, one service, one job in the design DB.
	modelDoc := map[string]any{"schema_version": 1, "module": "content", "model": "Card", "label": "卡片", "table": "cards", "fields": []map[string]any{{"name": "id", "type": "string", "label": "ID", "primary": true, "generated": "uuid"}}}
	call("POST", "projects/"+p+"/design/models", token, map[string]any{"document": modelDoc}, 201)
	svcDoc := map[string]any{"schema_version": 1, "module": "content", "service": "ContentService", "label": "内容服务", "messages": []map[string]any{{"message": "DeckSummary", "fields": []map[string]any{{"name": "id", "type": "string", "number": 1}}}}, "http": []map[string]any{{"name": "ListDecks", "method": "GET", "path": "/api/content/decks", "response": "DeckSummary"}}}
	call("POST", "projects/"+p+"/design/services", token, map[string]any{"document": svcDoc}, 201)
	jobDoc := map[string]any{"schema_version": 1, "module": "content", "job": "DigestJob", "label": "摘要", "schedule": "0 3 * * *", "executor": map[string]any{"kind": "http", "method": "POST", "path": "/jobs/x"}, "timeout_seconds": 60, "retry": map[string]any{"max_attempts": 1, "backoff_seconds": 60}}
	call("POST", "projects/"+p+"/design/jobs", token, map[string]any{"document": jobDoc}, 201)

	// Create the first release: snapshot freezes all three collections.
	r1 := call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r1", "code_ref": "sha-1"}, 201)
	snap1 := r1["design_versions"].(map[string]any)
	if snap1["models"].(map[string]any)["content/Card"] == "" || snap1["services"].(map[string]any)["content/ContentService"] == "" || snap1["jobs"].(map[string]any)["content/DigestJob"] == "" {
		t.Fatalf("snapshot incomplete: %v", snap1)
	}

	// Evolve the model afterwards; the frozen snapshot must not change.
	modelDoc["label"] = "卡片 v2"
	latest := call("GET", "projects/"+p+"/design/models/content/Card", token, nil, 200)
	call("PUT", "projects/"+p+"/design/models/content/Card", token, map[string]any{"version": latest["version"], "document": modelDoc}, 200)
	r2 := call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "r2", "code_ref": "sha-2"}, 201)
	if r2["design_versions"].(map[string]any)["models"].(map[string]any)["content/Card"] == snap1["models"].(map[string]any)["content/Card"] {
		t.Fatal("second release must capture the new model version")
	}
	prov := call("GET", "projects/"+p+"/releases/"+r1["id"].(string)+"/provenance", token, nil, 200)
	frozen := prov["release"].(map[string]any)["design_versions"].(map[string]any)["models"].(map[string]any)["content/Card"]
	if frozen != snap1["models"].(map[string]any)["content/Card"] {
		t.Fatal("first release snapshot mutated after design change")
	}
	if envs, ok := prov["environment_releases"].([]any); !ok || len(envs) != 0 {
		t.Fatalf("environment history: %v", prov["environment_releases"])
	}

	// List is newest-first; viewer may read but not create; bad input 400.
	list := callList("projects/"+p+"/releases", token, 200)
	if len(list) != 2 || list[0].(map[string]any)["id"] != r2["id"] {
		t.Fatalf("list: %v", list)
	}
	callList("projects/"+p+"/releases", tokenV, 200)
	call("POST", "projects/"+p+"/releases", tokenV, map[string]string{"label": "x", "code_ref": "y"}, 403)
	call("POST", "projects/"+p+"/releases", token, map[string]string{"label": "no-code-ref"}, 400)
	other := call("POST", "projects", tokenV, map[string]string{"slug": "rel-other", "name": "Other"}, 201)["id"].(string)
	call("GET", "projects/"+other+"/releases/"+r1["id"].(string)+"/provenance", token, nil, 404)
}
