package devplatform

import (
	"testing"

	"memflow/mozi-builder/mozi"
)

func TestFilterProjectByModuleKeepsOnlyModuleModelsAndInternalRelations(t *testing.T) {
	project := &mozi.ProjectIR{
		Modules: []*mozi.ModuleIR{
			{
				Name: "content",
				Models: []*mozi.ModelIR{
					{
						Name: "Deck",
						Relations: []mozi.RelationIR{
							{Name: "cards", TargetModel: "Card"},
							{Name: "user", TargetModel: "User"},
						},
					},
					{Name: "Card"},
				},
			},
			{
				Name: "user",
				Models: []*mozi.ModelIR{
					{
						Name: "User",
						Relations: []mozi.RelationIR{
							{Name: "decks", TargetModel: "Deck"},
						},
					},
				},
			},
		},
	}

	filtered := filterProjectByModule(project, "content")

	if len(filtered.Modules) != 1 {
		t.Fatalf("expected exactly one module, got %d", len(filtered.Modules))
	}
	if filtered.Modules[0].Name != "content" {
		t.Fatalf("expected content module, got %q", filtered.Modules[0].Name)
	}
	if len(filtered.Modules[0].Models) != 2 {
		t.Fatalf("expected only content models, got %d", len(filtered.Modules[0].Models))
	}

	deck := filtered.Modules[0].Models[0]
	if len(deck.Relations) != 1 {
		t.Fatalf("expected one internal relation, got %d", len(deck.Relations))
	}
	if deck.Relations[0].Name != "cards" {
		t.Fatalf("expected internal cards relation, got %q", deck.Relations[0].Name)
	}

	if len(project.Modules[0].Models[0].Relations) != 2 {
		t.Fatal("filterProjectByModule must not mutate the source project")
	}
}
