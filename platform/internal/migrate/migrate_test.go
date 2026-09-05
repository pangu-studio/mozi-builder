package migrate

import "testing"

func TestEmbeddedMigrationSets(t *testing.T) {
	for _, target := range []string{"design", "platform"} {
		entries, err := load(target)
		if err != nil {
			t.Fatal(err)
		}
		if entries[0].Version != 1 || entries[0].Checksum == "" || entries[0].SQL == "" {
			t.Fatalf("invalid %s migration metadata", target)
		}
	}
}

func TestUnknownTargetRejected(t *testing.T) {
	if _, err := load("legacy"); err == nil {
		t.Fatal("unknown target accepted")
	}
}

func TestValidateAppliedRejectsNewerDatabase(t *testing.T) {
	entries, err := load("design")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]Applied{}
	for _, migration := range entries {
		got[migration.Version] = migration.Applied
	}
	got[9999] = Applied{Version: 9999, Name: "future", Checksum: "future"}
	if err := validateApplied(entries, got, true); err == nil {
		t.Fatal("migration from a newer database was accepted")
	}
}
