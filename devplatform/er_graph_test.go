package devplatform

import (
	"testing"

	"github.com/pangu-studio/mozi-builder/mozi"
)

func erGraphTestProject() *mozi.ProjectIR {
	return &mozi.ProjectIR{
		Modules: []*mozi.ModuleIR{
			{
				Name: "content",
				Models: []*mozi.ModelIR{
					{
						Name:  "Deck",
						Label: "牌组",
						Table: "decks",
						Fields: []mozi.FieldIR{
							{Name: "id", Type: mozi.FieldTypeString, Primary: true},
							{Name: "name", Type: mozi.FieldTypeString, Required: true},
						},
						Relations: []mozi.RelationIR{
							{Name: "cards", Label: "包含", Type: mozi.RelationHasMany, TargetModel: "Card"},
						},
					},
					{
						Name:  "Card",
						Label: "卡片",
						Table: "cards",
						Fields: []mozi.FieldIR{
							{Name: "id", Type: mozi.FieldTypeString, Primary: true},
							{Name: "front", Type: mozi.FieldTypeText, Required: true},
						},
						Relations: []mozi.RelationIR{
							// 未声明 deck_id，验证隐式 FK 注入
							{Name: "deck", Type: mozi.RelationBelongsTo, TargetModel: "Deck"},
							{Name: "state", Type: mozi.RelationHasOne, TargetModel: "CardState"},
							{Name: "tags", Type: mozi.RelationManyToMany, TargetModel: "Tag"},
						},
					},
					{
						Name: "CardState",
						Fields: []mozi.FieldIR{
							{Name: "id", Type: mozi.FieldTypeString, Primary: true},
							{Name: "reps", Type: mozi.FieldTypeInt},
						},
						Relations: []mozi.RelationIR{
							{Name: "card", Type: mozi.RelationBelongsTo, TargetModel: "Card"},
						},
					},
					{
						Name: "ReviewLog",
						Fields: []mozi.FieldIR{
							{Name: "id", Type: mozi.FieldTypeString, Primary: true},
							// 已声明 card_id，验证注入跳过、不重复
							{Name: "card_id", Type: mozi.FieldTypeString, Required: true},
						},
						Relations: []mozi.RelationIR{
							{Name: "card", Type: mozi.RelationBelongsTo, TargetModel: "Card"},
						},
					},
					{
						Name: "Tag",
						Fields: []mozi.FieldIR{
							{Name: "id", Type: mozi.FieldTypeString, Primary: true},
							{Name: "name", Type: mozi.FieldTypeString},
						},
					},
				},
			},
		},
	}
}

func nodeByName(graph *ERGraph, name string) *ERGraphNode {
	for i := range graph.Nodes {
		if graph.Nodes[i].Name == name {
			return &graph.Nodes[i]
		}
	}
	return nil
}

func fieldByName(node *ERGraphNode, name string) *ERGraphField {
	for i := range node.Fields {
		if node.Fields[i].Name == name {
			return &node.Fields[i]
		}
	}
	return nil
}

func TestGenerateERGraphNodes(t *testing.T) {
	graph := GenerateERGraph(erGraphTestProject())

	if len(graph.Nodes) != 5 {
		t.Fatalf("node count = %d, want 5", len(graph.Nodes))
	}

	card := nodeByName(graph, "Card")
	if card == nil || card.ID != "Card" || card.Module != "content" || card.Table != "cards" {
		t.Fatalf("unexpected card node: %+v", card)
	}
	if !card.Fields[0].Primary {
		t.Fatalf("id field should be primary: %+v", card.Fields[0])
	}
}

