package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Databases is deliberately separate from v1's defaulting configuration.
// v2 refuses legacy database names, missing configuration and URL query db overrides.
type Databases struct{ Design, Platform string }

func LoadDatabases(getenv func(string) string) (Databases, error) {
	var result Databases
	for _, item := range []struct {
		key, name string
		target    *string
	}{{"MOZI_DB", "mozi_v2_design", &result.Design}, {"MOZI_PLATFORM_DB", "mozi_v2_platform", &result.Platform}} {
		raw := strings.TrimSpace(getenv(item.key))
		if raw == "" {
			return Databases{}, fmt.Errorf("%s is required; v2 has no database fallback", item.key)
		}
		u, err := url.Parse(raw)
		if err != nil || u == nil {
			return Databases{}, fmt.Errorf("%s must be a PostgreSQL URL", item.key)
		}
		if (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Fragment != "" || u.Opaque != "" {
			return Databases{}, fmt.Errorf("%s must be a PostgreSQL URL with a host", item.key)
		}
		if u.Path != "/"+item.name {
			return Databases{}, fmt.Errorf("%s must select the dedicated %s database", item.key, item.name)
		}
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return Databases{}, fmt.Errorf("%s contains an invalid query", item.key)
		}
		for key := range query {
			switch strings.ToLower(key) {
			case "dbname", "database", "host", "hostaddr", "port", "user", "password", "service", "options":
				return Databases{}, fmt.Errorf("%s may not override connection identity through query parameters", item.key)
			}
		}
		*item.target = raw
	}
	return result, nil
}
