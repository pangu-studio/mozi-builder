package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFileOnlyLoadsDatabaseKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# local\nMOZI_DB=postgres://host/mozi_v2_design?sslmode=disable&x=y\nIGNORED=secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOZI_DB", "")
	t.Setenv("IGNORED", "")
	if err := loadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MOZI_DB"); got != "postgres://host/mozi_v2_design?sslmode=disable&x=y" {
		t.Fatalf("unexpected URL %q", got)
	}
	if os.Getenv("IGNORED") != "" {
		t.Fatal("unrelated environment key was loaded")
	}
}
