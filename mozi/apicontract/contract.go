package apicontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

type Endpoint struct{ OperationID, Method, Path string }
type Artifact struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func ParseOpenAPI(data []byte) ([]Endpoint, error) {
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var result []Endpoint
	for path, methods := range doc.Paths {
		for method, op := range methods {
			if op.OperationID != "" {
				result = append(result, Endpoint{op.OperationID, strings.ToUpper(method), path})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OperationID < result[j].OperationID })
	return result, nil
}

func GenerateBruno(model *mozi.ModelIR, endpoints []Endpoint) ([]Artifact, error) {
	byID := map[string]Endpoint{}
	for _, e := range endpoints {
		byID[e.OperationID] = e
	}
	var out []Artifact
	for _, c := range model.APIIntent.TestContracts {
		e, ok := byID[c.OperationID]
		if !ok {
			return nil, fmt.Errorf("test contract %s references unknown operation %s", c.Name, c.OperationID)
		}
		body, _ := json.MarshalIndent(c.Request.Body, "", "  ")
		var b strings.Builder
		fmt.Fprintf(&b, "meta {\n  name: %s\n  type: http\n  seq: 1\n}\n\n%s {\n  url: {{baseUrl}}%s\n  body: json\n}\n", c.Name, strings.ToLower(e.Method), e.Path)
		if len(c.Request.Body) > 0 {
			fmt.Fprintf(&b, "\nbody:json {\n%s\n}\n", indent(string(body), 2))
		}
		fmt.Fprintf(&b, "\ntests {\n  test(\"status is %d\", function() { expect(res.status).to.equal(%d); });\n}\n", c.Expect.Status, c.Expect.Status)
		if c.Expect.ErrorCode != "" {
			fmt.Fprintf(&b, "\ntests {\n  test(\"error code is %s\", function() { expect(res.body.error.code).to.equal(%q); });\n}\n", c.Expect.ErrorCode, c.Expect.ErrorCode)
		}
		out = append(out, Artifact{Path: c.Name + ".bru", Content: b.String()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func indent(s string, n int) string {
	prefix := strings.Repeat(" ", n)
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
