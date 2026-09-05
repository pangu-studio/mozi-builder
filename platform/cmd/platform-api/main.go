package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/pangu-studio/mozi-builder/platform/internal/control"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	env := flag.String("env-file", "", "explicit database environment file")
	host := flag.String("host", "127.0.0.1", "listen host")
	port := flag.Int("port", 15180, "listen port")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := control.Open(ctx, *env)
	if err != nil {
		log.Fatal("database schema verification failed")
	}
	defer db.Close()
	designDB, err := control.OpenDesign(ctx)
	if err != nil {
		log.Fatal("design database verification failed")
	}
	defer designDB.Close()
	var c rest.RestConf
	if err = conf.FillDefault(&c); err != nil {
		log.Fatal(err)
	}
	c.Name = "mozi-platform"
	c.Host = *host
	c.Port = *port
	c.Timeout = 15000
	// Disable request-body logging: login requests contain passwords.
	c.Middlewares.Log = false
	server, err := rest.NewServer(c)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Stop()
	server.AddRoutes(control.API{DB: db, Design: designDB}.Routes())
	server.Start()
}
