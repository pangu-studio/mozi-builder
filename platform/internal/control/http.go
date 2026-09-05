package control

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zeromicro/go-zero/rest"
)

type API struct {
	DB     *sql.DB
	Design *sql.DB
}
type input struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
}

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, status int) {
	reply(w, status, map[string]string{"error": http.StatusText(status)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400)
		return false
	}
	return true
}
func dbError(w http.ResponseWriter, err error) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && (pe.Code == "23505" || pe.Code == "23503") {
		fail(w, 409)
		return
	}
	fail(w, 500)
}
func (a API) Routes() []rest.Route {
	return append([]rest.Route{
		{Method: "POST", Path: "/api/v2/login", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/logout", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/me", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/environments", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects/:project/environments", Handler: a.handle},
		{Method: "GET", Path: "/api/v2/projects/:project/members", Handler: a.handle},
		{Method: "POST", Path: "/api/v2/projects/:project/members", Handler: a.handle},
	}, a.designRoutes()...)
}
func audit(r *http.Request, tx *sql.Tx, user, project, action, resource, result string) error {
	_, err := tx.ExecContext(r.Context(), `INSERT INTO audit_events(actor_id,project_id,request_id,action,resource_type,resource_id,result) VALUES($1,NULLIF($2,''),$3,$4,$5,$6,$7)`, user, project, ID(), action, action, resource, result)
	return err
}
func (a API) handle(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/")
	if path == "login" {
		source, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			fail(w, 400)
			return
		}
		var attempts int
		err = a.DB.QueryRowContext(r.Context(), `INSERT INTO login_limits(source_hash,window_start,attempts) VALUES($1,now(),1) ON CONFLICT(source_hash) DO UPDATE SET attempts=CASE WHEN login_limits.window_start < now()-interval '1 minute' THEN 1 ELSE login_limits.attempts+1 END,window_start=CASE WHEN login_limits.window_start < now()-interval '1 minute' THEN now() ELSE login_limits.window_start END RETURNING attempts`, digest(source)).Scan(&attempts)
		if err != nil {
			dbError(w, err)
			return
		}
		if attempts > 20 {
			w.Header().Set("Retry-After", "60")
			fail(w, 429)
			return
		}
		var v input
		if !decode(w, r, &v) {
			return
		}
		if len(v.Password) > 72 || len(v.Email) > 254 {
			fail(w, 401)
			return
		}
		token, err := login(r.Context(), a.DB, v.Email, v.Password)
		if errors.Is(err, ErrUnauthorized) {
			fail(w, 401)
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 43200})
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		fail(w, 401)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	u, err := authenticate(r.Context(), a.DB, token)
	if errors.Is(err, ErrUnauthorized) {
		fail(w, 401)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if strings.Contains(path, "/design/") {
		a.handleDesign(w, r, u, path)
		return
	}
	if path == "me" {
		reply(w, 200, u)
		return
	}
	if path == "logout" {
		tx, e := a.DB.BeginTx(r.Context(), nil)
		if e != nil {
			dbError(w, e)
			return
		}
		defer tx.Rollback()
		_, err = tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, digest(token))
		if err == nil {
			err = audit(r, tx, u.ID, "", "session.logout", u.ID, "succeeded")
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			dbError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	if path == "projects" && r.Method == "GET" {
		rows, err := a.DB.QueryContext(r.Context(), `SELECT p.id,p.slug,p.name,m.role FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=$1 ORDER BY p.id LIMIT 500`, u.ID)
		if err != nil {
			dbError(w, err)
			return
		}
		defer rows.Close()
		items := []map[string]string{}
		for rows.Next() {
			var id, slug, name, role string
			if err = rows.Scan(&id, &slug, &name, &role); err != nil {
				dbError(w, err)
				return
			}
			items = append(items, map[string]string{"id": id, "slug": slug, "name": name, "role": role})
		}
		if rows.Err() != nil {
			fail(w, 500)
			return
		}
		reply(w, 200, items)
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	if path == "projects" {
		var v input
		if !decode(w, r, &v) {
			return
		}
		if !slugRE.MatchString(v.Slug) || strings.TrimSpace(v.Name) == "" || len(v.Name) > 200 {
			fail(w, 400)
			return
		}
		id := ID()
		_, err = tx.ExecContext(r.Context(), `INSERT INTO projects(id,slug,name,created_by) VALUES($1,$2,$3,$4)`, id, v.Slug, v.Name, u.ID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO project_members(project_id,user_id,role) VALUES($1,$2,'owner')`, id, u.ID)
		}
		if err == nil {
			err = audit(r, tx, u.ID, id, "project.create", id, "succeeded")
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 201, map[string]string{"id": id})
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "projects" {
		fail(w, 404)
		return
	}
	project, resource := parts[1], parts[2]
	var role string
	err = tx.QueryRowContext(r.Context(), `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 FOR SHARE`, project, u.ID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		err = audit(r, tx, u.ID, "", "project.access", project, "denied")
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			dbError(w, err)
			return
		}
		fail(w, 404)
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	if !Allowed(role, r.Method != "GET", resource == "members" && r.Method != "GET") {
		err = audit(r, tx, u.ID, project, resource, project, "denied")
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			dbError(w, err)
			return
		}
		fail(w, 403)
		return
	}
	if r.Method == "GET" {
		query := `SELECT id,slug,name,kind FROM environments WHERE project_id=$1 ORDER BY id LIMIT 500`
		if resource == "members" {
			query = `SELECT user_id,role,'','' FROM project_members WHERE project_id=$1 ORDER BY user_id LIMIT 500`
		}
		rows, err := tx.QueryContext(r.Context(), query, project)
		if err != nil {
			dbError(w, err)
			return
		}
		items := []map[string]string{}
		for rows.Next() {
			var id, slug, name, kind string
			if err = rows.Scan(&id, &slug, &name, &kind); err != nil {
				rows.Close()
				dbError(w, err)
				return
			}
			item := map[string]string{"id": id, "slug": slug, "name": name, "kind": kind}
			if resource == "members" {
				item = map[string]string{"user_id": id, "role": slug}
			}
			items = append(items, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			dbError(w, err)
			return
		}
		reply(w, 200, items)
		return
	}
	var v input
	if !decode(w, r, &v) {
		return
	}
	id := ID()
	if resource == "environments" {
		if !slugRE.MatchString(v.Slug) || strings.TrimSpace(v.Name) == "" || len(v.Name) > 200 || (v.Kind != "development" && v.Kind != "staging" && v.Kind != "production") {
			fail(w, 400)
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO environments(id,project_id,slug,name,kind,protected) VALUES($1,$2,$3,$4,$5,$6)`, id, project, v.Slug, v.Name, v.Kind, v.Kind == "production")
	} else if resource == "members" {
		if v.UserID == "" || (v.Role != "maintainer" && v.Role != "developer" && v.Role != "viewer") {
			fail(w, 400)
			return
		}
		id = v.UserID
		// Owner transfer is deliberately not part of this endpoint.
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `INSERT INTO project_members(project_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT(project_id,user_id) DO UPDATE SET role=excluded.role WHERE project_members.role<>'owner'`, project, v.UserID, v.Role)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				fail(w, 409)
				return
			}
		}
	} else {
		fail(w, 404)
		return
	}
	if err == nil {
		err = audit(r, tx, u.ID, project, resource+".write", id, "succeeded")
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		dbError(w, err)
		return
	}
	reply(w, 201, map[string]string{"id": id})
}
