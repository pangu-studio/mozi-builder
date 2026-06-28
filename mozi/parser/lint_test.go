package parser

import (
	"github.com/pangu-studio/mozi-builder/mozi"
	"testing"
)

func TestLintProjectFindsOrphanRelation(t *testing.T) {
	project := &mozi.ProjectIR{SchemaVersion: 1, Modules: []*mozi.ModuleIR{{Name: "content", Models: []*mozi.ModelIR{{
		SchemaVersion: 1, Module: "content", Name: "Deck", Fields: []mozi.FieldIR{{Name: "id", Primary: true}},
		Relations: []mozi.RelationIR{{Name: "cards", TargetModule: "content", TargetModel: "Card"}},
	}}}}}
	got := LintProject(project, LintOptions{})
	if got.Valid || !hasLintCode(got, "orphan-relation") {
		t.Fatalf("issues = %#v", got.Issues)
	}
}

func TestLintProjectStrictPromotesWarnings(t *testing.T) {
	project := &mozi.ProjectIR{SchemaVersion: 1, Modules: []*mozi.ModuleIR{{Name: "content", Models: []*mozi.ModelIR{{
		SchemaVersion: 1, Module: "content", Name: "Deck", Fields: []mozi.FieldIR{{Name: "id", Label: "ID", Primary: true}},
	}}}}}
	if !LintProject(project, LintOptions{}).Valid {
		t.Fatal("warnings should not fail default lint")
	}
	if LintProject(project, LintOptions{Strict: true}).Valid {
		t.Fatal("strict lint should promote warnings")
	}
}

func TestRepositoryModelsPassDefaultLint(t *testing.T) {
	project, err := ParseProject("../../models")
	if err != nil {
		t.Fatal(err)
	}
	result := LintProject(project, LintOptions{})
	if !result.Valid {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func hasLintCode(result *LintResult, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
