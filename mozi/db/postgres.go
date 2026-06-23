// Package db provides the design database access layer for mozi.
// The design database stores model metadata: modules, models, versions, fields, relations, and admin config.
// Uses PostgreSQL for multi-user collaboration support.
package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DefaultDesignDB is the default PostgreSQL connection string for the design database.
const DefaultDesignDB = "postgres://localhost:5432/memflow_design?sslmode=disable"

// Open opens a connection to the design database.
func Open(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open design database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping design database: %w", err)
	}

	return db, nil
}

// InitDB opens the database and runs migrations to create the schema.
func InitDB(connStr string) (*sql.DB, error) {
	db, err := Open(connStr)
	if err != nil {
		return nil, err
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate design database: %w", err)
	}

	return db, nil
}
