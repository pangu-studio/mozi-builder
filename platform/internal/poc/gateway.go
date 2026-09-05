// Package poc contains disposable phase-0 integration probes, not production controllers.
package poc

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

// Gateway only writes the dedicated PoC route/upstream. It cannot manage arbitrary IDs.
type Gateway struct {
	BaseURL, Key string
	Client       *http.Client
}

func (g Gateway) put(ctx context.Context, path string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(g.BaseURL, "/")+"/apisix/admin/"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", g.Key)
	req.Header.Set("Content-Type", "application/json")
	c := g.Client
	if c == nil {
		c = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gateway %s returned %d", path, resp.StatusCode)
	}
	return nil
}
func (g Gateway) Sync(ctx context.Context, nodes map[string]int) error {
	// Disable the route when an authoritative registry snapshot is empty. Keep
	// its upstream intact, so discovery transport failures never remove nodes.
	if len(nodes) == 0 {
		return g.put(ctx, "routes/mozi-v2-poc", map[string]any{"uri": "/poc/*", "status": 0, "plugins": map[string]any{}})
	}
	if err := g.put(ctx, "upstreams/mozi-v2-poc", map[string]any{"type": "roundrobin", "nodes": nodes}); err != nil {
		return err
	}
	return g.put(ctx, "routes/mozi-v2-poc", map[string]any{"uri": "/poc/*", "upstream_id": "mozi-v2-poc", "status": 1})
}
