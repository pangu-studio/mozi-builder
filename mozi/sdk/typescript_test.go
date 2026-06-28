package sdk

import (
	"strings"
	"testing"
)

func TestGenerateTypeScript(t *testing.T) {
	src := []byte(`{"openapi":"3.0.0","paths":{"/decks/{id}":{"get":{"operationId":"getDeck","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Deck"}}}}}}}},"components":{"schemas":{"Deck":{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"name":{"type":"string"}}}}}}`)
	out, err := GenerateTypeScript(src)
	if err != nil || !strings.Contains(out, "interface Deck") || !strings.Contains(out, "async getDeck") {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
