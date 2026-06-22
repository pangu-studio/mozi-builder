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
