package example

import "time"

// DeckSummary is a message of the ContentService contract.
type DeckSummary struct {
	Id    string    `json:"id"`
	Title string    `json:"title"`
	DueAt time.Time `json:"due_at"`
}

// CreateDeckRequest is a message of the ContentService contract.
type CreateDeckRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}
