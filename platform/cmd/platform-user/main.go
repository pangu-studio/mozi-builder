package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pangu-studio/mozi-builder/platform/internal/control"
	"log"
	"os"
	"time"
)

func main() {
	env := flag.String("env-file", "", "database env file")
	email := flag.String("email", "", "email")
	name := flag.String("name", "", "display name")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := control.Open(ctx, *env)
	if err != nil {
		log.Fatal("database verification failed")
	}
	defer db.Close()
	if err = control.CreateUser(ctx, db, *email, *name, os.Getenv("MOZI_USER_PASSWORD")); err != nil {
		log.Fatal("user creation failed: check input and duplicate email")
	}
	fmt.Println("user created")
}
