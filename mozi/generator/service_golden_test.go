package generator

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// serviceTemplateFS roots the engine at the embedded templates directory,
// matching how devplatform and apply consume mozi.EmbeddedTemplates.
func serviceTemplateFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(mozi.EmbeddedTemplates, "templates")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

// goldenService is the reference fixture mirrored from docs/v2/service-ir.md.
func goldenService() *mozi.ServiceIR {
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
			{
				Name: "GetDeckRequest",
				Fields: []mozi.MessageFieldIR{
					{Name: "id", Type: "string", Number: 1},
				},
			},
		},
		HTTP: []mozi.HTTPRouteIR{
			{Name: "ListDecks", Method: "GET", Path: "/api/content/decks", Response: "DeckSummary", Auth: "jwt"},
			{Name: "CreateDeck", Method: "POST", Path: "/api/content/decks", Request: "CreateDeckRequest", Response: "DeckSummary", Auth: "jwt"},
			{Name: "DeckHealth", Method: "GET", Path: "/api/content/health", Response: "DeckSummary", Auth: "public"},
		},
		RPC: []mozi.RPCMethodIR{
			{Name: "GetDeck", Request: "GetDeckRequest", Response: "DeckSummary"},
		},
	}
}

func renderGolden(t *testing.T, templateName, goldenName string) {
	t.Helper()
	engine := NewEngine(serviceTemplateFS(t))
	out, err := engine.ExecuteService(templateName, goldenService())
	if err != nil {
		t.Fatalf("render %s: %v", templateName, err)
	}
	path := filepath.Join("testdata", goldenName)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if out != string(want) {
		t.Errorf("rendered %s differs from golden %s (run with -update to refresh)\n--- got ---\n%s", templateName, goldenName, out)
	}
}

func TestServiceAPIGolden(t *testing.T) {
	renderGolden(t, "service/api.tmpl", "service.api.golden")
}

func TestServiceHandlerGolden(t *testing.T) {
	renderGolden(t, "service/handler.go.tmpl", "service_handler.go.golden")
}

func TestServiceTypesGolden(t *testing.T) {
	renderGolden(t, "service/types.go.tmpl", "service_types.go.golden")
}

func TestServiceProtoGolden(t *testing.T) {
	renderGolden(t, "service/proto.tmpl", "service.proto.golden")
}

func TestServiceHandlerIncrementalPreservesHandwrittenCode(t *testing.T) {
	engine := NewEngine(serviceTemplateFS(t))
	out, err := engine.ExecuteService("service/handler.go.tmpl", goldenService())
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a developer implementing the business logic below the marker.
	handwritten := "result := loadDecks()\n\tlog.Printf(\"served %d decks\", len(result))"
	marker := "// TODO: implement CreateDeck business logic here. This region is\n\t// handwritten and preserved across regeneration.\n\thttpx.OkJson(w, nil)"
	idx := strings.Index(out, marker)
	if idx < 0 {
		t.Fatal("handwritten TODO region not found in generated handler")
	}
	patched := out[:idx] + handwritten + out[idx+len(marker):]
	// Regenerate from an updated service (new field added) and splice the
	// plumbing sections into the patched file; the handwritten code must stay.
	updated := goldenService()
	updated.Messages[1].Fields = append(updated.Messages[1].Fields,
		mozi.MessageFieldIR{Name: "description", Type: "text", Number: 3})
	regen, err := engine.ExecuteService("service/handler.go.tmpl", updated)
	if err != nil {
		t.Fatal(err)
	}
	merged := patched
	for _, s := range ExtractMarkerSections(regen) {
		merged = ReplaceMarkerSection(merged, s.Name, "section", s.Content)
	}
	if !strings.Contains(merged, handwritten) {
		t.Fatal("handwritten business logic lost after regeneration")
	}
	if !strings.Contains(merged, "var req CreateDeckRequest") {
		t.Fatal("regenerated plumbing missing after merge")
	}
}
