package migration

import (
	"fmt"
	"regexp"
	"strings"
)

type Files struct {
	BaseName string `json:"base_name"`
	Up       string `json:"up"`
	Down     string `json:"down"`
}

// RenderSafe renders files only when every step is explicitly safe.
func RenderSafe(plan Plan, version, description string) (Files, error) {
	if len(plan.Steps) == 0 {
		return Files{}, fmt.Errorf("migration plan has no steps")
	}
	var up, down []string
	for _, step := range plan.Steps {
		if step.Risk != RiskSafe || step.RequiresConfirmation {
			return Files{}, fmt.Errorf("%s requires review and cannot be generated automatically", step.Kind)
		}
		if strings.TrimSpace(step.SQL) == "" {
			return Files{}, fmt.Errorf("%s has no SQL", step.Kind)
		}
		up = append(up, step.SQL)
		downSQL, ok := safeDown(step)
		if !ok {
			return Files{}, fmt.Errorf("%s has no safe down migration", step.Kind)
		}
		down = append([]string{downSQL}, down...)
	}
	base := sanitize(version + "_" + description)
	return Files{BaseName: base, Up: strings.Join(up, "\n") + "\n", Down: strings.Join(down, "\n") + "\n"}, nil
}

func safeDown(step Step) (string, bool) {
	if step.Kind != "add_column" {
		return "", false
	}
	parts := strings.Fields(step.SQL)
	if len(parts) < 6 {
		return "", false
	}
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", parts[2], parts[5]), true
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func sanitize(value string) string {
	return strings.Trim(unsafeName.ReplaceAllString(strings.ToLower(value), "_"), "_")
}
