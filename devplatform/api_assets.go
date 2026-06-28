package devplatform

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// APISurface describes the product surface served by an HTTP API.
type APISurface string

const (
	APISurfaceAdmin        APISurface = "admin"
	APISurfaceMiniApp      APISurface = "miniapp"
	APISurfaceDesktop      APISurface = "desktop"
	APISurfaceClientShared APISurface = "client-shared"
	APISurfaceInternal     APISurface = "internal"
	APISurfacePublic       APISurface = "public"
)

// APIAssetIndex is the dev-platform view of the generated OpenAPI contract.
type APIAssetIndex struct {
	Source        string             `json:"source"`
	Title         string             `json:"title"`
	Version       string             `json:"version"`
	BasePath      string             `json:"base_path"`
	GeneratedBy   string             `json:"generated_by"`
	Summary       APIAssetSummary    `json:"summary"`
	Modules       []APIModuleSummary `json:"modules"`
	Surfaces      []APISurfaceCount  `json:"surfaces"`
	Endpoints     []APIEndpointAsset `json:"endpoints"`
	SchemaModels  []APISchemaAsset   `json:"schema_models"`
	ContractDrift []APIContractDrift `json:"contract_drift"`
}

type APIContractDrift struct {
	Code      string `json:"code"`
	Model     string `json:"model"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}

// APIAssetSummary contains aggregate counts for the workbench.
type APIAssetSummary struct {
	EndpointCount int `json:"endpoint_count"`
	SchemaCount   int `json:"schema_count"`
	ModuleCount   int `json:"module_count"`
	SurfaceCount  int `json:"surface_count"`
}

// APIModuleSummary counts APIs discovered under one business module.
type APIModuleSummary struct {
	Name          string `json:"name"`
	Label         string `json:"label"`
	EndpointCount int    `json:"endpoint_count"`
	ModelCount    int    `json:"model_count"`
}

// APISurfaceCount counts APIs by product surface.
type APISurfaceCount struct {
	Surface       APISurface `json:"surface"`
	EndpointCount int        `json:"endpoint_count"`
}

// APIEndpointAsset is a normalized OpenAPI operation linked back to platform concepts.
type APIEndpointAsset struct {
	ID                 string            `json:"id"`
	Method             string            `json:"method"`
	Path               string            `json:"path"`
	OperationID        string            `json:"operation_id,omitempty"`
	DisplayName        string            `json:"display_name,omitempty"`
	Summary            string            `json:"summary,omitempty"`
	Description        string            `json:"description,omitempty"`
	Tags               []string          `json:"tags"`
	Surface            APISurface        `json:"surface"`
	Module             string            `json:"module,omitempty"`
	ModuleLabel        string            `json:"module_label,omitempty"`
	ModuleOverridden   bool              `json:"module_overridden"`
	BusinessModels     []string          `json:"business_models"`
	RequestSchemas     []string          `json:"request_schemas"`
	ResponseSchemas    []string          `json:"response_schemas"`
	AuthRequired       bool              `json:"auth_required"`
	Parameters         []ParameterDetail `json:"parameters"`
	RequestBodyExample json.RawMessage   `json:"request_body_example,omitempty"`
	Status             string            `json:"status"`
	SourceHash         string            `json:"source_hash"`
}

// ParameterDetail describes an OpenAPI operation parameter including its resolved type.
type ParameterDetail struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// APISchemaAsset is a lightweight schema discovered from OpenAPI definitions/components.
type APISchemaAsset struct {
	Name        string   `json:"name"`
	Module      string   `json:"module,omitempty"`
	Model       string   `json:"model,omitempty"`
	Kind        string   `json:"kind"`
	Fields      []string `json:"fields"`
	EndpointIDs []string `json:"endpoint_ids"`
}

type apiEndpointOverride struct {
	ModuleID    string
	DisplayName string
	Description string
}

type openAPIDocument struct {
	Swagger     string                          `json:"swagger"`
	OpenAPI     string                          `json:"openapi"`
	Info        openAPIInfo                     `json:"info"`
	BasePath    string                          `json:"basePath"`
	Paths       map[string]map[string]operation `json:"paths"`
	Definitions map[string]schemaRef            `json:"definitions"`
	Components  struct {
		Schemas map[string]schemaRef `json:"schemas"`
	} `json:"components"`
}

type openAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type operation struct {
	OperationID string                 `json:"operationId"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Parameters  []parameter            `json:"parameters"`
	RequestBody *requestBody           `json:"requestBody"`
	Responses   map[string]apiResponse `json:"responses"`
	Security    []map[string][]string  `json:"security"`
	Extensions  map[string]any         `json:"-"`
}

