// Phase-0 executable. No connection to the design/platform PostgreSQL databases.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pangu-studio/mozi-builder/platform/internal/poc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"
)

const prefix = "/mozi/poc/http/"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	mode := flag.String("mode", "api", "api, rpc, controller")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	switch *mode {
	case "rpc":
		rpc()
	case "api":
		api(ctx)
	case "controller":
		controller(ctx)
	default:
		log.Fatal("unknown mode")
	}
}
func rpc() {
	var c zrpc.RpcServerConf
	if err := conf.FillDefault(&c); err != nil {
		log.Fatal(err)
	}
	c.Name = "mozi-poc-rpc"
	c.ListenOn = "0.0.0.0:8081"
	c.Etcd = discov.EtcdConf{Hosts: []string{env("ETCD_ENDPOINT", "registry:2379")}, Key: "mozi.poc.rpc"}
	server := zrpc.MustNewServer(c, func(*grpc.Server) {})
	defer server.Stop()
	server.Start()
}
func etcd() *clientv3.Client {
	c, err := clientv3.New(clientv3.Config{Endpoints: []string{env("ETCD_ENDPOINT", "registry:2379")}, DialTimeout: 5 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	return c
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func api(ctx context.Context) {
	var cc zrpc.RpcClientConf
	conf.FillDefault(&cc)
	cc.Etcd = discov.EtcdConf{Hosts: []string{env("ETCD_ENDPOINT", "registry:2379")}, Key: "mozi.poc.rpc"}
	rpcClient := zrpc.MustNewClient(cc)
	defer rpcClient.Conn().Close()
	hc := health.NewHealthClient(rpcClient.Conn())
	host, _ := os.Hostname()
	var mu sync.Mutex
	seen := map[string]bool{}
	attempts := map[string]int{}
	var rc rest.RestConf
	conf.FillDefault(&rc)
	rc.Name = "mozi-poc-http"
	rc.Host = "0.0.0.0"
	rc.Port = 8080
	server := rest.MustNewServer(rc)
	defer server.Stop()
	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/poc/health", Handler: func(w http.ResponseWriter, r *http.Request) {
			reply, err := hc.Check(r.Context(), &health.HealthCheckRequest{})
			if err != nil {
				write(w, 503, map[string]string{"error": "rpc unavailable"})
				return
			}
			write(w, 200, map[string]any{"instance": host, "rpc_status": reply.Status.String()})
		}},
		{Method: "POST", Path: "/poc/jobs/run", Handler: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("slow") == "1" {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				write(w, 400, map[string]string{"error": "missing Idempotency-Key"})
				return
			}
			mu.Lock()
			defer mu.Unlock()
			attempts[key]++
			if r.URL.Query().Get("fail_first") == "1" && attempts[key] == 1 {
				write(w, 503, map[string]string{"error": "intentional first-attempt failure"})
				return
			}
			duplicate := seen[key]
			seen[key] = true
			write(w, 200, map[string]any{"run_id": key, "duplicate": duplicate, "attempts": attempts[key]})
		}},
		{Method: "GET", Path: "/poc/jobs/status", Handler: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			key := r.URL.Query().Get("id")
			write(w, 200, map[string]any{"run_id": key, "completed": seen[key], "attempts": attempts[key]})
		}},
	})
	c := etcd()
	defer c.Close()
	go register(ctx, c, host)
	go func() { <-ctx.Done(); server.Stop() }()
	server.Start()
}
func register(ctx context.Context, c *clientv3.Client, host string) {
	for ctx.Err() == nil {
		func() {
			call, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			lease, err := c.Grant(call, 10)
			if err != nil {
				log.Printf("register: %v", err)
				return
			}
			defer func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				c.Revoke(cleanup, lease.ID)
			}()
			_, err = c.Put(call, prefix+host, host+":8080", clientv3.WithLease(lease.ID))
			if err != nil {
				return
			}
			updates, err := c.KeepAlive(ctx, lease.ID)
			if err != nil {
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-updates:
					if !ok {
						return
					}
				}
			}
		}()
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func controller(ctx context.Context) {
	c := etcd()
	defer c.Close()
	gateway := poc.Gateway{BaseURL: env("APISIX_ADMIN_URL", "http://apisix:9180"), Key: env("APISIX_ADMIN_KEY", "")}
	if gateway.Key == "" {
		log.Fatal("APISIX_ADMIN_KEY required")
	}
	for ctx.Err() == nil {
		err := reconcile(ctx, c, gateway)
		if err != nil && ctx.Err() == nil {
			log.Printf("reconcile: %v; preserving last configuration", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func reconcile(ctx context.Context, c *clientv3.Client, g poc.Gateway) error {
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	snapshot, err := c.Get(call, prefix, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return err
	}
	nodesByKey := map[string]string{}
	for _, kv := range snapshot.Kvs {
		nodesByKey[string(kv.Key)] = string(kv.Value)
	}
	syncNodes := func() error {
		nodes := map[string]int{}
		for _, addr := range nodesByKey {
			nodes[addr] = 1
		}
		return g.Sync(ctx, nodes)
	}
	if err := syncNodes(); err != nil {
		return err
	}
	// Full snapshot + revision avoids missing changes between the initial read and watch.
	watch := c.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(snapshot.Header.Revision+1))
	for update := range watch {
		if err := update.Err(); err != nil {
			return err
		}
		for _, event := range update.Events {
			if event.Type == clientv3.EventTypeDelete {
				delete(nodesByKey, string(event.Kv.Key))
			} else {
				nodesByKey[string(event.Kv.Key)] = string(event.Kv.Value)
			}
		}
		if err := syncNodes(); err != nil {
			return err
		}
	}
	return fmt.Errorf("registry watch closed")
}
