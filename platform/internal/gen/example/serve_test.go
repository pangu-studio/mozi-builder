package example

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExampleServesHTTP wires the generated handlers into an HTTP mux and
// issues real requests: the phase-3 "HTTP example runs" evidence.
func TestExampleServesHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/content/decks", ListDecks)
	mux.HandleFunc("POST /api/content/decks", CreateDeck)
	mux.HandleFunc("GET /api/content/health", DeckHealth)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/content/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var health DeckSummary
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health.Id != "ok" {
		t.Fatalf("health: %+v", health)
	}

	resp, err = http.Post(srv.URL+"/api/content/decks", "application/json",
		strings.NewReader(`{"title":"英语单词","tags":["a2"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created DeckSummary
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "英语单词" {
		t.Fatalf("create echo: %+v", created)
	}

	resp, err = http.Get(srv.URL + "/api/content/decks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list DeckSummary
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Id != "demo-deck" {
		t.Fatalf("list: %+v", list)
	}

	// Malformed body must be rejected by the generated parse plumbing.
	resp, err = http.Post(srv.URL+"/api/content/decks", "application/json",
		strings.NewReader(`{"title":`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("malformed JSON must not return 200")
	}
}
