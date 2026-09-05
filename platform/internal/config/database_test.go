package config

import (
	"strings"
	"testing"
)

func TestDedicatedDatabasesRequired(t *testing.T) {
	valid := map[string]string{"MOZI_DB": "postgres://design:secret@db:5432/mozi_v2_design?sslmode=require", "MOZI_PLATFORM_DB": "postgres://platform:secret@db:5432/mozi_v2_platform?sslmode=require"}
	for _, tc := range []struct{ name, key, value string }{{"missing", "MOZI_DB", ""}, {"legacy", "MOZI_DB", "postgres://user:secret@db/mozi"}, {"query override", "MOZI_DB", valid["MOZI_DB"] + "&dbname=memflow_dev"}, {"wrong role database", "MOZI_PLATFORM_DB", valid["MOZI_DB"]}, {"malformed", "MOZI_DB", "postgres://user:secret%xx@db/mozi_v2_design"}, {"dsn syntax", "MOZI_DB", "host=db dbname=mozi password=secret"}} {
		t.Run(tc.name, func(t *testing.T) {
			get := func(key string) string {
				if key == tc.key {
					return tc.value
				}
				return valid[key]
			}
			_, err := LoadDatabases(get)
			if err == nil {
				t.Fatal("unsafe configuration accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("credential exposed in error")
			}
		})
	}
	if _, err := LoadDatabases(func(key string) string { return valid[key] }); err != nil {
		t.Fatal(err)
	}
}
