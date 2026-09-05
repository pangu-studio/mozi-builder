package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pangu-studio/mozi-builder/platform/internal/config"
	"github.com/pangu-studio/mozi-builder/platform/internal/migrate"
)

func main() {
	target := flag.String("target", "", "database target: design or platform")
	verify := flag.Bool("verify", false, "verify schema without applying migrations")
	envFile := flag.String("env-file", "", "optional dotenv file containing explicit database URLs")
	flag.Parse()
	if *target != "design" && *target != "platform" {
		log.Fatal("-target must be design or platform")
	}
	if *envFile != "" {
		if err := loadEnvFile(*envFile); err != nil {
			log.Fatal(err)
		}
	}
	dbs, err := config.LoadDatabases(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	dsn := dbs.Design
	if *target == "platform" {
		dsn = dbs.Platform
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("open database: ", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if *verify {
		if err := migrate.Verify(ctx, db, *target); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s database is current\n", *target)
		return
	}
	applied, err := migrate.Apply(ctx, db, *target)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range applied {
		fmt.Printf("applied %s %04d_%s\n", *target, item.Version, item.Name)
	}
	if len(applied) == 0 {
		fmt.Printf("%s database already current\n", *target)
	}
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open environment file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment file line")
		}
		key = strings.TrimSpace(key)
		if key != "MOZI_DB" && key != "MOZI_PLATFORM_DB" {
			continue
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"'`)); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read environment file: %w", err)
	}
	return nil
}
