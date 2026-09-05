package service

import (
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func validService() *mozi.ServiceIR {
	return &mozi.ServiceIR{
		Module: "content",
		Name:   "ContentService",
		Label:  "内容服务",
		Messages: []mozi.MessageIR{
			{
				Name: "DeckSummary",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
					{Name: "title", Type: "string", Number: 2},
				},
				ReservedNumbers: []int32{3},
				ReservedNames:   []string{"review_count"},
			},
			{
				Name: "GetDeckRequest",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
				},
			},
		},
		HTTP: []mozi.HTTPRouteIR{
			{Name: "ListDecks", Method: "GET", Path: "/api/content/decks", Response: "DeckSummary", Auth: "jwt"},
		},
		RPC: []mozi.RPCMethodIR{
			{Name: "GetDeck", Request: "GetDeckRequest", Response: "DeckSummary"},
		},
	}
}

func TestValidateValidService(t *testing.T) {
	res := Validate(validService())
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %v", res.Errors)
	}
}

func TestValidateRejectsReservedNumberReuse(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "new_field", Type: "string", Number: 3})
	res := Validate(svc)
	assertHasError(t, res, "reserved")
}

func TestValidateRejectsReservedNameReuse(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "review_count", Type: "int", Number: 4})
	res := Validate(svc)
	assertHasError(t, res, "reserved")
}

func TestValidateRejectsDuplicateNumber(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "other", Type: "string", Number: 2})
	res := Validate(svc)
	assertHasError(t, res, "already used")
}

func TestValidateRejectsDescriptorRange(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "meta", Type: "string", Number: 19000})
	res := Validate(svc)
	assertHasError(t, res, "descriptor range")
}

func TestValidateRejectsOutOfRangeNumber(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "bad", Type: "string", Number: 0})
	res := Validate(svc)
	assertHasError(t, res, "out of range")
}

func TestValidateRejectsBadFieldType(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields[0].Type = "bytes"
	res := Validate(svc)
	assertHasError(t, res, "invalid field type")
}

func TestValidateReferenceFormats(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields = append(svc.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "parent", Type: "message:DeckSummary", Number: 5},
		mozi.MessageFieldIR{Name: "deck", Type: "model:content/Deck", Number: 6},
		mozi.MessageFieldIR{Name: "bad", Type: "model:123 invalid!", Number: 7},
	)
	res := Validate(svc)
	assertHasError(t, res, "invalid model reference")
	if len(res.Errors) != 1 {
		t.Fatalf("expected exactly 1 error, got %v", res.Errors)
	}
}

func TestValidateRouteAndRPCReferences(t *testing.T) {
	svc := validService()
	svc.HTTP[0].Response = "Missing"
	svc.RPC[0].Request = "AlsoMissing"
	res := Validate(svc)
	assertHasError(t, res, `response message "Missing" not defined`)
	assertHasError(t, res, `request message "AlsoMissing" not defined`)
}

func TestValidateRenameWarning(t *testing.T) {
	svc := validService()
	svc.Messages[0].Fields[1].RenamedFrom = "name"
	res := Validate(svc)
	if !res.Valid {
		t.Fatalf("rename with unchanged number should stay valid, got %v", res.Errors)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, "reserved_names") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected warning recommending reserving the old name")
	}
}

func TestNextNumber(t *testing.T) {
	msg := &mozi.MessageIR{}
	if n, _ := NextNumber(msg); n != 1 {
		t.Fatalf("empty message: got %d, want 1", n)
	}
	msg.Fields = []mozi.MessageFieldIR{{Number: 2}}
	msg.ReservedNumbers = []int32{5}
	if n, _ := NextNumber(msg); n != 6 {
		t.Fatalf("got %d, want 6 (reserved must win over fields)", n)
	}
	msg.ReservedNumbers = []int32{18999}
	if n, _ := NextNumber(msg); n != 20000 {
		t.Fatalf("descriptor range not skipped: got %d, want 20000", n)
	}
}

func TestNextNumberExhausted(t *testing.T) {
	msg := &mozi.MessageIR{ReservedNumbers: []int32{MaxFieldNumber}}
	if _, err := NextNumber(msg); err == nil {
		t.Fatal("expected exhaustion error")
	}
}

func assertHasError(t *testing.T, res *ValidationResult, substr string) {
	t.Helper()
	if res.Valid {
		t.Fatalf("expected invalid result containing %q", substr)
	}
	for _, e := range res.Errors {
		if strings.Contains(e.Message, substr) {
			return
		}
	}
	t.Fatalf("no error containing %q; got %v", substr, res.Errors)
}
