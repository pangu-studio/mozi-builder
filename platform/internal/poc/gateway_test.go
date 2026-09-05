package poc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpstreamFailureDoesNotEnableRoute(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/apisix/admin/upstreams/mozi-v2-poc" {
			t.Errorf("unexpected write %s", r.URL.Path)
		}
		w.WriteHeader(503)
	}))
	defer s.Close()
	if err := (Gateway{BaseURL: s.URL}).Sync(context.Background(), map[string]int{"api:8080": 1}); err == nil {
		t.Fatal("expected failure")
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
func TestEmptySnapshotDisablesRouteWithoutDeletingUpstream(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PUT" || r.URL.Path != "/apisix/admin/routes/mozi-v2-poc" {
			t.Errorf("unexpected mutation")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != float64(0) {
			t.Error("route must be disabled")
		}
		if _, ok := body["plugins"].(map[string]any); !ok {
			t.Error("a route without upstream still needs plugins to satisfy APISIX schema")
		}
		w.WriteHeader(200)
	}))
	defer s.Close()
	if err := (Gateway{BaseURL: s.URL}).Sync(context.Background(), map[string]int{}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