type operationAlias operation

func (o *operation) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var alias operationAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*o = operation(alias)
	o.Extensions = map[string]any{}
	for key, value := range raw {
		if strings.HasPrefix(key, "x-") {
			var decoded any
			if err := json.Unmarshal(value, &decoded); err == nil {
				o.Extensions[key] = decoded
			}
		}
	}
	return nil
}

type parameter struct {
	Name   string    `json:"name"`
	In     string    `json:"in"`
	Schema schemaRef `json:"schema"`
}

type requestBody struct {
	Content map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema schemaRef `json:"schema"`
}

type apiResponse struct {
	Schema  schemaRef            `json:"schema"`
	Content map[string]mediaType `json:"content"`
}

type schemaRef struct {
	Ref                  string               `json:"$ref"`
	Type                 string               `json:"type"`
	Description          string               `json:"description"`
	Items                *schemaRef           `json:"items"`
	Properties           map[string]schemaRef `json:"properties"`
	AdditionalProperties any                  `json:"additionalProperties"`
}

// ListAPIAssets parses generated OpenAPI output and returns a platform API asset index.
func (s *Service) ListAPIAssets(ctx context.Context) (*APIAssetIndex, error) {
	docPath, err := findOpenAPIPath()
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(docPath)
	if err != nil {
		return nil, fmt.Errorf("read openapi document: %w", err)
	}

	var doc openAPIDocument
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi document: %w", err)
	}

	var project *mozi.ProjectIR
	if s != nil && s.Store != nil {
		project, _ = s.Store.LoadProject()
	}
	modules := buildModuleLookup(project)
	overrides := map[string]apiEndpointOverride{}
	if s != nil && s.Store != nil {
		if dbOverrides, err := s.Store.ListAPIEndpointOverrides(); err == nil {
			for id, item := range dbOverrides {
				overrides[id] = apiEndpointOverride{
					ModuleID:    item.ModuleID,
					DisplayName: item.DisplayName,
					Description: item.Description,
				}
			}
		}
	}

	schemas := doc.Definitions
	if len(schemas) == 0 {
		schemas = doc.Components.Schemas
	}

	index := &APIAssetIndex{
		Source:      filepath.ToSlash(docPath),
		Title:       doc.Info.Title,
		Version:     doc.Info.Version,
		BasePath:    doc.BasePath,
		GeneratedBy: "openapi",
	}

	schemaAssets := make(map[string]*APISchemaAsset, len(schemas))
	for name, schema := range schemas {
		fields := make([]string, 0, len(schema.Properties))
		for field := range schema.Properties {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		module, model := inferSchemaModel(name, modules)
		schemaAssets[name] = &APISchemaAsset{
			Name:   name,
			Module: module,
			Model:  model,
			Kind:   inferSchemaKind(name),
			Fields: fields,
		}
	}

	moduleCounts := map[string]int{}
	moduleModelRefs := map[string]map[string]bool{}
	surfaceCounts := map[APISurface]int{}

	for path, methods := range doc.Paths {
		for method, op := range methods {
			method = strings.ToUpper(method)
			if !isHTTPMethod(method) {
				continue
			}
			requestSchemas := collectRequestSchemas(op)
			responseSchemas := collectResponseSchemas(op)
			allSchemas := append(append([]string{}, requestSchemas...), responseSchemas...)
			moduleName, moduleLabel := inferEndpointModule(path, op.Tags, allSchemas, modules)
			models := inferBusinessModels(allSchemas, moduleName, schemaAssets)
			surface := inferSurface(path, op.Tags, op.Extensions)
			id := endpointID(method, path, op)
			displayName := op.Summary
			description := op.Description
			moduleOverridden := false
			if override, ok := overrides[id]; ok {
				if override.ModuleID != "" {
					moduleName = override.ModuleID
					moduleLabel = modules.label(moduleName)
					models = inferBusinessModels(allSchemas, moduleName, schemaAssets)
					moduleOverridden = true
				}
				if override.DisplayName != "" {
					displayName = override.DisplayName
				}
				if override.Description != "" {
					description = override.Description
				}
			}
			parameters := extractParameters(op, schemas)
			reqBodyExample := generateRequestBodyExample(op, schemas)

			endpoint := APIEndpointAsset{
				ID:                 id,
				Method:             method,
				Path:               path,
				OperationID:        op.OperationID,
				DisplayName:        displayName,
				Summary:            op.Summary,
				Description:        description,
				Tags:               op.Tags,
				Surface:            surface,
				Module:             moduleName,
				ModuleLabel:        moduleLabel,
				ModuleOverridden:   moduleOverridden,
				BusinessModels:     models,
				RequestSchemas:     requestSchemas,
				ResponseSchemas:    responseSchemas,
				AuthRequired:       len(op.Security) > 0,
				Parameters:         parameters,
				RequestBodyExample: reqBodyExample,
				Status:             "synced",
			}
			endpoint.SourceHash = hashEndpoint(endpoint)
			index.Endpoints = append(index.Endpoints, endpoint)

			surfaceCounts[surface]++
			if moduleName != "" {
				moduleCounts[moduleName]++
				if moduleModelRefs[moduleName] == nil {
					moduleModelRefs[moduleName] = map[string]bool{}
				}
				for _, model := range models {
					moduleModelRefs[moduleName][model] = true
				}
			}
			for _, schema := range allSchemas {
				if asset := schemaAssets[schema]; asset != nil {
					asset.EndpointIDs = append(asset.EndpointIDs, endpoint.ID)
				}
			}
		}
	}

	sort.Slice(index.Endpoints, func(i, j int) bool {
		if index.Endpoints[i].Path == index.Endpoints[j].Path {
			return index.Endpoints[i].Method < index.Endpoints[j].Method
		}
		return index.Endpoints[i].Path < index.Endpoints[j].Path
	})

	for name, count := range moduleCounts {
		index.Modules = append(index.Modules, APIModuleSummary{
			Name:          name,
			Label:         modules.label(name),
			EndpointCount: count,
			ModelCount:    len(moduleModelRefs[name]),
		})
	}
	for _, mod := range modules.byName {
		if mod == nil {
			continue
		}
		if _, ok := moduleCounts[mod.Name]; ok {
			continue
		}
		index.Modules = append(index.Modules, APIModuleSummary{
			Name:          mod.Name,
			Label:         modules.label(mod.Name),
			EndpointCount: 0,
			ModelCount:    len(mod.Models),
		})
	}
	sort.Slice(index.Modules, func(i, j int) bool { return index.Modules[i].Name < index.Modules[j].Name })

	for surface, count := range surfaceCounts {
		index.Surfaces = append(index.Surfaces, APISurfaceCount{Surface: surface, EndpointCount: count})
	}
	sort.Slice(index.Surfaces, func(i, j int) bool { return index.Surfaces[i].Surface < index.Surfaces[j].Surface })

	for _, asset := range schemaAssets {
		sort.Strings(asset.EndpointIDs)
		index.SchemaModels = append(index.SchemaModels, *asset)
	}
	sort.Slice(index.SchemaModels, func(i, j int) bool { return index.SchemaModels[i].Name < index.SchemaModels[j].Name })

	index.Summary = APIAssetSummary{
		EndpointCount: len(index.Endpoints),
		SchemaCount:   len(index.SchemaModels),
		ModuleCount:   len(index.Modules),
		SurfaceCount:  len(index.Surfaces),
	}
	index.ContractDrift = checkAPIContractDrift(project, index.Endpoints)
	return index, nil
}

