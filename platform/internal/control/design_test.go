package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/migrate"
	"github.com/zeromicro/go-zero/rest/router"
)

func isolatedDB(t *testing.T, dsn, target string) *sql.DB {
	t.Helper()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid config")
	}
	admin := stdlib.OpenDB(*cfg)
	schema := "test_" + strings.ToLower(ID())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(`CREATE SCHEMA ` + quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.Exec(`DROP SCHEMA ` + quoted + ` CASCADE`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if _, err = migrate.Apply(context.Background(), db, target); err != nil {
		t.Fatal(err)
	}
	return db
}
func TestDesignProjectIsolationAndVersions(t *testing.T) {
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
	ctx := context.Background()
	for _, email := range []string{"a@test.local", "b@test.local", "v@test.local"} {
		if err = CreateUser(ctx, db, email, "tester", "test-password-123"); err != nil {
			t.Fatal(err)
		}
	}
	tokenA, err := login(ctx, db, "a@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	tokenB, err := login(ctx, db, "b@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	tokenV, err := login(ctx, db, "v@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	send := func(method, path, token string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v2/"+path, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	call := func(method, path, token string, body any, status int) map[string]any {
		t.Helper()
		w := send(method, path, token, body)
		if w.Code != status {
			t.Fatalf("%s %s: %d expected %d: %s", method, path, w.Code, status, w.Body.String())
		}
		result := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	pa := call("POST", "projects", tokenA, map[string]string{"slug": "alpha", "name": "Alpha"}, 201)["id"].(string)
	pb := call("POST", "projects", tokenB, map[string]string{"slug": "beta", "name": "Beta"}, 201)["id"].(string)
	var viewer string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='v@test.local'`).Scan(&viewer); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+pa+"/members", tokenA, map[string]string{"user_id": viewer, "role": "viewer"}, 201)
	doc := map[string]any{"schema_version": 1, "module": "content", "model": "Card", "label": "Alpha card", "table": "cards", "fields": []map[string]any{{"name": "id", "type": "string", "label": "ID", "primary": true, "generated": "uuid"}}, "semantics": map[string]any{"purpose": "isolation test"}, "ui_intent": map[string]any{"product_goal": "test"}, "api_intent": map[string]any{"auth": "user_jwt"}, "admin": map[string]any{"page_size": 20}}
	rootA := "projects/" + pa + "/design/models"
	rootB := "projects/" + pb + "/design/models"
	itemA := rootA + "/content/Card"
	itemB := rootB + "/content/Card"
	one := call("POST", rootA, tokenA, map[string]any{"document": doc}, 201)
	v1 := one["version"].(string)
	doc["label"] = "Beta card"
	call("POST", rootB, tokenB, map[string]any{"document": doc}, 201)
	got := call("GET", itemA, tokenA, nil, 200)
	if got["document"].(map[string]any)["label"] != "Alpha card" {
		t.Fatal("cross-project overwrite")
	}
	for _, c := range []struct{ method, path string }{{"GET", rootB}, {"POST", rootB}, {"GET", itemB}, {"PUT", itemB}, {"DELETE", itemB}, {"GET", itemB + "/history"}} {
		call(c.method, c.path, tokenA, map[string]any{"version": v1, "document": doc}, 404)
	}
	call("GET", rootA, tokenV, nil, 200)
	call("PUT", itemA, tokenV, map[string]any{"version": v1, "document": doc}, 403)
	call("PUT", itemA, tokenA, map[string]any{"document": doc}, 428)
	doc["label"] = "Alpha updated"
	two := call("PUT", itemA, tokenA, map[string]any{"version": v1, "document": doc}, 200)
	v2 := two["version"].(string)
	if v1 == v2 {
		t.Fatal("unchanged version")
	}
	call("PUT", itemA, tokenA, map[string]any{"version": v1, "document": doc}, 409)
	call("DELETE", itemA, tokenA, map[string]any{"version": v1}, 409)
	// Two simultaneous editors with the same base: exactly one may commit.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- send("PUT", itemA, tokenA, map[string]any{"version": v2, "document": doc}).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	latest := call("GET", itemA, tokenA, nil, 200)
	version := latest["version"].(string)
	// Snapshot insertion failure rolls back the document/version update.
	if _, err = designDB.Exec(`CREATE FUNCTION fail_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture failure'; END $$; CREATE TRIGGER fail_history BEFORE INSERT ON design_model_history FOR EACH ROW EXECUTE FUNCTION fail_history()`); err != nil {
		t.Fatal(err)
	}
	call("PUT", itemA, tokenA, map[string]any{"version": version, "document": doc}, 500)
	if call("GET", itemA, tokenA, nil, 200)["version"] != version {
		t.Fatal("partial write")
	}
	if _, err = designDB.Exec(`DROP TRIGGER fail_history ON design_model_history`); err != nil {
		t.Fatal(err)
	}
	call("DELETE", itemA, tokenA, map[string]any{"version": version}, 200)
	call("GET", itemA, tokenA, nil, 404)
	call("POST", rootA, tokenA, map[string]any{"document": doc}, 409)
	call("GET", itemB, tokenB, nil, 200)
	var count int
	if err = designDB.QueryRow(`SELECT count(*) FROM design_model_history WHERE project_id=$1`, pa).Scan(&count); err != nil || count != 4 {
		t.Fatalf("history count %d: %v", count, err)
	}
	call("GET", itemA+"/history", tokenA, nil, 200)
	if _, err = designDB.Exec(`UPDATE design_model_history SET action='deleted'`); err == nil {
		t.Fatal("history mutable")
	}
}
