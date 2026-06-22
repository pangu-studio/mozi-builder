package devplatform

import (
	"testing"

	"memflow/mozi-builder/mozi"
)

func TestInferSurfaceFromPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want APISurface
	}{
		{name: "admin", path: "/api/admin/users", want: APISurfaceAdmin},
		{name: "miniapp", path: "/miniapp-api/v1/wechat/session", want: APISurfaceMiniApp},
		{name: "desktop", path: "/desktop-api/v1/devices", want: APISurfaceDesktop},
		{name: "client shared", path: "/client-api/v1/profile", want: APISurfaceClientShared},
		{name: "dev platform", path: "/api/dev-platform/apis", want: APISurfaceInternal},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := inferSurface(tt.path, nil, nil)
			if got != tt.want {
				t.Fatalf("inferSurface(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestInferEndpointModuleUsesPathTagAndSchema(t *testing.T) {
	lookup := buildModuleLookup(&mozi.ProjectIR{
		Modules: []*mozi.ModuleIR{
			{
				Name:      "content",
				Label:     "内容",
				APIPrefix: "content",
				Models: []*mozi.ModelIR{
					{Name: "Deck"},
				},
			},
		},
	})

	module, label := inferEndpointModule("/api/admin/content/decks", nil, nil, lookup)
	if module != "content" || label != "内容" {
		t.Fatalf("path module = %q/%q, want content/内容", module, label)
	}

	module, label = inferEndpointModule("/api/cards", []string{"内容"}, nil, lookup)
	if module != "content" || label != "内容" {
		t.Fatalf("tag module = %q/%q, want content/内容", module, label)
	}

	module, label = inferEndpointModule("/api/decks", nil, []string{"ent.Deck"}, lookup)
	if module != "content" || label != "内容" {
		t.Fatalf("schema module = %q/%q, want content/内容", module, label)
	}
}

func TestInferBusinessModelsFromSchemas(t *testing.T) {
	assets := map[string]*APISchemaAsset{
		"ent.Deck":            {Name: "ent.Deck", Module: "content", Model: "Deck"},
		"model.CreateDeckReq": {Name: "model.CreateDeckReq", Module: "content", Model: "CreateDeck"},
	}

	got := inferBusinessModels([]string{"ent.Deck", "model.CreateDeckReq"}, "content", assets)
	if len(got) != 2 {
		t.Fatalf("expected 2 business models, got %d: %#v", len(got), got)
	}
	if got[0] != "content/CreateDeck" || got[1] != "content/Deck" {
		t.Fatalf("unexpected business models: %#v", got)
	}
}
