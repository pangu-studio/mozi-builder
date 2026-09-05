package differ

import (
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func baseService() *mozi.ServiceIR {
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
					{Name: "review_count", Type: "int", Number: 3},
				},
			},
		},
		HTTP: []mozi.HTTPRouteIR{
			{Name: "ListDecks", Method: "GET", Path: "/api/content/decks", Response: "DeckSummary"},
		},
		RPC: []mozi.RPCMethodIR{
			{Name: "GetDeck", Request: "GetDeckRequest", Response: "DeckSummary"},
		},
	}
}

func findChange(t *testing.T, changes []FieldChange, category, name string) FieldChange {
	t.Helper()
	for _, c := range changes {
		if c.Category == category && c.Name == name {
			return c
		}
	}
	t.Fatalf("change %s/%s not found in %v", category, name, changes)
	return FieldChange{}
}

func TestCompareServicesNoChanges(t *testing.T) {
	res := CompareServices(baseService(), baseService())
	if res.HasChanges {
		t.Fatalf("expected no changes, got %v", res.Changes)
	}
}

func TestCompareServicesAddedFieldSafe(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields = append(to.Messages[0].Fields,
		mozi.MessageFieldIR{Name: "due", Type: "time", Number: 4})
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.due")
	if c.Type != ChangeAdded || c.Compatibility != CompatibilitySafe {
		t.Fatalf("added field should be safe, got %+v", c)
	}
}

func TestCompareServicesRemovedFieldReservedConditional(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields = to.Messages[0].Fields[:2]
	to.Messages[0].ReservedNumbers = []int32{3}
	to.Messages[0].ReservedNames = []string{"review_count"}
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.review_count")
	if c.Type != ChangeRemoved || c.Compatibility != CompatibilityConditional {
		t.Fatalf("reserved removal should be conditional, got %+v", c)
	}
}

func TestCompareServicesRemovedFieldUnreservedBreaking(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields = to.Messages[0].Fields[:2]
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.review_count")
	if c.Type != ChangeRemoved || c.Compatibility != CompatibilityBreaking {
		t.Fatalf("unreserved removal should be breaking, got %+v", c)
	}
}

func TestCompareServicesNumberChangeAlwaysBreaking(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields[1].Number = 7
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.title")
	if c.Compatibility != CompatibilityBreaking {
		t.Fatalf("number change must be breaking, got %+v", c)
	}
}

func TestCompareServicesRenameSameNumberSafe(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields[1].Name = "deck_title"
	to.Messages[0].Fields[1].RenamedFrom = "title"
	to.Messages[0].ReservedNames = []string{"title"}
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.deck_title")
	if c.Compatibility != CompatibilitySafe {
		t.Fatalf("rename with unchanged number should be safe, got %+v", c)
	}
}

func TestCompareServicesRenameNumberChangeBreaking(t *testing.T) {
	to := baseService()
	to.Messages[0].Fields[1].Name = "deck_title"
	to.Messages[0].Fields[1].RenamedFrom = "title"
	to.Messages[0].Fields[1].Number = 9
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.deck_title")
	if c.Compatibility != CompatibilityBreaking {
		t.Fatalf("rename that also changes number must be breaking, got %+v", c)
	}
}

func TestCompareServicesTypeChangeClassification(t *testing.T) {
	// int → float: conditional
	to := baseService()
	to.Messages[0].Fields[2].Type = "float"
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryMessageField, "DeckSummary.review_count")
	if c.Compatibility != CompatibilityConditional {
		t.Fatalf("int→float should be conditional, got %+v", c)
	}

	// string → int: breaking
	to2 := baseService()
	to2.Messages[0].Fields[0].Type = "int"
	res2 := CompareServices(baseService(), to2)
	c2 := findChange(t, res2.Changes, ServiceCategoryMessageField, "DeckSummary.id")
	if c2.Compatibility != CompatibilityBreaking {
		t.Fatalf("string→int should be breaking, got %+v", c2)
	}
}

func TestCompareServicesRoutes(t *testing.T) {
	// removal → breaking
	to := baseService()
	to.HTTP = nil
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryHTTP, "ListDecks")
	if c.Compatibility != CompatibilityBreaking {
		t.Fatalf("route removal should be breaking, got %+v", c)
	}

	// path change → breaking; auth change → conditional
	to2 := baseService()
	to2.HTTP[0].Path = "/api/v2/decks"
	to2.HTTP[0].Auth = "admin"
	res2 := CompareServices(baseService(), to2)
	breaking, conditional := 0, 0
	for _, ch := range res2.Changes {
		if ch.Category != ServiceCategoryHTTP {
			continue
		}
		switch ch.Compatibility {
		case CompatibilityBreaking:
			breaking++
		case CompatibilityConditional:
			conditional++
		}
	}
	if breaking == 0 || conditional == 0 {
		t.Fatalf("expected breaking path change and conditional auth change, got %v", res2.Changes)
	}

	// addition → safe
	from := baseService()
	from.HTTP = nil
	res3 := CompareServices(from, baseService())
	c3 := findChange(t, res3.Changes, ServiceCategoryHTTP, "ListDecks")
	if c3.Compatibility != CompatibilitySafe {
		t.Fatalf("route addition should be safe, got %+v", c3)
	}
}

func TestCompareServicesRPC(t *testing.T) {
	to := baseService()
	to.RPC = nil
	res := CompareServices(baseService(), to)
	c := findChange(t, res.Changes, ServiceCategoryRPC, "GetDeck")
	if c.Compatibility != CompatibilityBreaking {
		t.Fatalf("rpc removal should be breaking, got %+v", c)
	}

	to2 := baseService()
	to2.RPC[0].Idempotency = "read_safe"
	res2 := CompareServices(baseService(), to2)
	c2 := findChange(t, res2.Changes, ServiceCategoryRPC, "GetDeck")
	if c2.Compatibility != CompatibilityConditional {
		t.Fatalf("idempotency change should be conditional, got %+v", c2)
	}
}
