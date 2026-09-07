package example

import (
	"io/fs"
	"os"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/generator"
)

// exampleService is the fixture mirrored from docs/v2/service-ir.md.
func exampleService() *mozi.ServiceIR {
	return &mozi.ServiceIR{
		SchemaVersion: 1,
		Module:        "content",
		Name:          "ContentService",
		Label:         "内容服务",
		Description:   "牌组与卡片的读写契约",
		Domain:        "content",
		Messages: []mozi.MessageIR{
			{
				Name: "DeckSummary",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
					{Name: "title", Type: "string", Number: 2},
					{Name: "due_at", Type: "time", Number: 3},
				},
				ReservedNumbers: []int32{4},
				ReservedNames:   []string{"review_count"},
			},
			{
				Name: "CreateDeckRequest",
				Fields: []mozi.MessageFieldIR{
					{Name: "title", Type: "string", Number: 1},
					{Name: "tags", Type: "string", Number: 2, Repeated: true},
				},
			},
		},
		HTTP: []mozi.HTTPRouteIR{
			{Name: "ListDecks", Method: "GET", Path: "/api/content/decks", Response: "DeckSummary", Auth: "jwt"},
			{Name: "CreateDeck", Method: "POST", Path: "/api/content/decks", Request: "CreateDeckRequest", Response: "DeckSummary", Auth: "jwt"},
			{Name: "DeckHealth", Method: "GET", Path: "/api/content/health", Response: "DeckSummary", Auth: "public"},
		},
	}
}

// renderExample renders one template against the fixture with the package
// overridden to this directory's package name.
func renderExample(t *testing.T, templateName string) string {
	t.Helper()
	sub, err := fs.Sub(mozi.EmbeddedTemplates, "templates")
	if err != nil {
		t.Fatal(err)
	}
	engine := generator.NewEngine(sub)
	ctx := generator.BuildServiceContext(exampleService())
	ctx.Package = "example"
	out, err := engine.ExecuteServiceContext(templateName, ctx)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTypesMatchRender(t *testing.T) {
	want := renderExample(t, "service/types.go.tmpl")
	got, err := os.ReadFile("types_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("types_gen.go stale; regenerate with MOZI_REGEN_EXAMPLE=1")
	}
}

// TestHandlerMarkersMatchRender splices freshly rendered marker sections into
// the committed handler and requires no change: regeneration is idempotent
// and handwritten business logic outside the markers is preserved.
func TestHandlerMarkersMatchRender(t *testing.T) {
	fresh := renderExample(t, "service/handler.go.tmpl")
	committed, err := os.ReadFile("handler_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	merged := string(committed)
	for _, s := range generator.ExtractMarkerSections(fresh) {
		merged = generator.ReplaceMarkerSection(merged, s.Name, "section", s.Content)
	}
	if merged != string(committed) {
		t.Fatal("handler_gen.go marker sections stale; regenerate with MOZI_REGEN_EXAMPLE=1")
	}
}

// TestRegenerate rewrites the committed artifacts: types wholesale, handler
// via marker splice so handwritten logic survives. Run explicitly:
//
//	MOZI_REGEN_EXAMPLE=1 go test ./internal/gen/example/ -run TestRegenerate
func TestRegenerate(t *testing.T) {
	if os.Getenv("MOZI_REGEN_EXAMPLE") != "1" {
		t.Skip("set MOZI_REGEN_EXAMPLE=1 to regenerate example artifacts")
	}
	if err := os.WriteFile("types_gen.go", []byte(renderExample(t, "service/types.go.tmpl")), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := renderExample(t, "service/handler.go.tmpl")
	existing, err := os.ReadFile("handler_gen.go")
	if err != nil {
		existing = []byte(fresh)
	}
	merged := string(existing)
	for _, s := range generator.ExtractMarkerSections(fresh) {
		merged = generator.ReplaceMarkerSection(merged, s.Name, "section", s.Content)
	}
	if merged != string(existing) {
		if err := os.WriteFile("handler_gen.go", []byte(merged), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