func checkAPIContractDrift(project *mozi.ProjectIR, endpoints []APIEndpointAsset) []APIContractDrift {
	if project == nil {
		return nil
	}
	var drift []APIContractDrift
	for _, mod := range project.Modules {
		for _, model := range mod.Models {
			ref := mod.Name + "/" + model.Name
			if len(model.APIIntent.Operations) == 0 {
				continue
			}
			matched := map[string]bool{}
			for _, endpoint := range endpoints {
				if endpointMentionsModel(endpoint, ref, model.Name) {
					matched[strings.ToLower(endpoint.Method)] = true
					matched[strings.ToLower(endpoint.OperationID)] = true
				}
			}
			for _, operation := range model.APIIntent.Operations {
				key := strings.ToLower(strings.TrimSpace(operation))
				if !matched[key] && !matched[operationMethod(key)] {
					drift = append(drift, APIContractDrift{Code: "missing-openapi-operation", Model: ref, Operation: operation, Message: "API intent operation is not represented in OpenAPI"})
				}
			}
		}
	}
	sort.Slice(drift, func(i, j int) bool {
		if drift[i].Model != drift[j].Model {
			return drift[i].Model < drift[j].Model
		}
		return drift[i].Operation < drift[j].Operation
	})
	return drift
}

func operationMethod(operation string) string {
	switch operation {
	case "list", "get", "read": return "get"
	case "create": return "post"
	case "update": return "put"
	case "delete": return "delete"
	default: return ""
	}
}

