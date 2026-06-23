// Package generator provides the template-based code generation engine.
// The engine is generic — it knows nothing about ent, gin, or react.
// Project-specific knowledge lives in the templates that are injected via fs.FS.
package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"strings"
	"text/template"

	"github.com/pangu-sutido/mozi-builder/mozi"
)

// Engine executes code generation templates against model IRs.
type Engine struct {
	templateFS fs.FS
	funcMap    template.FuncMap
}

// NewEngine creates a new generator engine with the given template filesystem.
func NewEngine(templateFS fs.FS) *Engine {
	return &Engine{
		templateFS: templateFS,
		funcMap:    defaultFuncMap(),
	}
}

// NewEngineWithFuncs creates a new generator engine with custom template functions.
func NewEngineWithFuncs(templateFS fs.FS, funcMap template.FuncMap) *Engine {
	merged := defaultFuncMap()
	maps.Copy(merged, funcMap)
	return &Engine{
		templateFS: templateFS,
		funcMap:    merged,
	}
}

// Execute runs a single template against the model and returns the generated code.
// templateName is relative to the templateFS root (e.g., "backend/schema.go.tmpl").
// Uses BuildContext to create a basic context from the model.
func (e *Engine) Execute(templateName string, model *mozi.ModelIR) (string, error) {
	return e.ExecuteContext(templateName, BuildContext(model))
}

// ExecuteContext runs a single template against a pre-built TemplateContext.
// Use this when the context needs module or project information.
func (e *Engine) ExecuteContext(templateName string, ctx *TemplateContext) (string, error) {
	tmplContent, err := fs.ReadFile(e.templateFS, templateName)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templateName, err)
	}

	tmpl, err := template.New(templateName).Delims("[[", "]]").Funcs(e.funcMap).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", templateName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// ExecuteToPath executes a template and also tracks it as a generated file path.
func (e *Engine) ExecuteToPath(templateName string, model *mozi.ModelIR, outputPath string) (*GeneratedFile, error) {
	content, err := e.Execute(templateName, model)
	if err != nil {
		return nil, err
	}

	return &GeneratedFile{
		Path:    outputPath,
		Content: content,
		Model:   model.Name,
	}, nil
}

// GeneratedFile represents a file produced by code generation.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Model   string `json:"model"`
}

// defaultFuncMap returns the standard Go template helper functions.
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"title": func(s string) string {
			if s == "" {
				return ""
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"trim":                strings.TrimSpace,
		"contains":            strings.Contains,
		"hasPrefix":           strings.HasPrefix,
		"hasSuffix":           strings.HasSuffix,
		"replace":             strings.ReplaceAll,
		"join":                strings.Join,
		"snakeCase":           moziToSnake,
		"camelCase":           moziToCamel,
		"kebabCase":           moziToKebab,
		"snakeToPascal":       snakeToPascal,
		"snakeToCamel":        snakeToCamel,
		"add":                 func(a, b int) int { return a + b },
		"sub":                 func(a, b int) int { return a - b },
		"GoTypeForField":      GoTypeForField,
		"TSTypeForField":      TSTypeForField,
		"AdminServiceName":    AdminServiceName,
		"AdminHandlerName":    AdminHandlerName,
		"OutputFileName":      OutputFileName,
		"AdminOutputFileName": AdminOutputFileName,
		"hasDefault":          func(f mozi.FieldIR) bool { return f.Default != nil },
		"defaultVal": func(f mozi.FieldIR) string {
			if f.Default != nil {
				return *f.Default
			}
			return ""
		},
	}
}

// Case conversion helpers

func moziToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func moziToCamel(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func moziToKebab(s string) string {
	snake := moziToSnake(s)
	return strings.ReplaceAll(snake, "_", "-")
}

// snakeToPascal converts snake_case to PascalCase.
// E.g., "created_at" → "CreatedAt", "wechat_openid" → "WechatOpenid"
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// snakeToCamel converts snake_case to camelCase.
// E.g., "created_at" → "createdAt", "wechat_openid" → "wechatOpenid"
func snakeToCamel(s string) string {
	pascal := snakeToPascal(s)
	if pascal == "" {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}
