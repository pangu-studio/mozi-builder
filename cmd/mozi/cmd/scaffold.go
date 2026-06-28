package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed all:scaffold_templates
var scaffoldFS embed.FS

// scaffoldVars holds the template variables for project scaffolding.
type scaffoldVars struct {
	Name            string // project name, e.g. "myapp"
	Module          string // Go module path, e.g. "github.com/foo/myapp"
	ModuleBase      string // last path segment of Module, e.g. "myapp"
	UIDir           string // web frontend dir, e.g. "myapp-ui"
	DesktopDir      string // Tauri desktop dir, e.g. "myapp-desktop"
	MiniappDir      string // WeChat mini-program dir, e.g. "myapp-miniapp"
	EnvPrefix       string // uppercased sanitized Name, e.g. "MYAPP"
	AddrEnv         string // e.g. "MYAPP_ADDR"
	DBDsnEnv        string // e.g. "MYAPP_DB_DSN"
	DevPlatformEnv  string // e.g. "MYAPP_DEV_PLATFORM"
	AdminPathEnv    string // e.g. "MYAPP_ADMIN_PATH"
	BuilderTokenEnv string // e.g. "MYAPP_BUILDER_TOKEN"
	TauriIdent      string // reverse-dns-ish desktop identifier
}

// scaffoldOpts controls which optional clients to scaffold.
type scaffoldOpts struct {
	Desktop bool
	Miniapp bool
	Force   bool
}

// sanitizeEnvPrefix converts a project name to a valid env-var prefix.
// Non-alphanumeric characters (except underscore) become '_'; result is uppercased.
func sanitizeEnvPrefix(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.ToUpper(b.String())
}

// tauriIdent derives a reverse-DNS-ish identifier from a Go module path.
// Example: github.com/foo/myapp → github.com.foo.myapp.desktop
func tauriIdent(module string) string {
	return strings.ReplaceAll(module, "/", ".") + ".desktop"
}

// moduleBase returns the last path segment of a Go module path.
func moduleBase(module string) string {
	idx := strings.LastIndex(module, "/")
	if idx < 0 {
		return module
	}
	return module[idx+1:]
}

// newScaffoldVars builds scaffoldVars from user inputs.
func newScaffoldVars(name, module, uiDir, desktopDir, miniappDir string) scaffoldVars {
	base := moduleBase(module)
	prefix := sanitizeEnvPrefix(name)
	return scaffoldVars{
		Name:            name,
		Module:          module,
		ModuleBase:      base,
		UIDir:           uiDir,
		DesktopDir:      desktopDir,
		MiniappDir:      miniappDir,
		EnvPrefix:       prefix,
		AddrEnv:         prefix + "_ADDR",
		DBDsnEnv:        prefix + "_DB_DSN",
		DevPlatformEnv:  prefix + "_DEV_PLATFORM",
		AdminPathEnv:    prefix + "_ADMIN_PATH",
		BuilderTokenEnv: prefix + "_BUILDER_TOKEN",
		TauriIdent:      tauriIdent(module),
	}
}

// scaffold holds the state for a single scaffold operation.
type scaffold struct {
	rootDir   string
	vars      scaffoldVars
	templates fs.FS
}

// walkAndRender walks an embed-FS subtree (rootPrefix) and writes rendered
// files into targetSubdir under the scaffold rootDir.  Template files (suffix
// .tmpl) are rendered with [[/]] delimiters; other files are copied verbatim.
func (s *scaffold) walkAndRender(rootPrefix, targetSubdir string) error {
	prefix := rootPrefix + "/"
	return fs.WalkDir(s.templates, rootPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, prefix)
		if rel == "" || rel == "." {
			return nil
		}

		// Create directories
		if d.IsDir() {
			dirPath := filepath.Join(s.rootDir, targetSubdir, rel)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("create dir %s: %w", dirPath, err)
			}
			return nil
		}

		// Determine output path: strip .tmpl suffix, map to target
		name := rel
		isTemplate := strings.HasSuffix(rel, ".tmpl")
		if isTemplate {
			name = strings.TrimSuffix(rel, ".tmpl")
		}
		outPath := filepath.Join(s.rootDir, targetSubdir, name)

		// Ensure parent directory
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("create parent for %s: %w", outPath, err)
		}

		if isTemplate {
			data, err := fs.ReadFile(s.templates, path)
			if err != nil {
				return fmt.Errorf("read template %s: %w", path, err)
			}
			tmpl, err := template.New(filepath.Base(path)).Delims("[[", "]]").Parse(string(data))
			if err != nil {
				return fmt.Errorf("parse template %s: %w", path, err)
			}
			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create %s: %w", outPath, err)
			}
			defer f.Close()
			if err := tmpl.Execute(f, s.vars); err != nil {
				return fmt.Errorf("render %s: %w", path, err)
			}
			return nil
		}

		// Binary files — copy verbatim
		data, err := fs.ReadFile(s.templates, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		return os.WriteFile(outPath, data, 0644)
	})
}

// runScaffold creates the project directory and writes all enabled sub-projects.
func runScaffold(rootDir string, vars scaffoldVars, opts scaffoldOpts) error {
	// Check / create target directory
	info, err := os.Stat(rootDir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", rootDir)
		}
		if !opts.Force {
			entries, _ := os.ReadDir(rootDir)
			if len(entries) > 0 {
				return fmt.Errorf("%s exists and is not empty (use --force to overwrite)", rootDir)
			}
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(rootDir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", rootDir, err)
		}
	} else {
		return fmt.Errorf("stat %s: %w", rootDir, err)
	}

	s := &scaffold{rootDir: rootDir, vars: vars, templates: scaffoldFS}

	fmt.Println("→ Scaffolding backend …")
	if err := s.walkAndRender("scaffold_templates/backend", ""); err != nil {
		return fmt.Errorf("backend: %w", err)
	}

	fmt.Printf("→ Scaffolding web frontend (%s) …\n", vars.UIDir)
	if err := s.walkAndRender("scaffold_templates/web", vars.UIDir); err != nil {
		return fmt.Errorf("web: %w", err)
	}

	if opts.Desktop {
		fmt.Printf("→ Scaffolding Tauri desktop (%s) …\n", vars.DesktopDir)
		if err := s.walkAndRender("scaffold_templates/desktop", vars.DesktopDir); err != nil {
			return fmt.Errorf("desktop: %w", err)
		}
	}

	if opts.Miniapp {
		fmt.Printf("→ Scaffolding mini-program (%s) …\n", vars.MiniappDir)
		if err := s.walkAndRender("scaffold_templates/miniapp", vars.MiniappDir); err != nil {
			return fmt.Errorf("miniapp: %w", err)
		}
	}

	return nil
}
