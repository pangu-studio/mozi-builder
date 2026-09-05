package control

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrUnauthorized = errors.New("unauthorized")

func ID() string { return rand.Text() }
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func CreateUser(ctx context.Context, db *sql.DB, email, name, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 || strings.TrimSpace(name) == "" || len(password) < 12 || len(password) > 72 {
		return errors.New("valid email, name and 12–72 byte password required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO users(id,email,display_name,password_hash) VALUES($1,$2,$3,$4)`, ID(), email, name, string(hash))
	return err
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"display_name"`
}

func login(ctx context.Context, db *sql.DB, email, password string) (string, error) {
	var id, hash string
	err := db.QueryRowContext(ctx, `SELECT id,password_hash FROM users WHERE email=$1 AND NOT disabled`, strings.ToLower(strings.TrimSpace(email))).Scan(&id, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if errors.Is(err, sql.ErrNoRows) {
		hash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || id == "" {
		return "", ErrUnauthorized
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) SELECT $1,id,$3 FROM users WHERE id=$2 AND NOT disabled`, digest(token), id, time.Now().Add(12*time.Hour))
	if err == nil {
		n, e := result.RowsAffected()
		if e != nil {
			return "", e
		}
		if n != 1 {
			return "", ErrUnauthorized
		}
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(actor_id,request_id,action,resource_type,resource_id,result) VALUES($1,$2,'session.login','session',$1,'succeeded')`, id, ID())
	}
	if err == nil {
		err = tx.Commit()
	}
	return token, err
}
func authenticate(ctx context.Context, db *sql.DB, token string) (User, error) {
	var u User
	if len(token) != 64 {
		return u, ErrUnauthorized
	}
	err := db.QueryRowContext(ctx, `SELECT u.id,u.email,u.display_name FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND NOT u.disabled`, digest(token)).Scan(&u.ID, &u.Email, &u.Name)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrUnauthorized
	}
	return u, err
}
func Allowed(role string, write, members bool) bool {
	if members {
		return role == "owner"
	}
	if write {
		return role == "owner" || role == "maintainer"
	}
	return role == "owner" || role == "maintainer" || role == "developer" || role == "viewer"
}
