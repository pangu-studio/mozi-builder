package i18n

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

type Entry struct {
	Key    string `json:"key"`
	Source string `json:"source"`
	Kind   string `json:"kind"`
	Model  string `json:"model,omitempty"`
}
type Catalog struct {
	Locale  string  `json:"locale"`
	Entries []Entry `json:"entries"`
}
type Validation struct {
	Missing, Stale    []string
	PlaceholderErrors []string
}

func Extract(project *mozi.ProjectIR, locale string) Catalog {
	entries := map[string]Entry{}
	for _, code := range project.ErrorCodes {
		key := code.I18nKey
		if key == "" {
			key = "error." + strings.ToLower(code.Code)
		}
		entries[key] = Entry{key, code.Message, "error", ""}
	}
	for _, mod := range project.Modules {
		for _, model := range mod.Models {
			ref := mod.Name + "/" + model.Name
			modelKey := "model." + strings.ToLower(mod.Name) + "." + snake(model.Name) + ".label"
			entries[modelKey] = Entry{modelKey, model.Label, "model", ref}
			for _, field := range model.Fields {
				key := field.I18nKey
				if key == "" {
					key = "field." + strings.ToLower(mod.Name) + "." + snake(model.Name) + "." + field.Name
				}
				entries[key] = Entry{key, field.Label, "field", ref}
			}
		}
	}
	result := Catalog{Locale: locale}
	for _, entry := range entries {
		result.Entries = append(result.Entries, entry)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Key < result.Entries[j].Key })
	return result
}

func Validate(catalog Catalog, translations map[string]string) Validation {
	result := Validation{}
	expected := map[string]Entry{}
	for _, e := range catalog.Entries {
		expected[e.Key] = e
		if _, ok := translations[e.Key]; !ok {
			result.Missing = append(result.Missing, e.Key)
		} else if !samePlaceholders(e.Source, translations[e.Key]) {
			result.PlaceholderErrors = append(result.PlaceholderErrors, fmt.Sprintf("%s placeholder mismatch", e.Key))
		}
	}
	for key := range translations {
		if _, ok := expected[key]; !ok {
			result.Stale = append(result.Stale, key)
		}
	}
	sort.Strings(result.Missing)
	sort.Strings(result.Stale)
	sort.Strings(result.PlaceholderErrors)
	return result
}

var placeholder = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)

func samePlaceholders(a, b string) bool {
	left, right := placeholder.FindAllString(a, -1), placeholder.FindAllString(b, -1)
	sort.Strings(left)
	sort.Strings(right)
	return strings.Join(left, ",") == strings.Join(right, ",")
}
func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
