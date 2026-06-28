package manifest

import "testing"

func TestRecordGenWithMetadata(t *testing.T) {
	m := &Manifest{Version: 2, Models: map[string]ModelGenInfo{}}
	m.RecordGenWithMetadata("content/Deck", "v2", "0.2.0", "templates-v1", map[string][]byte{"deck.go": []byte("package deck")}, "generated")
	info := m.GetGenInfo("content/Deck")
	if info.GeneratorVersion != "0.2.0" || info.TemplateVersion != "templates-v1" {
		t.Fatalf("info = %#v", info)
	}
	if info.Files["deck.go"].Hash == "" || info.Files["deck.go"].Ownership != "generated" {
		t.Fatalf("file = %#v", info.Files["deck.go"])
	}
}
