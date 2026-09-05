package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
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

func loadEnvFile(path string) error { return config.LoadEnvFile(path) }
