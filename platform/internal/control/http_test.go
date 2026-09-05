package control

import (
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest/router"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/migrate"
)

func TestPermissions(t *testing.T) {
	for _, role := range []string{"owner", "maintainer", "developer", "viewer", "", "admin"} {
		if Allowed(role, true, true) != (role == "owner") {
			t.Fatal(role)
		}
		if Allowed(role, true, false) != (role == "owner" || role == "maintainer") {
			t.Fatal(role)
		}
		if Allowed(role, false, false) != (role == "owner" || role == "maintainer" || role == "developer" || role == "viewer") {
			t.Fatal(role)
		}
	}
}
func TestPostgresIsolationAndSessions(t *testing.T) {
	env := os.Getenv("MOZI_INTEGRATION_ENV")
	if env == "" {
		t.Skip("set MOZI_INTEGRATION_ENV to a database env file")
	}
	if err := config.LoadEnvFile(env); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadDatabases(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	pg, err := pgx.ParseConfig(cfg.Platform)
	if err != nil {
		t.Fatal("invalid database configuration")
	}
	admin := stdlib.OpenDB(*pg)
	defer admin.Close()
	schema := "test_" + strings.ToLower(ID()) // generated letters/digits only, quoted as an identifier
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(`CREATE SCHEMA ` + quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(`DROP SCHEMA ` + quoted + ` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	pg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*pg)
	defer db.Close()
	ctx := context.Background()
	if _, err = migrate.Apply(ctx, db, "platform"); err != nil {
		t.Fatal(err)
	}
	if _, err = migrate.Apply(ctx, db, "platform"); err != nil {
		t.Fatal(err)
	}
	if err = migrate.Verify(ctx, db, "platform"); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"owner@test.local", "other@test.local", "viewer@test.local"} {
		if err = CreateUser(ctx, db, email, "测试用户", "test-password-123"); err != nil {
			t.Fatal(err)
		}
	}
	a := API{DB: db}
	mux := router.NewRouter()
	for _, route := range a.Routes() {
		if err = mux.Handle(route.Method, route.Path, http.HandlerFunc(route.Handler)); err != nil {
			t.Fatal(err)
		}
	}
	call := func(method, path, token, body string, status int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v2/"+path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		v := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	call("POST", "login", "", `{"email":"owner@test.local","password":"wrong"}`, 401)
	token := call("POST", "login", "", `{"email":"owner@test.local","password":"test-password-123"}`, 200)["access_token"].(string)
	other, err := login(ctx, db, "other@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := login(ctx, db, "viewer@test.local", "test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	project := call("POST", "projects", token, `{"slug":"test-project","name":"项目"}`, 201)["id"].(string)
	path := "projects/" + project + "/environments"
	call("GET", path, "", "", 401)
	var ownerID string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='owner@test.local'`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+project+"/members", token, `{"user_id":"`+ownerID+`","role":"viewer"}`, 409)
	call("GET", path, other, "", 404)
	call("POST", path, other, `{"slug":"dev","name":"开发","kind":"development"}`, 404)
	var viewerID string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='viewer@test.local'`).Scan(&viewerID); err != nil {
		t.Fatal(err)
	}
	call("POST", "projects/"+project+"/members", token, `{"user_id":"`+viewerID+`","role":"viewer"}`, 201)
	call("GET", path, viewer, "", 200)
	call("POST", path, viewer, `{"slug":"dev","name":"开发","kind":"development"}`, 403)
	call("POST", "projects/"+project+"/members", viewer, `{"user_id":"`+viewerID+`","role":"maintainer"}`, 403)
	call("POST", path, token, `{"slug":"dev","name":"开发","kind":"development"}`, 201)
	call("POST", path, token, `{"slug":"dev","name":"开发","kind":"development"}`, 409)
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM audit_events WHERE action='environments.write' AND result='succeeded'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count %d: %v", count, err)
	}
	if _, err = db.Exec(`UPDATE audit_events SET result='failed'`); err == nil {
		t.Fatal("audit mutation allowed")
	}
	if _, err = db.Exec(`DELETE FROM audit_events`); err == nil {
		t.Fatal("audit deletion allowed")
	}
	// A failed audit INSERT must roll back the environment write.
	_, err = db.Exec(`CREATE FUNCTION reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_audit()`)
	if err != nil {
		t.Fatal(err)
	}
	call("POST", path, token, `{"slug":"rollback","name":"回滚","kind":"staging"}`, 500)
	if err = db.QueryRow(`SELECT count(*) FROM environments WHERE slug='rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("write survived failed audit", err)
	}
	if _, err = db.Exec(`DROP TRIGGER reject_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	call("POST", "logout", token, "", 204)
	call("GET", "me", token, "", 401)
	_, err = db.Exec(`UPDATE sessions SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, digest(other))
	if err != nil {
		t.Fatal(err)
	}
	call("GET", "me", other, "", 401)
	_, err = db.Exec(`UPDATE users SET disabled=true WHERE id=$1`, viewerID)
	if err != nil {
		t.Fatal(err)
	}
	call("GET", "me", viewer, "", 401)
	if _, err = db.Exec(`UPDATE login_limits SET attempts=20`); err != nil {
		t.Fatal(err)
	}
	call("POST", "login", "", `{"email":"owner@test.local","password":"test-password-123"}`, 429)
}
