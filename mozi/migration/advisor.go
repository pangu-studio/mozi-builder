package migration

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
	"github.com/pangu-studio/mozi-builder/mozi/differ"
)

type Risk string

const (
	RiskSafe        Risk = "safe"
	RiskConditional Risk = "conditional"
	RiskDangerous   Risk = "dangerous"
)

type Step struct {
	Kind                 string `json:"kind"`
	Risk                 Risk   `json:"risk"`
	Description          string `json:"description"`
	SQL                  string `json:"sql,omitempty"`
	Reversible           bool   `json:"reversible"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
}

type Plan struct {
	Dialect      string `json:"dialect"`
	ModelRef     string `json:"model_ref"`
	Table        string `json:"table"`
	Steps        []Step `json:"steps"`
	HasDangerous bool   `json:"has_dangerous"`
}

// Advise produces reviewable PostgreSQL guidance. It never executes SQL.
func Advise(from, to *mozi.ModelIR, diff *differ.DiffResult) Plan {
	plan := Plan{Dialect: "postgres", ModelRef: diff.ModelRef, Table: to.Table, Steps: []Step{}}
	if diff.FromVersion == "" {
		plan.Steps = append(plan.Steps, Step{Kind: "create_table", Risk: RiskConditional, Description: "First model version: create the table through the project ORM/migration tool", Reversible: true, RequiresConfirmation: true})
		return plan
	}
	for _, change := range diff.Changes {
		var step *Step
		switch {
		case change.Category == "field" && change.Type == differ.ChangeAdded:
			field := to.GetField(change.Name)
			if field == nil {
				continue
			}
			risk := RiskSafe
			if field.Required && field.Default == nil {
				risk = RiskConditional
			}
			sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", ident(to.Table), ident(field.Name), sqlType(field.Type))
			if field.Default != nil {
				sql += " DEFAULT " + sqlLiteral(*field.Default)
			}
			if field.Required {
				sql += " NOT NULL"
			}
			sql += ";"
			step = &Step{Kind: "add_column", Risk: risk, Description: "Add column " + field.Name, SQL: sql, Reversible: true, RequiresConfirmation: risk != RiskSafe}
		case change.Category == "field" && change.Type == differ.ChangeRemoved:
			step = &Step{Kind: "drop_column", Risk: RiskDangerous, Description: "Drop column " + change.Name + "; data loss is irreversible", SQL: fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", ident(from.Table), ident(change.Name)), Reversible: false, RequiresConfirmation: true}
		case change.Category == "field" && change.Type == differ.ChangeModified && strings.Contains(change.Detail, "renamed"):
			step = &Step{Kind: "rename_column", Risk: RiskConditional, Description: change.Detail, SQL: fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", ident(to.Table), ident(change.OldValue), ident(change.NewValue)), Reversible: true, RequiresConfirmation: true}
		case change.Category == "field" && change.Type == differ.ChangeModified && strings.Contains(change.Detail, "— type"):
			step = &Step{Kind: "alter_column_type", Risk: RiskDangerous, Description: change.Detail + "; provide an explicit USING expression", Reversible: false, RequiresConfirmation: true}
		case change.Category == "meta" && change.Name == "table":
			step = &Step{Kind: "rename_table", Risk: RiskConditional, Description: change.Detail, SQL: fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", ident(change.OldValue), ident(change.NewValue)), Reversible: true, RequiresConfirmation: true}
		}
		if step != nil {
			plan.Steps = append(plan.Steps, *step)
			plan.HasDangerous = plan.HasDangerous || step.Risk == RiskDangerous
		}
	}
	return plan
}

var safeIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ident(value string) string {
	if safeIdent.MatchString(value) {
		return `"` + value + `"`
	}
	return `"INVALID_IDENTIFIER"`
}

func sqlType(fieldType mozi.FieldType) string {
	switch fieldType {
	case mozi.FieldTypeInt:
		return "BIGINT"
	case mozi.FieldTypeFloat:
		return "DOUBLE PRECISION"
	case mozi.FieldTypeBool:
		return "BOOLEAN"
	case mozi.FieldTypeTime:
		return "TIMESTAMPTZ"
	case mozi.FieldTypeJSON:
		return "JSONB"
	case mozi.FieldTypeText:
		return "TEXT"
	default:
		return "TEXT"
	}
}

func sqlLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