func TestGenerateERGraphInjectsImplicitForeignKeys(t *testing.T) {
	graph := GenerateERGraph(erGraphTestProject())

	// belongs_to 的 FK 注入到声明方：Card.deck → deck_id
	card := nodeByName(graph, "Card")
	deckID := fieldByName(card, "deck_id")
	if deckID == nil {
		t.Fatalf("card node missing injected deck_id, fields: %+v", card.Fields)
	}
	if !deckID.ForeignKey || deckID.Type != "string" || deckID.Primary {
		t.Fatalf("unexpected injected deck_id: %+v", deckID)
	}

	// CardState.card → card_id 注入到 CardState
	state := nodeByName(graph, "CardState")
	if fk := fieldByName(state, "card_id"); fk == nil || !fk.ForeignKey {
		t.Fatalf("card_state node missing injected card_id, fields: %+v", state.Fields)
	}

	// 已声明的 card_id 不重复注入、保持原样（不标 ForeignKey）
	reviewLog := nodeByName(graph, "ReviewLog")
	count := 0
	for _, f := range reviewLog.Fields {
		if f.Name == "card_id" {
			count++
			if f.ForeignKey {
				t.Fatalf("declared card_id should not be marked FK: %+v", f)
			}
		}
	}
	if count != 1 {
		t.Fatalf("review_log card_id count = %d, want 1", count)
	}

	// many_to_many 不产生 FK：Card 与 Tag 都不应有 tags 相关 FK 字段
	if fk := fieldByName(card, "tags_id"); fk != nil {
		t.Fatalf("many_to_many should not inject FK: %+v", fk)
	}
	tag := nodeByName(graph, "Tag")
	if len(tag.Fields) != 2 {
		t.Fatalf("tag node should have only declared fields: %+v", tag.Fields)
	}
}

func TestGenerateERGraphEdgeEndpoints(t *testing.T) {
	graph := GenerateERGraph(erGraphTestProject())

	byID := make(map[string]ERGraphEdge)
	for _, e := range graph.Edges {
		byID[e.ID] = e
	}

	belongsTo, ok := byID["Card->Deck:deck"]
	if !ok {
		t.Fatalf("missing belongs_to edge, edges: %+v", graph.Edges)
	}
	if belongsTo.SourceField != "deck_id" || belongsTo.TargetField != "id" {
		t.Fatalf("belongs_to endpoints = %q -> %q, want deck_id -> id",
			belongsTo.SourceField, belongsTo.TargetField)
	}
	if belongsTo.SourceCard != "N" || belongsTo.TargetCard != "1" {
		t.Fatalf("belongs_to cardinality = %s -> %s, want N -> 1",
			belongsTo.SourceCard, belongsTo.TargetCard)
	}

	hasMany, ok := byID["Deck->Card:cards"]
	if !ok {
		t.Fatalf("missing has_many edge, edges: %+v", graph.Edges)
	}
	// FK 在 B（Card）上，取 Card 的反向 belongs_to（deck）的 FK 名
	if hasMany.SourceField != "id" || hasMany.TargetField != "deck_id" {
		t.Fatalf("has_many endpoints = %q -> %q, want id -> deck_id",
			hasMany.SourceField, hasMany.TargetField)
	}
	if hasMany.SourceCard != "1" || hasMany.TargetCard != "N" {
		t.Fatalf("has_many cardinality = %s -> %s, want 1 -> N",
			hasMany.SourceCard, hasMany.TargetCard)
	}

	hasOne, ok := byID["Card->CardState:state"]
	if !ok {
		t.Fatalf("missing has_one edge, edges: %+v", graph.Edges)
	}
	// FK 在 CardState 上（其 belongs_to card 的 card_id）
	if hasOne.SourceField != "id" || hasOne.TargetField != "card_id" {
		t.Fatalf("has_one endpoints = %q -> %q, want id -> card_id",
			hasOne.SourceField, hasOne.TargetField)
	}
	if hasOne.SourceCard != "1" || hasOne.TargetCard != "1" {
		t.Fatalf("has_one cardinality = %s -> %s, want 1 -> 1",
			hasOne.SourceCard, hasOne.TargetCard)
	}

	m2m, ok := byID["Card->Tag:tags"]
	if !ok {
		t.Fatalf("missing many_to_many edge, edges: %+v", graph.Edges)
	}
	if m2m.SourceField != "id" || m2m.TargetField != "id" {
		t.Fatalf("many_to_many endpoints = %q -> %q, want id -> id",
			m2m.SourceField, m2m.TargetField)
	}
	if m2m.SourceCard != "N" || m2m.TargetCard != "N" {
		t.Fatalf("many_to_many cardinality = %s -> %s, want N -> N",
			m2m.SourceCard, m2m.TargetCard)
	}
}

func TestGenerateERGraphEdgeIDsUnique(t *testing.T) {
	project := erGraphTestProject()
	card := project.Modules[0].Models[1]
	card.Relations = append(card.Relations, mozi.RelationIR{
		Name: "deck", Type: mozi.RelationBelongsTo, TargetModel: "Deck",
	})

	graph := GenerateERGraph(project)

	seen := make(map[string]bool)
	for _, e := range graph.Edges {
		if seen[e.ID] {
			t.Fatalf("duplicate edge id %q", e.ID)
		}
		seen[e.ID] = true
	}
}
