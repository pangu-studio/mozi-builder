package migration

import (
	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
	"testing"
)

func TestAdviseSafeAndDangerousChanges(t *testing.T) {
	from := &mozi.ModelIR{Module: "content", Name: "Deck", Table: "decks", Fields: []mozi.FieldIR{{Name: "legacy", Type: mozi.FieldTypeString}}}
	to := &mozi.ModelIR{Module: "content", Name: "Deck", Table: "decks", Fields: []mozi.FieldIR{{Name: "color", Type: mozi.FieldTypeString}}}
	plan := Advise(from, to, differ.Compare(from, to, "v1", "v2"))
	if len(plan.Steps) != 2 || !plan.HasDangerous {
		t.Fatalf("plan = %#v", plan)
	}
	if plan.Steps[0].SQL == "" {
		t.Fatal("safe add should include SQL")
	}
}
