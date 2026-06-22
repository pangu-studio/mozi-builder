package devplatform

import (
	"fmt"
	"strings"

	"memflow/mozi-builder/mozi"
)

// GenerateMermaidER generates a Mermaid erDiagram DSL string from a project IR.
// Uses ELK layout engine with LR direction to ensure horizontal entity arrangement.
func GenerateMermaidER(project *mozi.ProjectIR) string {
	var sb strings.Builder

	// ELK layout engine with left-to-right direction for horizontal arrangement.
	sb.WriteString("%%{init: {'layout': 'elk', 'er': {'layoutDirection': 'LR'}} }%%\n")
	sb.WriteString("erDiagram\n")
	sb.WriteString("    direction LR\n")
	sb.WriteString("\n")

	// Emit relationships first
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			for _, r := range m.Relations {
				rel := mermaidRelation(m, r)
				if rel != "" {
					sb.WriteString("    ")
					sb.WriteString(rel)
					sb.WriteString("\n")
				}
			}
		}
	}

	sb.WriteString("\n")

	// Emit entity blocks
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			sb.WriteString(mermaidEntity(m))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// mermaidRelation converts a relation to Mermaid relation syntax.
func mermaidRelation(model *mozi.ModelIR, rel mozi.RelationIR) string {
	from := model.Name
	to := rel.TargetModel

	switch rel.Type {
	case mozi.RelationHasMany:
		return fmt.Sprintf("%s ||--o{ %s : %s", from, to, rel.Name)
	case mozi.RelationHasOne:
		return fmt.Sprintf("%s ||--|| %s : %s", from, to, rel.Name)
	case mozi.RelationBelongsTo:
		return fmt.Sprintf("%s }o--|| %s : %s", from, to, rel.Name)
	case mozi.RelationManyToMany:
		return fmt.Sprintf("%s }o--o{ %s : %s", from, to, rel.Name)
	}
	return ""
}

// mermaidEntity converts a model to a Mermaid entity block.
func mermaidEntity(model *mozi.ModelIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("    %s {\n", model.Name))

	for _, f := range model.Fields {
		typeStr := mermaidType(f.Type)
		keys := make([]string, 0, 2)
		if f.Primary {
			keys = append(keys, "PK")
		}
		if f.Unique {
			keys = append(keys, "UK")
		}
		annotations := ""
		if len(keys) > 0 {
			annotations += " " + strings.Join(keys, ",")
		}
		if f.Required {
			annotations += ` "NOT NULL"`
		}
		sb.WriteString(fmt.Sprintf("        %s %s%s\n", typeStr, f.Name, annotations))
	}

	sb.WriteString("    }")
	return sb.String()
}

// mermaidType maps FieldType to Mermaid type name.
func mermaidType(ft mozi.FieldType) string {
	switch ft {
	case mozi.FieldTypeString, mozi.FieldTypeText, mozi.FieldTypeEnum:
		return "string"
	case mozi.FieldTypeInt:
		return "int"
	case mozi.FieldTypeFloat:
		return "float"
	case mozi.FieldTypeBool:
		return "bool"
	case mozi.FieldTypeTime:
		return "time"
	case mozi.FieldTypeJSON:
		return "json"
	default:
		return "string"
	}
}
