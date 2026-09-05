package control

import (
	"context"
	"database/sql"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/migrate"
)

func Open(ctx context.Context, envFile string) (*sql.DB, error) {
	if envFile != "" {
		if err := config.LoadEnvFile(envFile); err != nil {
			return nil, err
		}
	}
	cfg, err := config.LoadDatabases(os.Getenv)
	if err != nil {
		return nil, err
	}
	for _, v := range []struct{ dsn, target string }{{cfg.Design, "design"}, {cfg.Platform, "platform"}} {
		db, err := sql.Open("pgx", v.dsn)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(2)
		if err = migrate.Verify(ctx, db, v.target); err != nil {
			db.Close()
			return nil, err
		}
		if v.target == "platform" {
			return db, nil
		}
		db.Close()
	}
	panic("unreachable")
}