func endpointMentionsModel(endpoint APIEndpointAsset, ref, name string) bool {
	for _, model := range endpoint.BusinessModels {
		if model == ref || model == name || strings.HasSuffix(model, "/"+name) {
			return true
		}
	}
	return false
}

type moduleLookup struct {
	byName   map[string]*mozi.ModuleIR
	byPrefix map[string]*mozi.ModuleIR
	byLabel  map[string]*mozi.ModuleIR
	models   map[string]string
}

func buildModuleLookup(project *mozi.ProjectIR) moduleLookup {
	lookup := moduleLookup{
		byName:   map[string]*mozi.ModuleIR{},
		byPrefix: map[string]*mozi.ModuleIR{},
		byLabel:  map[string]*mozi.ModuleIR{},
		models:   map[string]string{},
	}
	if project == nil {
		return lookup
	}
	for _, mod := range project.Modules {
		if mod == nil {
			continue
		}
		lookup.byName[strings.ToLower(mod.Name)] = mod
		lookup.byLabel[strings.ToLower(mod.Label)] = mod
		if mod.APIPrefix != "" {
			lookup.byPrefix[strings.ToLower(strings.Trim(mod.APIPrefix, "/"))] = mod
		}
		for _, model := range mod.Models {
			if model != nil {
				lookup.models[strings.ToLower(model.Name)] = mod.Name
			}
		}
	}
	return lookup
}

func (m moduleLookup) label(name string) string {
	if mod := m.byName[strings.ToLower(name)]; mod != nil && mod.Label != "" {
		return mod.Label
	}
	return name
}

