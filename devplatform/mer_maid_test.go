package devplatform

import (
	"strings"
	"testing"

	"github.com/pangu-sutido/mozi-builder/mozi"
)

func TestMermaidEntityFormatsRequiredFieldsAsComment(t *testing.T) {
	model := &mozi.ModelIR{
		Name: "Card",
		Fields: []mozi.FieldIR{
			{Name: "id", Type: mozi.FieldTypeString, Primary: true},
			{Name: "front", Type: mozi.FieldTypeText, Required: true},
			{Name: "slug", Type: mozi.FieldTypeString, Unique: true, Required: true},
		},
	}

	got := mermaidEntity(model)

	wantLines := []string{
		`        string id PK`,
		`        string front "NOT NULL"`,
		`        string slug UK "NOT NULL"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Fatalf("mermaid entity missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "front NOT NULL") || strings.Contains(got, "slug UK NOT NULL") {
		t.Fatalf("required fields must use Mermaid comments, got:\n%s", got)
	}
}

func TestMermaidRelationUsesBusinessLabel(t *testing.T) {
	model := &mozi.ModelIR{Name: "Deck"}
	rel := mozi.RelationIR{
		Name:        "cards",
		Label:       "包含",
		Type:        mozi.RelationHasMany,
		TargetModel: "Card",
	}

	got := mermaidRelation(model, rel)
	want := "Deck ||--o{ Card : 包含"
	if got != want {
		t.Fatalf("mermaid relation = %q, want %q", got, want)
	}
}

func TestMermaidRelationFallsBackToRelationTypeLabel(t *testing.T) {
	model := &mozi.ModelIR{Name: "Card"}
	rel := mozi.RelationIR{
		Name:        "deck",
		Type:        mozi.RelationBelongsTo,
		TargetModel: "Deck",
	}

	got := mermaidRelation(model, rel)
	want := "Card }o--|| Deck : 属于"
	if got != want {
		t.Fatalf("mermaid relation = %q, want %q", got, want)
	}
}
