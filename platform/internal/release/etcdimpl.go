package release

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery is the production RPC adapter: go-zero style key-value
// registrations in etcd. Keys are <service>/<sanitized-node> so writes are
// idempotent upserts and re-execution after a crash is safe.
type EtcdDiscovery struct {
	Endpoints   []string
	DialTimeout time.Duration
}

func (e EtcdDiscovery) client() (*clientv3.Client, error) {
	timeout := e.DialTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return clientv3.New(clientv3.Config{Endpoints: e.Endpoints, DialTimeout: timeout})
}

func nodeKey(service, node string) string {
	safe := strings.NewReplacer("/", "_", ":", "_").Replace(node)
	return strings.TrimSuffix(service, "/") + "/" + safe
}

func (e EtcdDiscovery) RegisterRPC(ctx context.Context, service string, nodes []string) error {
	cli, err := e.client()
	if err != nil {
		return err
	}
	defer cli.Close()
	prefix := strings.TrimSuffix(service, "/") + "/"
	// Upsert desired nodes, then remove stale keys not in the desired set.
	want := map[string]bool{}
	for _, n := range nodes {
		want[n] = true
		if _, err = cli.Put(ctx, nodeKey(service, n), n); err != nil {
			return err
		}
	}
	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, kv := range resp.Kvs {
		if !want[string(kv.Value)] {
			if _, err = cli.Delete(ctx, string(kv.Key)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e EtcdDiscovery) DeregisterRPC(ctx context.Context, service string) error {
	cli, err := e.client()
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.Delete(ctx, strings.TrimSuffix(service, "/")+"/", clientv3.WithPrefix())
	return err
}

func (e EtcdDiscovery) ReadRPC(ctx context.Context, service string) ([]string, error) {
	cli, err := e.client()
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	resp, err := cli.Get(ctx, strings.TrimSuffix(service, "/")+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("read rpc registry: %w", err)
	}
	nodes := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		nodes = append(nodes, string(kv.Value))
	}
	sort.Strings(nodes)
	return nodes, nil
}
