package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Desired is the target configuration of a release operation. The two adapter
// families are deliberately separate shapes: RPC discovery writes go-zero etcd
// keys, HTTP routing writes APISIX upstream/route payloads. The two
// registration formats must never be mixed (architecture red line).
type Desired struct {
	// RPC discovery (etcd): service key and instance addresses.
	RPCService string   `json:"rpc_service,omitempty"`
	RPCNodes   []string `json:"rpc_nodes,omitempty"`
	// HTTP routing (APISIX): upstream nodes (host:port → weight) and route.
	HTTPRoute string         `json:"http_route,omitempty"`
	HTTPURI   string         `json:"http_uri,omitempty"`
	HTTPNodes map[string]int `json:"http_nodes,omitempty"`
	Disabled  bool           `json:"disabled,omitempty"`
}

// Observed is the read-back state used for verification and drift detection.
type Observed struct {
	RPCNodes  []string       `json:"rpc_nodes,omitempty"`
	HTTPNodes map[string]int `json:"http_nodes,omitempty"`
	RouteLive bool           `json:"route_live"`
}

// EtcdAdapter writes and reads RPC discovery registrations. Writes must be
// idempotent upserts; read-back returns nil error and empty nodes when the
// registry is unreachable — discovery transport failures never remove nodes.
type EtcdAdapter interface {
	RegisterRPC(ctx context.Context, service string, nodes []string) error
	DeregisterRPC(ctx context.Context, service string) error
	ReadRPC(ctx context.Context, service string) ([]string, error)
}

// ApisixAdapter writes and reads HTTP routes/upstreams. Writes must be
// idempotent upserts. Disabling a route uses an empty plugins object and
// never references an upstream that does not exist (PoC lesson).
type ApisixAdapter interface {
	SyncRoute(ctx context.Context, routeID, uri string, nodes map[string]int) error
	DisableRoute(ctx context.Context, routeID, uri string) error
	ReadRoute(ctx context.Context, routeID string) (nodes map[string]int, live bool, err error)
}

// ErrUnspecified is returned when a kind's desired payload is incomplete.
var ErrUnspecified = errors.New("release desired state incomplete")

// ParseDesired decodes and checks the desired payload for a kind.
func ParseDesired(kind string, raw json.RawMessage) (Desired, error) {
	var d Desired
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, fmt.Errorf("%w: invalid json", ErrUnspecified)
	}
	switch kind {
	case "deploy", "scale":
		if d.RPCService == "" && d.HTTPRoute == "" {
			return d, fmt.Errorf("%w: rpc_service or http_route required", ErrUnspecified)
		}
	case "route":
		if d.HTTPRoute == "" || d.HTTPURI == "" {
			return d, fmt.Errorf("%w: http_route and http_uri required", ErrUnspecified)
		}
	case "disable":
		if d.RPCService == "" && d.HTTPRoute == "" {
			return d, fmt.Errorf("%w: rpc_service or http_route required", ErrUnspecified)
		}
	default:
		return d, fmt.Errorf("%w: unknown kind %q", ErrUnspecified, kind)
	}
	return d, nil
}

// matches reports whether the read-back state satisfies the desired state.
// Nil/empty node lists on the observed side (registry unreachable) never
// count as a match, and also never as drift-worthy removal — callers
// distinguish the two via the error from the adapter read.
func (d Desired) matches(o Observed) bool {
	if d.Disabled {
		return !o.RouteLive
	}
	if d.HTTPRoute != "" {
		if !o.RouteLive || !nodesEqual(d.HTTPNodes, o.HTTPNodes) {
			return false
		}
	}
	if d.RPCService != "" && !stringSetsEqual(d.RPCNodes, o.RPCNodes) {
		return false
	}
	return true
}

func nodesEqual(want, got map[string]int) bool {
	if len(want) != len(got) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func stringSetsEqual(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	seen := make(map[string]int, len(want))
	for _, s := range want {
		seen[s]++
	}
	for _, s := range got {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// Execute applies the desired state through the adapters. All writes are
// idempotent upserts, so resuming after a crash simply re-executes.
func Execute(ctx context.Context, kind string, d Desired, etcd EtcdAdapter, apisix ApisixAdapter) error {
	if kind == "disable" || d.Disabled {
		if d.RPCService != "" {
			if err := etcd.DeregisterRPC(ctx, d.RPCService); err != nil {
				return err
			}
		}
		if d.HTTPRoute != "" {
			return apisix.DisableRoute(ctx, d.HTTPRoute, d.HTTPURI)
		}
		return nil
	}
	if d.RPCService != "" {
		if err := etcd.RegisterRPC(ctx, d.RPCService, d.RPCNodes); err != nil {
			return err
		}
	}
	if d.HTTPRoute != "" {
		return apisix.SyncRoute(ctx, d.HTTPRoute, d.HTTPURI, d.HTTPNodes)
	}
	return nil
}

// ReadBack observes the current state for verification and drift checks.
func ReadBack(ctx context.Context, d Desired, etcd EtcdAdapter, apisix ApisixAdapter) (Observed, error) {
	var o Observed
	if d.RPCService != "" {
		nodes, err := etcd.ReadRPC(ctx, d.RPCService)
		if err != nil {
			return o, err
		}
		o.RPCNodes = nodes
	}
	if d.HTTPRoute != "" {
		nodes, live, err := apisix.ReadRoute(ctx, d.HTTPRoute)
		if err != nil {
			return o, err
		}
		o.HTTPNodes = nodes
		o.RouteLive = live
	}
	return o, nil
}

// Matches exposes desired-vs-observed comparison for the controller.
func (d Desired) Matches(o Observed) bool { return d.matches(o) }