func findOpenAPIPath() (string, error) {
	candidates := []string{
		filepath.Join("docs", "swagger.json"),
		filepath.Join("docs", "openapi.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("openapi document not found; expected docs/swagger.json")
}

// extractParameters flattens operation parameters into a frontend-friendly form,
// resolving $ref schemas where possible.
func extractParameters(op operation, schemas map[string]schemaRef) []ParameterDetail {
	var out []ParameterDetail
	for _, param := range op.Parameters {
		resolved := resolveSchema(param.Schema, schemas)
		pd := ParameterDetail{
			Name:        param.Name,
			In:          param.In,
			Required:    true, // OpenAPI 2.0 params don't have an explicit required; body params are required
			Type:        resolved.Type,
			Description: resolved.Description,
		}
		if pd.Type == "" {
			pd.Type = "string"
		}
		out = append(out, pd)
	}
	// Also collect body parameters: if there's a body param, unwrap its schema properties
	for _, param := range op.Parameters {
		if param.In != "body" {
			continue
		}
		resolved := resolveSchema(param.Schema, schemas)
		for propName, propSchema := range resolved.Properties {
			propResolved := resolveSchema(propSchema, schemas)
			propType := propResolved.Type
			if propType == "" {
				propType = "string"
			}
			out = append(out, ParameterDetail{
				Name:        propName,
				In:          "body",
				Required:    false,
				Type:        propType,
				Description: propResolved.Description,
			})
		}
	}
	return out
}

// resolveSchema follows a $ref or returns the schema itself.
func resolveSchema(s schemaRef, schemas map[string]schemaRef) schemaRef {
	if s.Ref != "" {
		name := schemaNameFromRef(s.Ref)
		if target, ok := schemas[name]; ok {
			return target
		}
	}
	return s
}

// generateRequestBodyExample creates an example JSON from the first body parameter or requestBody schema.
func generateRequestBodyExample(op operation, schemas map[string]schemaRef) json.RawMessage {
	// First, check for a body parameter (OpenAPI 2.0 style)
	for _, param := range op.Parameters {
		if param.In == "body" {
			resolved := resolveSchema(param.Schema, schemas)
			example := generateExample(resolved, schemas, 0)
			if example != nil {
				raw, err := json.Marshal(example)
				if err == nil && string(raw) != "null" {
					return raw
				}
			}
		}
	}
	// Then check requestBody (OpenAPI 3.0 style)
	if op.RequestBody != nil {
		for _, media := range op.RequestBody.Content {
			if media.Schema.Ref != "" || len(media.Schema.Properties) > 0 {
				resolved := resolveSchema(media.Schema, schemas)
				example := generateExample(resolved, schemas, 0)
				if example != nil {
					raw, err := json.Marshal(example)
					if err == nil && string(raw) != "null" {
						return raw
					}
				}
				break
			}
		}
	}
	return nil
}

// generateExample recurses through a schema to produce a type-appropriate default value.
// Depth is capped at 5 to avoid infinite recursion on cyclic schemas.
func generateExample(s schemaRef, schemas map[string]schemaRef, depth int) any {
	if depth > 5 {
		return nil
	}
	resolved := resolveSchema(s, schemas)

	// Object with properties
	if len(resolved.Properties) > 0 {
		obj := make(map[string]any, len(resolved.Properties))
		for key, prop := range resolved.Properties {
			obj[key] = generateExample(prop, schemas, depth+1)
		}
		return obj
	}

	// Array type
	if resolved.Type == "array" {
		if resolved.Items != nil {
			itemExample := generateExample(*resolved.Items, schemas, depth+1)
			return []any{itemExample}
		}
		return []any{"string"}
	}

	// Scalar types
	switch resolved.Type {
	case "integer", "int":
		return 0
	case "number", "float":
		return 0.0
	case "boolean", "bool":
		return false
	case "string":
		return ""
	default:
		return ""
	}
}

func collectRequestSchemas(op operation) []string {
	seen := map[string]bool{}
	for _, param := range op.Parameters {
		collectSchemaNames(param.Schema, seen)
	}
	if op.RequestBody != nil {
		for _, media := range op.RequestBody.Content {
			collectSchemaNames(media.Schema, seen)
		}
	}
	return sortedKeys(seen)
}

func collectResponseSchemas(op operation) []string {
	seen := map[string]bool{}
	for _, resp := range op.Responses {
		collectSchemaNames(resp.Schema, seen)
		for _, media := range resp.Content {
			collectSchemaNames(media.Schema, seen)
		}
	}
	return sortedKeys(seen)
}

func collectSchemaNames(schema schemaRef, seen map[string]bool) {
	if schema.Ref != "" {
		seen[schemaNameFromRef(schema.Ref)] = true
	}
	if schema.Items != nil {
		collectSchemaNames(*schema.Items, seen)
	}
	for _, prop := range schema.Properties {
		collectSchemaNames(prop, seen)
	}
}

func schemaNameFromRef(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func inferEndpointModule(path string, tags, schemas []string, modules moduleLookup) (string, string) {
	segments := pathSegments(path)
	for _, segment := range segments {
		if mod := modules.byPrefix[strings.ToLower(segment)]; mod != nil {
			return mod.Name, mod.Label
		}
		if mod := modules.byName[strings.ToLower(segment)]; mod != nil {
			return mod.Name, mod.Label
		}
	}
	for _, tag := range tags {
		if mod := modules.byLabel[strings.ToLower(tag)]; mod != nil {
			return mod.Name, mod.Label
		}
		if mod := modules.byName[strings.ToLower(tag)]; mod != nil {
			return mod.Name, mod.Label
		}
	}
	for _, schema := range schemas {
		_, model := splitSchemaName(schema)
		if moduleName := modules.models[strings.ToLower(model)]; moduleName != "" {
			return moduleName, modules.label(moduleName)
		}
	}
	if len(tags) > 0 {
		return slugify(tags[0]), tags[0]
	}
	return "", ""
}

func inferSurface(path string, tags []string, extensions map[string]any) APISurface {
	if value, ok := extensions["x-api-surface"].(string); ok && value != "" {
		return APISurface(value)
	}
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "/admin-api/") || strings.HasPrefix(lower, "/api/admin"):
		return APISurfaceAdmin
	case strings.Contains(lower, "/miniapp-api/") || strings.Contains(lower, "/wechat"):
		return APISurfaceMiniApp
	case strings.Contains(lower, "/desktop-api/"):
		return APISurfaceDesktop
	case strings.Contains(lower, "/internal-api/") || strings.HasPrefix(lower, "/api/dev-platform"):
		return APISurfaceInternal
	case strings.Contains(lower, "/client-api/"):
		return APISurfaceClientShared
	}
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		if strings.Contains(tagLower, "admin") || strings.Contains(tag, "管理") {
			return APISurfaceAdmin
		}
	}
	return APISurfaceClientShared
}

