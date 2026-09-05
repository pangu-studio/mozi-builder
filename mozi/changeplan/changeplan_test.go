package changeplan

import (
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
)

func fixtureModel() *mozi.ModelIR {
	return &mozi.ModelIR{
		Module: "content",
		Name:   "Deck",
		Label:  "牌组",
		Table:  "decks",
		Fields: []mozi.FieldIR{
			{Name: "id", Type: mozi.FieldTypeString, Label: "ID", Primary: true, Generated: mozi.GeneratedUUID},
			{Name: "title", Type: mozi.FieldTypeString, Label: "标题", Required: true},
		},
		Semantics: mozi.SemanticConfig{Purpose: "组织卡片"},
	}
}

func TestBuildPendingWithFieldAddition(t *testing.T) {
	from := fixtureModel()
	to := fixtureModel()
	to.Fields = append(to.Fields, mozi.FieldIR{Name: "due_at", Type: mozi.FieldTypeTime, Label: "到期"})
	diff := differ.Compare(from, to, "v1", "v2")

	result := Build(Input{
		Model:         to,
		Previous:      from,
		Diff:          diff,
		AffectedFiles: diff.AffectedFiles(),
		Status:        Pending,
		ModuleIcon:    "folder",
	})

	if result.ModelRef != "content/Deck" {
		t.Fatalf("model ref: %s", result.ModelRef)
	}
	if !strings.Contains(result.Intent, "1 additions") {
		t.Fatalf("intent should count additions: %s", result.Intent)
	}
	foundBackend := false
	for _, task := range result.Tasks {
		if task.Area == "backend" {
			foundBackend = true
		}
	}
	if !foundBackend {
		t.Fatalf("expected backend task, got %v", result.Tasks)
	}
	joined := strings.Join(result.Checks, "\n")
	if !strings.Contains(joined, "mozi diff --model content/Deck") {
		t.Fatalf("checks should include diff: %s", joined)
	}
	if !strings.HasPrefix(result.Prompt, "Change plan for content/Deck [status: pending]") {
		t.Fatalf("prompt header: %.80s", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "Purpose: 组织卡片") {
		t.Fatal("prompt should carry semantics")
	}
	if !strings.Contains(result.Prompt, "Module icon: folder") {
		t.Fatal("prompt should carry module icon")
	}
}

func TestBuildAppliedAndNoDiff(t *testing.T) {
	diff := differ.Compare(fixtureModel(), fixtureModel(), "v1", "v1")
	if diff.HasChanges {
		t.Fatal("identical models must not produce changes")
	}
	applied := Build(Input{Model: fixtureModel(), Diff: diff, Status: Applied})
	if applied.Contracts[0] != "No pending model changes — skip code generation." {
		t.Fatalf("applied contracts: %v", applied.Contracts)
	}
	if applied.RequiresApproval {
		t.Fatal("no changes must not require approval")
	}
	noDiff := Build(Input{Model: fixtureModel(), Diff: diff, Status: NoDiff})
	if !strings.Contains(noDiff.Intent, "No model changes") {
		t.Fatalf("no_diff intent: %s", noDiff.Intent)
	}
}

func TestBuildBreakingRequiresApproval(t *testing.T) {
	from := fixtureModel()
	to := fixtureModel()
	to.Fields = to.Fields[:1] // remove title
	diff := differ.Compare(from, to, "v1", "v2")
	result := Build(Input{Model: to, Previous: from, Diff: diff, Status: Pending})
	if !result.RequiresApproval {
		t.Fatal("field removal is breaking and must require approval")
	}
}
