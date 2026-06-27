// Package apply plans and writes mozi-generated code.
package apply

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/generator"
)

// File represents one generated file in a plan.
type File struct {
	Path    string
	AbsPath string
	Content string
	Model   string
}

// WriteResult reports the outcome of writing one generated file.
type WriteResult struct {
	Path       string `json:"path"`
	Action     string `json:"action"`
	HashBefore string `json:"hash_before,omitempty"`
	HashAfter  string `json:"hash_after,omitempty"`
}

// FindProjectRoot finds the business project root from MOZI_PROJECT_ROOT or cwd.
func FindProjectRoot() (string, error) {
	if root := os.Getenv("MOZI_PROJECT_ROOT"); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if fileExists(filepath.Join(abs, "go.mod")) && dirExists(filepath.Join(abs, "models")) {
			return abs, nil
		}
		return "", fmt.Errorf("project root %s must contain go.mod and models/", abs)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && dirExists(filepath.Join(dir, "models")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root not found")
		}
		dir = parent
	}
}

// PlanModel generates all files for a model without writing them.
func PlanModel(engine *generator.Engine, model *mozi.ModelIR, mod *mozi.ModuleIR, project *mozi.ProjectIR, projectRoot, target string) ([]File, error) {
	if target == "" {
		target = "all"
	}
	if target != "all" && target != "backend" && target != "frontend" {
		return nil, fmt.Errorf("invalid target: %s", target)
	}

	ctx := generator.BuildContextWithModule(model, mod, project)
	var files []File

	add := func(relPath, content string) {
		files = append(files, File{
			Path:    filepath.ToSlash(relPath),
			AbsPath: filepath.Join(projectRoot, filepath.FromSlash(relPath)),
			Content: content,
			Model:   model.Name,
		})
	}

	if target == "all" || target == "backend" {
		content, err := engine.ExecuteContext("backend/schema.go.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("schema: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("ent/schema", generator.EntSchemaFileName(mod.Name, model.Name))), content)

		content, err = engine.ExecuteContext("backend/model.go.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("model: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("internal/model", mod.Name, snake(model.Name)+".go")), content)

		content, err = engine.ExecuteContext("backend/handler.go.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("handler: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("internal/handler", mod.Name, snake(model.Name)+".go")), content)

		content, err = engine.ExecuteContext("backend/service.go.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("service: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("internal/service", mod.Name, snake(model.Name)+".go")), content)
	}

	if target == "all" || target == "frontend" {
		content, err := engine.ExecuteContext("frontend/list.tsx.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("list: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("admin/src/pages", mod.Name, model.Name+"List.tsx")), content)

		content, err = engine.ExecuteContext("frontend/form.tsx.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("form: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("admin/src/pages", mod.Name, model.Name+"Form.tsx")), content)

		content, err = engine.ExecuteContext("frontend/api.ts.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("api: %w", err)
		}
		apiRel := filepath.ToSlash(filepath.Join("admin/src/api", mod.Name+".ts"))
		existingAPI, _ := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(apiRel)))
		add(apiRel, appendSnippet(string(existingAPI), content, "mozi:api "+model.Name))

		content, err = engine.ExecuteContext("frontend/store.ts.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("store: %w", err)
		}
		add(filepath.ToSlash(filepath.Join("admin/src/stores", mod.Name, ctx.NameCamel+".ts")), content)

		content, err = engine.ExecuteContext("frontend/app.tsx.tmpl", ctx)
		if err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
		appRel := "admin/src/App.tsx"
		existingApp, _ := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(appRel)))
		add(appRel, mergeAppTSX(string(existingApp), mod.Name+"/"+model.Name, content))
	}

	return files, nil
}

// Write writes a generated plan to disk.
func Write(files []File) ([]WriteResult, error) {
	results := make([]WriteResult, 0, len(files))
	for _, f := range files {
		before, existed, err := readExisting(f.AbsPath)
		if err != nil {
			return nil, err
		}

		action := "created"
		if existed {
			if string(before) == f.Content {
				action = "skipped"
			} else {
				action = "updated"
			}
		}

		if action != "skipped" {
			if err := os.MkdirAll(filepath.Dir(f.AbsPath), 0755); err != nil {
				return nil, fmt.Errorf("create dir %s: %w", filepath.Dir(f.AbsPath), err)
			}
			if err := os.WriteFile(f.AbsPath, []byte(f.Content), 0644); err != nil {
				return nil, fmt.Errorf("write %s: %w", f.AbsPath, err)
			}
		}

		result := WriteResult{
			Path:      f.Path,
			Action:    action,
			HashAfter: hashString(f.Content),
		}
		if existed {
			result.HashBefore = hashBytes(before)
		}
		results = append(results, result)
	}
	return results, nil
}

func readExisting(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, nil
}

func hashString(s string) string {
	return hashBytes([]byte(s))
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func snake(s string) string {
	var r strings.Builder
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				r.WriteByte('_')
			}
			r.WriteRune(c + 32)
		} else {
			r.WriteRune(c)
		}
	}
	return r.String()
}

func appendSnippet(existing, snippet, marker string) string {
	if strings.Contains(existing, marker) {
		return existing
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + snippet + "\n"
}

func mergeAppTSX(existing, ref, snippet string) string {
	routeMarker := "mozi:route " + ref
	if strings.Contains(existing, routeMarker) {
		return existing
	}
	importStart := "// mozi:import " + ref
	importEnd := "// mozi:end import " + ref
	importLines := extractSection(snippet, importStart, importEnd)
	routeStart := "{/* mozi:route " + ref
	routeEnd := "{/* mozi:end route " + ref + " */}"
	routeLines := extractSection(snippet, routeStart, routeEnd)

	if importLines != "" {
		lines := strings.Split(existing, "\n")
		lastImport := -1
		for i, line := range lines {
			if strings.Contains(line, "import ") && strings.Contains(line, "from ") {
				lastImport = i
			}
		}
		if lastImport >= 0 {
			var newLines []string
			newLines = append(newLines, lines[:lastImport+1]...)
			newLines = append(newLines, importLines)
			newLines = append(newLines, lines[lastImport+1:]...)
			existing = strings.Join(newLines, "\n")
		}
	}

	if routeLines != "" {
		search := `<Route path="*" element={<Navigate to="/" replace />} />`
		if strings.Contains(existing, search) {
			existing = strings.Replace(existing, search, routeLines+"\n"+search, 1)
		}
	}

	return existing
}

func extractSection(content, startMarker, endMarker string) string {
	si := strings.Index(content, startMarker)
	if si < 0 {
		return ""
	}
	ei := strings.Index(content[si:], endMarker)
	if ei < 0 {
		return ""
	}
	return content[si : si+ei+len(endMarker)]
}