func inferBusinessModels(schemas []string, moduleName string, assets map[string]*APISchemaAsset) []string {
	seen := map[string]bool{}
	for _, schema := range schemas {
		asset := assets[schema]
		if asset == nil || asset.Model == "" {
			continue
		}
		ref := asset.Model
		if asset.Module != "" {
			ref = asset.Module + "/" + asset.Model
		} else if moduleName != "" {
			ref = moduleName + "/" + asset.Model
		}
		seen[ref] = true
	}
	return sortedKeys(seen)
}

func inferSchemaModel(name string, modules moduleLookup) (string, string) {
	module, model := splitSchemaName(name)
	if model == "" {
		return "", ""
	}
	if moduleName := modules.models[strings.ToLower(model)]; moduleName != "" {
		return moduleName, model
	}
	if module != "" {
		return module, model
	}
	return "", model
}

func splitSchemaName(name string) (string, string) {
	parts := strings.Split(name, ".")
	model := parts[len(parts)-1]
	for _, suffix := range []string{"Req", "Request", "Resp", "Response", "DTO"} {
		model = strings.TrimSuffix(model, suffix)
	}
	if len(parts) > 1 {
		return parts[len(parts)-2], model
	}
	return "", model
}

func inferSchemaKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "req") || strings.Contains(lower, "request"):
		return "request"
	case strings.Contains(lower, "resp") || strings.Contains(lower, "response"):
		return "response"
	case strings.HasPrefix(lower, "ent."):
		return "entity"
	default:
		return "schema"
	}
}

func pathSegments(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "api" || strings.HasPrefix(part, ":") || strings.HasPrefix(part, "{") {
			continue
		}
		out = append(out, part)
	}
	return out
}

func endpointID(method, path string, op operation) string {
	if value, ok := op.Extensions["x-platform-api-id"].(string); ok && value != "" {
		return value
	}
	if op.OperationID != "" {
		return op.OperationID
	}
	return slugify(strings.ToLower(method) + "-" + path)
}

func hashEndpoint(endpoint APIEndpointAsset) string {
	payload := strings.Join([]string{
		endpoint.Method,
		endpoint.Path,
		strings.Join(endpoint.RequestSchemas, ","),
		strings.Join(endpoint.ResponseSchemas, ","),
		string(endpoint.Surface),
	}, "|")
	sum := sha1.Sum([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
