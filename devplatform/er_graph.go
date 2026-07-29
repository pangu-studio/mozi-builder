package devplatform

import (
	"fmt"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// ERGraphField is a single field of an ER graph node.
type ERGraphField struct {
	Name       string `json:"name"`
	Type       string `json:"type"` // same mapping as mermaidType (string/int/float/bool/time/json)
	Label      string `json:"label,omitempty"`
	Primary    bool   `json:"primary,omitempty"`
	Unique     bool   `json:"unique,omitempty"`
	Required   bool   `json:"required,omitempty"`
	ForeignKey bool   `json:"foreign_key,omitempty"`
}

// ERGraphNode is one entity (model) in the ER graph.
type ERGraphNode struct {
	ID     string         `json:"id"` // model.Name
	Name   string         `json:"name"`
	Label  string         `json:"label"`
	Module string         `json:"module"`
	Table  string         `json:"table"`
	Fields []ERGraphField `json:"fields"`
}

// ERGraphEdge is one relation between two entities.
type ERGraphEdge struct {
	ID          string `json:"id"` // source->target:name, suffixed when duplicated
	Source      string `json:"source"`
	Target      string `json:"target"`
	Type        string `json:"type"` // has_many/has_one/belongs_to/many_to_many
	Label       string `json:"label"`
	SourceField string `json:"source_field"` // field-level endpoint, may be empty
	TargetField string `json:"target_field"`
	SourceCard  string `json:"source_card"` // "1" or "N"
	TargetCard  string `json:"target_card"`
}

// ERGraph is a structured ER diagram representation for frontend renderers
// such as React Flow, replacing the Mermaid DSL for interactive rendering.
type ERGraph struct {
	Nodes []ERGraphNode `json:"nodes"`
	Edges []ERGraphEdge `json:"edges"`
}

// GenerateERGraph builds a structured ER graph from a project IR.
// It walks the same modules/models/relations as GenerateMermaidER.
func GenerateERGraph(project *mozi.ProjectIR) *ERGraph {
	graph := &ERGraph{Nodes: []ERGraphNode{}, Edges: []ERGraphEdge{}}

	// Index models by name across all modules (relations may cross modules).
	modelsByName := make(map[string]*mozi.ModelIR)
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			modelsByName[m.Name] = m
		}
	}

	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			node := ERGraphNode{
				ID:     m.Name,
				Name:   m.Name,
				Label:  m.Label,
				Module: mod.Name,
				Table:  m.Table,
				Fields: make([]ERGraphField, 0, len(m.Fields)),
			}
			for _, f := range m.Fields {
				node.Fields = append(node.Fields, ERGraphField{
					Name:     f.Name,
					Type:     mermaidType(f.Type),
					Label:    f.Label,
					Primary:  f.Primary,
					Unique:   f.Unique,
					Required: f.Required,
				})
			}
			node.Fields = appendImplicitForeignKeys(node.Fields, m)
			graph.Nodes = append(graph.Nodes, node)
		}
	}

	edgeIDs := make(map[string]int)
	for _, mod := range project.Modules {
		for _, m := range mod.Models {
			for _, r := range m.Relations {
				edge, ok := erGraphEdge(m, r, modelsByName)
				if !ok {
					continue
				}
				base := fmt.Sprintf("%s->%s:%s", edge.Source, edge.Target, r.Name)
				if n := edgeIDs[base]; n > 0 {
					edge.ID = fmt.Sprintf("%s#%d", base, n+1)
				} else {
					edge.ID = base
				}
				edgeIDs[base]++
				graph.Edges = append(graph.Edges, edge)
			}
		}
	}

	return graph
}

// appendImplicitForeignKeys appends the implicit FK columns that the code
// generator creates for belongs_to relations (see FkFieldName in
// mozi/generator/context.go and backend/schema.go.tmpl: the FK lives on the
// model declaring the belongs_to, named "<relation name>_id", string type).
// Fields already declared in the model are left untouched.
func appendImplicitForeignKeys(fields []ERGraphField, model *mozi.ModelIR) []ERGraphField {
	existing := make(map[string]bool, len(fields))
	for _, f := range fields {
		existing[f.Name] = true
	}
	for _, r := range model.Relations {
		if r.Type != mozi.RelationBelongsTo {
			continue
		}
		name := fkFieldName(r)
		if existing[name] {
			continue
		}
		existing[name] = true
		fields = append(fields, ERGraphField{
			Name:       name,
			Type:       "string",
			Label:      r.Label,
			Required:   r.Required,
			ForeignKey: true,
		})
	}
	return fields
}

// fkFieldName mirrors the generator's FkFieldName: "<relation name>_id".
func fkFieldName(rel mozi.RelationIR) string {
	return rel.Name + "_id"
}

// erGraphEdge converts a relation to a structured edge. Field-level endpoints
// follow the generator's FK conventions deterministically:
//   - belongs_to (A→B): FK "<rel>_id" on A → B's primary field
//   - has_many / has_one (A→B): A's primary field → inverse belongs_to FK on B
//   - many_to_many: join table, no FK on either side → primary fields
func erGraphEdge(model *mozi.ModelIR, rel mozi.RelationIR, modelsByName map[string]*mozi.ModelIR) (ERGraphEdge, bool) {
	target := modelsByName[rel.TargetModel]

	edge := ERGraphEdge{
		Source: model.Name,
		Target: rel.TargetModel,
		Type:   string(rel.Type),
		Label:  mermaidRelationLabel(rel),
	}

	switch rel.Type {
	case mozi.RelationBelongsTo:
		edge.SourceField = fkFieldName(rel)
		edge.TargetField = primaryField(target)
		edge.SourceCard, edge.TargetCard = "N", "1"
	case mozi.RelationHasMany:
		edge.SourceField = primaryField(model)
		edge.TargetField = inverseForeignKeyField(target, model.Name)
		edge.SourceCard, edge.TargetCard = "1", "N"
	case mozi.RelationHasOne:
		edge.SourceField = primaryField(model)
		edge.TargetField = inverseForeignKeyField(target, model.Name)
		edge.SourceCard, edge.TargetCard = "1", "1"
	case mozi.RelationManyToMany:
		edge.SourceField = primaryField(model)
		edge.TargetField = primaryField(target)
		edge.SourceCard, edge.TargetCard = "N", "N"
	default:
		return ERGraphEdge{}, false
	}

	return edge, true
}

// inverseForeignKeyField finds the belongs_to relation on target that points
// back to sourceName and returns its FK field name. Returns "" when the target
// declares no such inverse relation (the generator would not create a FK).
func inverseForeignKeyField(target *mozi.ModelIR, sourceName string) string {
	if target == nil {
		return ""
	}
	for _, r := range target.Relations {
		if r.Type == mozi.RelationBelongsTo && r.TargetModel == sourceName {
			return fkFieldName(r)
		}
	}
	return ""
}

// primaryField returns the model's primary field name, falling back to "id".
func primaryField(model *mozi.ModelIR) string {
	if model == nil {
		return "id"
	}
	for _, f := range model.Fields {
		if f.Primary {
			return f.Name
		}
	}
	return "id"
}
