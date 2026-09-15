package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestApisixAdminPayloads verifies the Admin API wire format without Docker:
// disable uses empty plugins and never touches the upstream; sync writes the
// upstream before the route; read-back reports nodes and liveness.
func TestApisixAdminPayloads(t *testing.T) {
	type call struct {
		method, path, body string
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "k" {
			w.WriteHeader(403)
			return
		}
		var body strings.Builder
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body.Write(buf)
		calls = append(calls, call{r.Method, r.URL.Path, body.String()})
		switch {
		case r.Method == "PUT":
			w.WriteHeader(200)
		case strings.HasPrefix(r.URL.Path, "/apisix/admin/routes/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"value": map[string]any{"status": 1, "upstream_id": "r1"}})
		case strings.HasPrefix(r.URL.Path, "/apisix/admin/upstreams/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"value": map[string]any{"nodes": map[string]int{"a:80": 1}}})
		}
	}))
	defer srv.Close()
	a := ApisixAdmin{BaseURL: srv.URL, Key: "k"}
	ctx := context.Background()

	if err := a.SyncRoute(ctx, "r1", "/acc/*", map[string]int{"a:80": 1}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0].path != "/apisix/admin/upstreams/r1" || calls[1].path != "/apisix/admin/routes/r1" {
		t.Fatalf("sync order: %+v", calls)
	}
	if !strings.Contains(calls[1].body, `"status":1`) || !strings.Contains(calls[1].body, `"upstream_id":"r1"`) {
		t.Fatalf("route payload: %s", calls[1].body)
	}

	nodes, live, err := a.ReadRoute(ctx, "r1")
	if err != nil || !live || nodes["a:80"] != 1 {
		t.Fatalf("readback: %v %v %v", nodes, live, err)
	}

	calls = nil
	if err := a.DisableRoute(ctx, "r1", "/acc/*"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !strings.Contains(calls[0].body, `"status":0`) || !strings.Contains(calls[0].body, `"plugins":{}`) {
		t.Fatalf("disable payload: %+v", calls)
	}

	if err := a.SyncRoute(ctx, "r1", "/acc/*", map[string]int{}); err == nil {
		t.Fatal("empty nodes must be rejected; disable is the empty-state path")
	}
}
