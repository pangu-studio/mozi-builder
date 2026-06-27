package differ

import (
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// TestSummaryMixedChanges verifies Summary() aggregates added/removed/modified
// counts and per-category tallies correctly. This is the data the per-version
// "+N -N ~N" badges in the history view render from.
func TestSummaryMixedChanges(t *testing.T) {
	from := &mozi.ModelIR{
		Module: "content", Name: "Card",
		Fields: []mozi.FieldIR{
			{Name: "front", Type: mozi.FieldTypeText, Label: "正面"},  // -> modified (label)
			{Name: "extra", Type: mozi.FieldTypeString, Label: "额外"}, // -> removed
		},
	}
	to := &mozi.ModelIR{
		Module: "content", Name: "Card",
		Fields: []mozi.FieldIR{
			{Name: "front", Type: mozi.FieldTypeText, Label: "正面内容"}, // modified
			{Name: "back", Type: mozi.FieldTypeText, Label: "背面"},      // added
		},
	}

	summary := Compare(from, to, "v1", "v2").Summary()

	if !summary.HasChanges {
		t.Fatal("expected HasChanges=true")
	}
	if summary.FromVersion != "v1" || summary.ToVersion != "v2" {
		t.Fatalf("version wiring wrong: from=%q to=%q", summary.FromVersion, summary.ToVersion)
	}
	if summary.Counts["added"] != 1 {
		t.Errorf("added count = %d, want 1", summary.Counts["added"])
	}
	if summary.Counts["removed"] != 1 {
		t.Errorf("removed count = %d, want 1", summary.Counts["removed"])
	}
	if summary.Counts["modified"] != 1 {
		t.Errorf("modified count = %d, want 1", summary.Counts["modified"])
	}
	if summary.ByCategory["field"] != 3 {
		t.Errorf("field category tally = %d, want 3", summary.ByCategory["field"])
	}
	if len(summary.Changes) != 3 {
		t.Errorf("change count = %d, want 3", len(summary.Changes))
	}
}

// TestSummaryFirstVersion mirrors the empty-predecessor edge case the service
// uses for a model's first version: everything shows as added, FromVersion is empty.
func TestSummaryFirstVersion(t *testing.T) {
	empty := &mozi.ModelIR{Module: "content", Name: "Card"}
	current := &mozi.ModelIR{
		Module: "content", Name: "Card",
		Fields: []mozi.FieldIR{
			{Name: "id", Type: mozi.FieldTypeString},
			{Name: "front", Type: mozi.FieldTypeText},
			{Name: "back", Type: mozi.FieldTypeText},
		},
	}

	summary := Compare(empty, current, "", "v1").Summary()

	if !summary.HasChanges {
		t.Fatal("first version should report all fields as added")
	}
	if summary.Counts["added"] != 3 {
		t.Errorf("added count = %d, want 3", summary.Counts["added"])
	}
	if summary.Counts["removed"] != 0 || summary.Counts["modified"] != 0 {
		t.Errorf("first version should have no removed/modified, got removed=%d modified=%d",
			summary.Counts["removed"], summary.Counts["modified"])
	}
	if summary.FromVersion != "" {
		t.Errorf("first version FromVersion = %q, want empty", summary.FromVersion)
	}
}

// TestSummaryNoChanges verifies a no-op version yields HasChanges=false and zero
// counts, which the UI renders as the "无变更" badge and a non-expandable row.
func TestSummaryNoChanges(t *testing.T) {
	m := &mozi.ModelIR{
		Module: "content", Name: "Card",
		Fields: []mozi.FieldIR{{Name: "front", Type: mozi.FieldTypeText, Label: "正面"}},
	}
	summary := Compare(m, m, "v1", "v2").Summary()

	if summary.HasChanges {
		t.Fatal("expected HasChanges=false")
	}
	if len(summary.Changes) != 0 {
		t.Errorf("change count = %d, want 0", len(summary.Changes))
	}
	for _, k := range []string{"added", "removed", "modified"} {
		if summary.Counts[k] != 0 {
			t.Errorf("%s count = %d, want 0", k, summary.Counts[k])
		}
	}
}
