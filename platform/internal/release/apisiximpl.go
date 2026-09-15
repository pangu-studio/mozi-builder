package release

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ApisixAdmin is the production HTTP adapter: APISIX Admin API route and
// upstream management. Writes are idempotent PUTs. Disabling a route uses an
// empty plugins object and never references a missing upstream.
type ApisixAdmin struct {
	BaseURL string
	Key     string
	Client  *http.Client
}

func (a ApisixAdmin) call(ctx context.Context, method, path string, value any, out any) (int, error) {
	var body io.Reader
	if value != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.BaseURL, "/")+"/apisix/admin/"+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-API-KEY", a.Key)
	req.Header.Set("Content-Type", "application/json")
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = json.Unmarshal(data, out)
	}
	return resp.StatusCode, nil
}

func (a ApisixAdmin) SyncRoute(ctx context.Context, routeID, uri string, nodes map[string]int) error {
	if len(nodes) == 0 {
		return fmt.Errorf("sync route %s with no nodes; use disable instead", routeID)
	}
	if status, err := a.call(ctx, http.MethodPut, "upstreams/"+routeID, map[string]any{"type": "roundrobin", "nodes": nodes}, nil); err != nil || status >= 300 {
		return fmt.Errorf("sync upstream %s: status %d: %w", routeID, status, err)
	}
	status, err := a.call(ctx, http.MethodPut, "routes/"+routeID, map[string]any{"uri": uri, "upstream_id": routeID, "status": 1}, nil)
	if err != nil || status >= 300 {
		return fmt.Errorf("sync route %s: status %d: %w", routeID, status, err)
	}
	return nil
}

func (a ApisixAdmin) DisableRoute(ctx context.Context, routeID, uri string) error {
	// Empty plugins satisfies the APISIX schema; the upstream is kept intact
	// so discovery transport failures never remove nodes.
	status, err := a.call(ctx, http.MethodPut, "routes/"+routeID, map[string]any{"uri": uri, "status": 0, "plugins": map[string]any{}}, nil)
	if err != nil || status >= 300 {
		return fmt.Errorf("disable route %s: status %d: %w", routeID, status, err)
	}
	return nil
}

func (a ApisixAdmin) ReadRoute(ctx context.Context, routeID string) (map[string]int, bool, error) {
	var route struct {
		Value struct {
			Status     int    `json:"status"`
			UpstreamID string `json:"upstream_id"`
		} `json:"value"`
	}
	status, err := a.call(ctx, http.MethodGet, "routes/"+routeID, nil, &route)
	if status == 404 {
		return nil, false, nil
	}
	if err != nil || status >= 300 {
		return nil, false, fmt.Errorf("read route %s: status %d: %w", routeID, status, err)
	}
	live := route.Value.Status == 1
	if route.Value.UpstreamID == "" {
		return nil, live, nil
	}
	var upstream struct {
		Value struct {
			Nodes map[string]int `json:"nodes"`
		} `json:"value"`
	}
	status, err = a.call(ctx, http.MethodGet, "upstreams/"+route.Value.UpstreamID, nil, &upstream)
	if status == 404 {
		return nil, live, nil
	}
	if err != nil || status >= 300 {
		return nil, false, fmt.Errorf("read upstream %s: status %d: %w", route.Value.UpstreamID, status, err)
	}
	return upstream.Value.Nodes, live, nil
}
