package generator

import (
	"strings"
	"testing"
)

const markerFixture = `package handler

// handwritten header stays untouched

// mozi:section ListDecksRequest — generated plumbing
oldGenerated()
// mozi:end ListDecksRequest

// handwritten footer stays untouched
`

func TestExtractMarkerSections(t *testing.T) {
	sections := ExtractMarkerSections(markerFixture)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Name != "ListDecksRequest" || strings.TrimSpace(sections[0].Content) != "oldGenerated()" {
		t.Fatalf("unexpected section: %+v", sections[0])
	}
}

func TestReplaceMarkerSectionPreservesSurroundings(t *testing.T) {
	out := ReplaceMarkerSection(markerFixture, "ListDecksRequest", "section", "newGenerated()\nmoreGenerated()")
	if !strings.Contains(out, "newGenerated()\nmoreGenerated()") {
		t.Fatal("new content not spliced in")
	}
	if strings.Contains(out, "oldGenerated()") {
		t.Fatal("old generated content should be replaced")
	}
	for _, keep := range []string{"// handwritten header stays untouched", "// handwritten footer stays untouched"} {
		if !strings.Contains(out, keep) {
			t.Fatalf("handwritten content lost: %q", keep)
		}
	}
	if strings.Count(out, "mozi:section ListDecksRequest") != 1 || strings.Count(out, "mozi:end ListDecksRequest") != 1 {
		t.Fatal("markers must be preserved exactly once")
	}
}

func TestReplaceMarkerSectionAppendsMissing(t *testing.T) {
	out := ReplaceMarkerSection(markerFixture, "CreateDeckRequest", "section", "created()")
	if !strings.Contains(out, "// mozi:section CreateDeckRequest") || !strings.Contains(out, "created()") {
		t.Fatal("missing section should be appended with markers")
	}
	if !strings.Contains(out, "oldGenerated()") {
		t.Fatal("existing sections must stay intact when appending")
	}
}
