package i18n

import (
	"github.com/pangu-studio/mozi-builder/mozi"
	"testing"
)

func TestExtractAndValidate(t *testing.T) {
	p := &mozi.ProjectIR{Modules: []*mozi.ModuleIR{{Name: "content", Models: []*mozi.ModelIR{{Module: "content", Name: "Deck", Label: "牌组", Fields: []mozi.FieldIR{{Name: "name", Label: "名称"}}}}}}}
	c := Extract(p, "zh-CN")
	if len(c.Entries) != 2 {
		t.Fatalf("entries=%#v", c.Entries)
	}
	v := Validate(c, map[string]string{})
	if len(v.Missing) != 2 {
		t.Fatalf("validation=%#v", v)
	}
}

func TestValidateAllowsPlaceholderReordering(t *testing.T) {
	c := Catalog{Entries: []Entry{{Key: "message", Source: "{first} then {second}"}}}
	v := Validate(c, map[string]string{"message": "{second} after {first}"})
	if len(v.PlaceholderErrors) != 0 {
		t.Fatalf("validation=%#v", v)
	}
}
