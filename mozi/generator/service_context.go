package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// ServiceTemplateContext is the context object passed to service templates
// (go-zero .api files, handler skeletons). It carries the ServiceIR plus
// pre-computed per-field Go types so templates never re-derive type mappings.
type ServiceTemplateContext struct {
	Service *mozi.ServiceIR

	Name   string // PascalCase: ContentService
	Module string // content
	Domain string
	Label  string

	Messages   []ServiceMessageContext
	HTTP       []ServiceRouteContext
	PublicHTTP []ServiceRouteContext // routes with auth == "public"
	AuthedHTTP []ServiceRouteContext // routes requiring jwt/admin
	RPC        []mozi.RPCMethodIR
}

// ServiceMessageContext is one message with render-ready fields.
type ServiceMessageContext struct {
	Name        string
	Description string
	Fields      []ServiceFieldContext
}

// ServiceFieldContext is one message field with its resolved Go type.
type ServiceFieldContext struct {
	Name     string // snake_case, e.g. review_count
	JSONName string // camelCase, e.g. reviewCount
	GoType   string // string, int, []DeckSummary, ...
	Repeated bool
	Number   int32
}

// ServiceRouteContext is one HTTP route with render-ready flags.
type ServiceRouteContext struct {
	Name       string
	Method     string
	Path       string
	Request    string
	Response   string
	HasRequest bool
	Auth       string
}

// BuildServiceContext pre-computes everything service templates need.
func BuildServiceContext(svc *mozi.ServiceIR) *ServiceTemplateContext {
	ctx := &ServiceTemplateContext{
		Service: svc,
		Name:    svc.Name,
		Module:  svc.Module,
		Domain:  svc.Domain,
		Label:   svc.Label,
		RPC:     svc.RPC,
	}
	for _, m := range svc.Messages {
		mc := ServiceMessageContext{Name: m.Name, Description: m.Description}
		for _, f := range m.Fields {
			mc.Fields = append(mc.Fields, ServiceFieldContext{
				Name:     f.Name,
				JSONName: snakeToCamel(f.Name),
				GoType:   messageFieldGoType(f),
				Repeated: f.Repeated,
				Number:   f.Number,
			})
		}
		ctx.Messages = append(ctx.Messages, mc)
	}
	for _, r := range svc.HTTP {
		rc := ServiceRouteContext{
			Name: r.Name, Method: strings.ToUpper(r.Method), Path: r.Path,
			Request: r.Request, Response: r.Response, HasRequest: r.Request != "",
			Auth: r.Auth,
		}
		ctx.HTTP = append(ctx.HTTP, rc)
		if r.Auth == "public" {
			ctx.PublicHTTP = append(ctx.PublicHTTP, rc)
		} else {
			ctx.AuthedHTTP = append(ctx.AuthedHTTP, rc)
		}
	}
	return ctx
}

// messageFieldGoType resolves a message field type to the Go type used in
// generated request/response structs.
func messageFieldGoType(f mozi.MessageFieldIR) string {
	base := ""
	if strings.HasPrefix(f.Type, mozi.MessageRefPrefix) {
		base = strings.TrimPrefix(f.Type, mozi.MessageRefPrefix)
	} else if strings.HasPrefix(f.Type, mozi.ModelRefPrefix) {
		target := strings.TrimPrefix(f.Type, mozi.ModelRefPrefix)
		if i := strings.LastIndex(target, "/"); i >= 0 {
			target = target[i+1:]
		}
		base = target
	} else {
		base = mozi.FieldType(f.Type).GoType()
	}
	if f.Repeated {
		return "[]" + base
	}
	return base
}

// ExecuteService runs a service template against the given ServiceIR.
func (e *Engine) ExecuteService(templateName string, svc *mozi.ServiceIR) (string, error) {
	tmplContent, err := fs.ReadFile(e.templateFS, templateName)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templateName, err)
	}
	tmpl, err := template.New(templateName).Delims("[[", "]]").Funcs(e.funcMap).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", templateName, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, BuildServiceContext(svc)); err != nil {
		return "", fmt.Errorf("execute template %s: %w", templateName, err)
	}
	return buf.String(), nil
}
