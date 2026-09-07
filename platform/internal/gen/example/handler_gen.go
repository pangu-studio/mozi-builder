package example

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ListDecks handles GET /api/content/decks.
func ListDecks(w http.ResponseWriter, r *http.Request) {
	// mozi:section ListDecksRequest — generated request plumbing
	_ = r
	// mozi:end ListDecksRequest

	// Handwritten business logic: return a demo deck list.
	httpx.OkJson(w, DeckSummary{Id: "demo-deck", Title: "示例牌组"})
}

// CreateDeck handles POST /api/content/decks.
func CreateDeck(w http.ResponseWriter, r *http.Request) {
	// mozi:section CreateDeckRequest — generated request plumbing
	var req CreateDeckRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
	_ = req
	// mozi:end CreateDeckRequest

	// Handwritten business logic: echo the created deck.
	httpx.OkJson(w, DeckSummary{Id: "new-deck", Title: req.Title})
}

// DeckHealth handles GET /api/content/health.
func DeckHealth(w http.ResponseWriter, r *http.Request) {
	// mozi:section DeckHealthRequest — generated request plumbing
	_ = r
	// mozi:end DeckHealthRequest

	// Handwritten business logic: health probe.
	httpx.OkJson(w, DeckSummary{Id: "ok"})
}
