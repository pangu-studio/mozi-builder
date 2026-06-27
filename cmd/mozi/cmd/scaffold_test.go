package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeEnvPrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{"myapp", "MYAPP"},
		{"my-app", "MY_APP"},
		{"MyApp", "MYAPP"},
		{"my_app", "MY_APP"},
		{"123app", "123APP"},
		{"my.app", "MY_APP"},
	}
	for _, tc := range tests {
		got := sanitizeEnvPrefix(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeEnvPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTauriIdent(t *testing.T) {
	got := tauriIdent("github.com/foo/myapp")
	want := "github.com.foo.myapp.desktop"
	if got != want {
		t.Errorf("tauriIdent = %q, want %q", got, want)
	}
}

func TestModuleBase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"github.com/foo/myapp", "myapp"},
		{"myapp", "myapp"},
		{"github.com/foo/bar/baz", "baz"},
	}
	for _, tc := range tests {
		got := moduleBase(tc.in)
		if got != tc.want {
			t.Errorf("moduleBase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewScaffoldVars(t *testing.T) {
	v := newScaffoldVars("myapp", "github.com/foo/myapp", "myapp-ui", "myapp-desktop", "myapp-miniapp")

	if v.Name != "myapp" {
		t.Errorf("Name = %q", v.Name)
	}
	if v.Module != "github.com/foo/myapp" {
		t.Errorf("Module = %q", v.Module)
	}
	if v.ModuleBase != "myapp" {
		t.Errorf("ModuleBase = %q", v.ModuleBase)
	}
	if v.UIDir != "myapp-ui" {
		t.Errorf("UIDir = %q", v.UIDir)
	}
	if v.EnvPrefix != "MYAPP" {
		t.Errorf("EnvPrefix = %q", v.EnvPrefix)
	}
	if v.AddrEnv != "MYAPP_ADDR" {
		t.Errorf("AddrEnv = %q", v.AddrEnv)
	}
	if v.DevPlatformEnv != "MYAPP_DEV_PLATFORM" {
		t.Errorf("DevPlatformEnv = %q", v.DevPlatformEnv)
	}
	if v.AdminPathEnv != "MYAPP_ADMIN_PATH" {
		t.Errorf("AdminPathEnv = %q", v.AdminPathEnv)
	}
	if v.BuilderTokenEnv != "MYAPP_BUILDER_TOKEN" {
		t.Errorf("BuilderTokenEnv = %q", v.BuilderTokenEnv)
	}
	if v.TauriIdent != "github.com.foo.myapp.desktop" {
		t.Errorf("TauriIdent = %q", v.TauriIdent)
	}
}

func TestScaffoldBackendOnly(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")

	err := runScaffold(root, vars, scaffoldOpts{})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// Core backend files
	assertFile(t, root, "go.mod")
	assertFile(t, root, "Makefile")
	assertFile(t, root, ".env")
	assertFile(t, root, ".env.example")
	assertFile(t, root, ".gitignore")
	assertFile(t, root, "CLAUDE.md")
	assertFile(t, root, "README.md")
	assertFile(t, root, "cmd/server/main.go")
	assertFile(t, root, "internal/middleware/cors.go")
	assertFile(t, root, "internal/middleware/builder.go")
	assertFile(t, root, "models/_project.yaml")
	assertFile(t, root, "docs/dev-platform-guide.md")

	// Verify template rendering in main.go
	mainGo := readFile(t, filepath.Join(root, "cmd/server/main.go"))
	if !strings.Contains(mainGo, `"github.com/test/testapp/internal/middleware"`) {
		t.Error("main.go missing correct module import")
	}
	if !strings.Contains(mainGo, `TESTAPP_DEV_PLATFORM`) {
		t.Error("main.go missing env var prefix")
	}

	// Verify _project.yaml
	projectYaml := readFile(t, filepath.Join(root, "models/_project.yaml"))
	if !strings.Contains(projectYaml, "name: testapp") {
		t.Error("_project.yaml missing name")
	}
	if !strings.Contains(projectYaml, "module: github.com/test/testapp") {
		t.Error("_project.yaml missing module")
	}

	// Verify .env
	envContent := readFile(t, filepath.Join(root, ".env"))
	if !strings.Contains(envContent, "TESTAPP_ADDR") {
		t.Error(".env missing env prefix")
	}

	// Web files
	assertFile(t, root, "testapp-ui/package.json")
	assertFile(t, root, "testapp-ui/vite.config.ts")
	assertFile(t, root, "testapp-ui/src/main.tsx")
	assertFile(t, root, "testapp-ui/src/App.tsx")
	assertFile(t, root, "testapp-ui/src/api.ts")
	assertFile(t, root, "testapp-ui/src/components/Layout.tsx")
	assertFile(t, root, "testapp-ui/src/pages/Dashboard.tsx")
	assertFile(t, root, "testapp-ui/src/stores/auth.ts")

	// Desktop and miniapp must NOT exist
	assertNoFile(t, root, "testapp-desktop/package.json")
	assertNoFile(t, root, "testapp-miniapp/package.json")
}

func TestScaffoldWithDesktop(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")

	err := runScaffold(root, vars, scaffoldOpts{Desktop: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// Desktop files
	assertFile(t, root, "testapp-desktop/package.json")
	assertFile(t, root, "testapp-desktop/vite.config.ts")
	assertFile(t, root, "testapp-desktop/src/App.tsx")
	assertFile(t, root, "testapp-desktop/src-tauri/Cargo.toml")
	assertFile(t, root, "testapp-desktop/src-tauri/tauri.conf.json")
	assertFile(t, root, "testapp-desktop/src-tauri/build.rs")
	assertFile(t, root, "testapp-desktop/src-tauri/src/main.rs")
	assertFile(t, root, "testapp-desktop/src-tauri/src/lib.rs")
	assertFile(t, root, "testapp-desktop/src-tauri/capabilities/default.json")

	// Verify Cargo.toml has no [[bin]] (would conflict with template delimiters)
	cargoToml := readFile(t, filepath.Join(root, "testapp-desktop/src-tauri/Cargo.toml"))
	if strings.Contains(cargoToml, "[[") {
		t.Error("Cargo.toml contains raw [[ characters (template delimiter conflict)")
	}

	// Verify tauri.conf.json rendering
	tauriConf := readFile(t, filepath.Join(root, "testapp-desktop/src-tauri/tauri.conf.json"))
	if !strings.Contains(tauriConf, "github.com.test.testapp.desktop") {
		t.Error("tauri.conf.json missing correct identifier")
	}

	// Miniapp must NOT exist
	assertNoFile(t, root, "testapp-miniapp/package.json")
}

func TestScaffoldWithMiniapp(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")

	err := runScaffold(root, vars, scaffoldOpts{Miniapp: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// Miniapp files
	assertFile(t, root, "testapp-miniapp/package.json")
	assertFile(t, root, "testapp-miniapp/babel.config.js")
	assertFile(t, root, "testapp-miniapp/tsconfig.json")
	assertFile(t, root, "testapp-miniapp/project.config.json")
	assertFile(t, root, "testapp-miniapp/config/index.ts")
	assertFile(t, root, "testapp-miniapp/config/dev.ts")
	assertFile(t, root, "testapp-miniapp/config/prod.ts")
	assertFile(t, root, "testapp-miniapp/src/app.tsx")
	assertFile(t, root, "testapp-miniapp/src/app.config.ts")
	assertFile(t, root, "testapp-miniapp/src/app.scss")
	assertFile(t, root, "testapp-miniapp/src/utils/api.ts")
	assertFile(t, root, "testapp-miniapp/src/pages/index/index.tsx")

	// Desktop must NOT exist
	assertNoFile(t, root, "testapp-desktop/package.json")
}

func TestScaffoldWithAll(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")

	err := runScaffold(root, vars, scaffoldOpts{Desktop: true, Miniapp: true})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	assertFile(t, root, "testapp-ui/src/App.tsx")
	assertFile(t, root, "testapp-desktop/src-tauri/Cargo.toml")
	assertFile(t, root, "testapp-miniapp/src/app.tsx")
}

func TestScaffoldRejectsNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	// Create a file in target
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")
	err := runScaffold(root, vars, scaffoldOpts{})
	if err == nil {
		t.Error("expected error for non-empty directory without --force")
	}
	if !strings.Contains(err.Error(), "not empty") && !strings.Contains(err.Error(), "--force") {
		t.Errorf("error message should mention non-empty dir and --force: %v", err)
	}
}

func TestScaffoldForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "testapp")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	vars := newScaffoldVars("testapp", "github.com/test/testapp", "testapp-ui", "testapp-desktop", "testapp-miniapp")
	err := runScaffold(root, vars, scaffoldOpts{Force: true})
	if err != nil {
		t.Fatalf("scaffold with --force: %v", err)
	}
	assertFile(t, root, "go.mod") // newly scaffolded
}

// helpers

func assertFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if _, err := os.Stat(p); err != nil {
		t.Errorf("missing file: %s", rel)
	}
}

func assertNoFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if _, err := os.Stat(p); err == nil {
		t.Errorf("unexpected file: %s", rel)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
