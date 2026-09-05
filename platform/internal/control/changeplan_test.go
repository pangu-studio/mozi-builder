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

// TestDesignChangePlan covers the change-plan subresource: pending after
// create/update, cross-project 404, viewer read access, and 404 for the
// services collection which has no change-plan support yet.
func TestDesignChangePlan(t *testing.T) {
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
	if err = CreateUser(ctx, db, "cp@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	if err = CreateUser(ctx, db, "cpv@test.local", "tester", "test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, err := login(ctx, db, "cp@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	tokenV, err := login(ctx, db, "cpv@test.local", "test-password-123")
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
	p := call("POST", "projects", token, map[string]string{"slug": "cp-proj", "name": "CP"}, 201)["id"].(string)
	var viewer string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='cpv@test.local'`).Scan(&viewer); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+p+"/members", token, map[string]string{"user_id": viewer, "role": "viewer"}, 201)

	doc := map[string]any{"schema_version": 1, "module": "content", "model": "Note", "label": "便签", "table": "notes", "fields": []map[string]any{{"name": "id", "type": "string", "label": "ID", "primary": true, "generated": "uuid"}}}
	root := "projects/" + p + "/design/models"
	item := root + "/content/Note"
	one := call("POST", root, token, map[string]any{"document": doc}, 201)

	// Freshly created: diff against empty identity is all-additions → pending.
	plan := call("GET", item+"/change-plan", token, nil, 200)
	if plan["status"] != "pending" || plan["model_ref"] != "content/Note" {
		t.Fatalf("unexpected plan: %v", plan)
	}
	if plan["requires_approval"] == true {
		t.Fatal("all-additions diff must not require approval")
	}

	// After an update, the plan carries the modification diff.
	doc["label"] = "便签 v2"
	call("PUT", item, token, map[string]any{"version": one["version"], "document": doc}, 200)
	plan = call("GET", item+"/change-plan", token, nil, 200)
	if plan["status"] != "pending" {
		t.Fatalf("status after update: %v", plan["status"])
	}
	diff := plan["diff"].(map[string]any)
	if diff["has_changes"] != true {
		t.Fatalf("expected changes, got %v", diff)
	}

	// Viewer may read the plan; non-members get 404; services 404 for now.
	call("GET", item+"/change-plan", tokenV, nil, 200)
	call("GET", "projects/"+p+"/design/services/content/X/change-plan", token, nil, 404)
	other := call("POST", "projects", tokenV, map[string]string{"slug": "cp-other", "name": "Other"}, 201)["id"].(string)
	call("GET", strings.Replace(item, p, other, 1)+"/change-plan", token, nil, 404)
}
