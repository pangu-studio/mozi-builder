package apply

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/generator"
)

func TestPlanModelGeneratesZustandStore(t *testing.T) {
	projectRoot := t.TempDir()
	mustWrite(t, filepath.Join(projectRoot, "admin/src/api/content.ts"), "import api from './index'\n")
	mustWrite(t, filepath.Join(projectRoot, "admin/src/App.tsx"), `import React from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'

const App = () => (
  <Routes>
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>
)

export default App
`)

	model := &mozi.ModelIR{
		Module: "content",
		Name:   "Deck",
		Label:  "牌组",
		Table:  "decks",
		Fields: []mozi.FieldIR{
			{Name: "id", Type: mozi.FieldTypeString, Label: "ID", Primary: true},
			{Name: "name", Type: mozi.FieldTypeString, Label: "名称", Required: true, Searchable: true, Listable: true, Editable: true, FormType: "text"},
		},
		Admin: mozi.AdminConfig{PageSize: 20, SearchFields: []string{"name"}},
	}
	mod := &mozi.ModuleIR{Name: "content", Label: "内容管理", APIPrefix: "content"}
	project := &mozi.ProjectIR{Backend: mozi.BackendConfig{Package: "memflow/cloud"}}
	tmplFS, err := fs.Sub(mozi.EmbeddedTemplates, "templates")
	if err != nil {
		t.Fatal(err)
	}
	engine := generator.NewEngine(tmplFS)

	files, err := PlanModel(engine, model, mod, project, projectRoot, "frontend")
	if err != nil {
		t.Fatalf("PlanModel() error = %v", err)
	}

	var listContent, formContent, storeContent string
	for _, file := range files {
		switch file.Path {
		case "admin/src/pages/content/DeckList.tsx":
			listContent = file.Content
		case "admin/src/pages/content/DeckForm.tsx":
			formContent = file.Content
		case "admin/src/stores/content/deck.ts":
			storeContent = file.Content
		}
	}

	if storeContent == "" {
		t.Fatalf("store file was not generated; files: %#v", files)
	}
	for _, want := range []string{
		"import { create } from 'zustand'",
		"export const useDeckStore = create<DeckState>",
		"loadList: async () =>",
	} {
		if !strings.Contains(storeContent, want) {
			t.Fatalf("store content missing %q:\n%s", want, storeContent)
		}
	}
	if !strings.Contains(listContent, "useDeckStore") {
		t.Fatalf("list page does not use generated store:\n%s", listContent)
	}
	if !strings.Contains(formContent, "useDeckStore") {
		t.Fatalf("form page does not use generated store:\n%s", formContent)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
