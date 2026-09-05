// Package service provides validation and proto field-numbering rules for
// mozi.ServiceIR documents. Field numbers are part of the gRPC wire contract:
// they are stored explicitly, never reused after deletion, and changes to
// existing numbers are always breaking.
package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// Proto field number bounds. 19000–19999 are reserved by the protobuf
// implementation itself and must never be assigned.
const (
	MaxFieldNumber       int32 = 536870911 // 2^29 - 1
	DescriptorRangeStart int32 = 19000
	DescriptorRangeEnd   int32 = 19999
)

var (
	pascalCaseRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	fieldNameRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	errorCodeRe  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	modelRefRe   = regexp.MustCompile(`^([a-z][a-z0-9_]*/)?[A-Z][A-Za-z0-9]*$`)
)

// ValidationError represents a single ServiceIR validation issue.
type ValidationError struct {
	Service string `json:"service"`
	Where   string `json:"where,omitempty"` // message/field/route context
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	if e.Where != "" {
		return fmt.Sprintf("[%s %s] %s", e.Service, e.Where, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Service, e.Message)
}

// ValidationResult holds the result of validating a ServiceIR.
type ValidationResult struct {
	Valid    bool               `json:"valid"`
	Errors   []*ValidationError `json:"errors,omitempty"`
	Warnings []*ValidationError `json:"warnings,omitempty"`
}

// NextNumber returns the next assignable field number for a message:
// one past the highest of current field numbers and reserved numbers.
func NextNumber(msg *mozi.MessageIR) (int32, error) {
	var max int32
	for _, f := range msg.Fields {
		if f.Number > max {
			max = f.Number
		}
	}
	for _, n := range msg.ReservedNumbers {
		if n > max {
			max = n
		}
	}
	next := max + 1
	if next > MaxFieldNumber {
		return 0, fmt.Errorf("message %s: field numbers exhausted", msg.Name)
	}
	if next >= DescriptorRangeStart && next <= DescriptorRangeEnd {
		next = DescriptorRangeEnd + 1
	}
	return next, nil
}

// Validate checks a ServiceIR for structural and numbering correctness.
// Cross-model references (model: module/Model) are checked for format only;
// target resolution happens at lint time with the full project available.
func Validate(svc *mozi.ServiceIR) *ValidationResult {
	result := &ValidationResult{Valid: true}
	ref := svc.Module + "/" + svc.Name
	add := func(where, msg string) {
		result.Errors = append(result.Errors, &ValidationError{Service: ref, Where: where, Message: msg})
		result.Valid = false
	}
	warn := func(where, msg string) {
		result.Warnings = append(result.Warnings, &ValidationError{Service: ref, Where: where, Message: msg})
	}

	if svc.Name == "" {
		add("", "service name is required")
	} else if !pascalCaseRe.MatchString(svc.Name) {
		add("", fmt.Sprintf("service name must be PascalCase: %s", svc.Name))
	}
	if svc.Module == "" {
		add("", "module is required")
	} else if !moduleNameRe.MatchString(svc.Module) {
		add("", fmt.Sprintf("module must be snake_case: %s", svc.Module))
	}
	if len(svc.HTTP) == 0 && len(svc.RPC) == 0 {
		warn("", "service exposes no operations")
	}

	msgNames := make(map[string]bool)
	for i := range svc.Messages {
		validateMessage(&svc.Messages[i], msgNames, add, warn)
	}

	validateHTTP(svc, add)
	validateRPC(svc, add)
	return result
}

func validateMessage(msg *mozi.MessageIR, msgNames map[string]bool, add, warn func(where, msg string)) {
	where := msg.Name
	if msg.Name == "" {
		add("", "message name is required")
		return
	}
	if !pascalCaseRe.MatchString(msg.Name) {
		add(where, fmt.Sprintf("message name must be PascalCase: %s", msg.Name))
	}
	if msgNames[msg.Name] {
		add(where, "duplicate message name")
	}
	msgNames[msg.Name] = true

	reservedNums := make(map[int32]bool)
	for _, n := range msg.ReservedNumbers {
		if err := checkNumber(n); err != nil {
			add(where, fmt.Sprintf("reserved number %d: %s", n, err))
		}
		if reservedNums[n] {
			add(where, fmt.Sprintf("duplicate reserved number %d", n))
		}
		reservedNums[n] = true
	}
	reservedNames := make(map[string]bool)
	for _, n := range msg.ReservedNames {
		if reservedNames[n] {
			add(where, fmt.Sprintf("duplicate reserved name %q", n))
		}
		reservedNames[n] = true
	}

	seenNames := make(map[string]bool)
	seenNums := make(map[int32]string)
	for _, f := range msg.Fields {
		fw := where + "." + f.Name
		if f.Name == "" {
			add(where, "field name is required")
			continue
		}
		if !fieldNameRe.MatchString(f.Name) {
			add(fw, fmt.Sprintf("field name must be snake_case: %s", f.Name))
		}
		if seenNames[f.Name] {
			add(fw, "duplicate field name")
		}
		seenNames[f.Name] = true

		if reservedNames[f.Name] {
			add(fw, fmt.Sprintf("field name %q is reserved from a deleted field and must not be reused", f.Name))
		}
		if err := checkNumber(f.Number); err != nil {
			add(fw, err.Error())
		} else {
			if prev, ok := seenNums[f.Number]; ok {
				add(fw, fmt.Sprintf("field number %d already used by %q", f.Number, prev))
			}
			seenNums[f.Number] = f.Name
			if reservedNums[f.Number] {
				add(fw, fmt.Sprintf("field number %d is reserved from a deleted field and must not be reused", f.Number))
			}
		}

		validateFieldType(fw, f.Type, add)

		if f.RenamedFrom != "" {
			if seenNames[f.RenamedFrom] {
				add(fw, fmt.Sprintf("renamed_from %q collides with an existing field", f.RenamedFrom))
			}
			if !reservedNames[f.RenamedFrom] {
				warn(fw, fmt.Sprintf("renamed_from %q is not in reserved_names; reserve the old name to prevent accidental reuse", f.RenamedFrom))
			}
		}
	}
}

func validateFieldType(where, typ string, add func(where, msg string)) {
	if strings.HasPrefix(typ, mozi.MessageRefPrefix) {
		name := strings.TrimPrefix(typ, mozi.MessageRefPrefix)
		if !pascalCaseRe.MatchString(name) {
			add(where, fmt.Sprintf("invalid message reference %q", typ))
		}
		return
	}
	if strings.HasPrefix(typ, mozi.ModelRefPrefix) {
		target := strings.TrimPrefix(typ, mozi.ModelRefPrefix)
		if !modelRefRe.MatchString(target) {
			add(where, fmt.Sprintf("invalid model reference %q (want model:Model or model:module/Model)", typ))
		}
		return
	}
	switch mozi.FieldType(typ) {
	case mozi.FieldTypeString, mozi.FieldTypeInt, mozi.FieldTypeFloat, mozi.FieldTypeBool,
		mozi.FieldTypeTime, mozi.FieldTypeText, mozi.FieldTypeEnum, mozi.FieldTypeJSON:
	default:
		add(where, fmt.Sprintf("invalid field type: %s", typ))
	}
}

func validateHTTP(svc *mozi.ServiceIR, add func(where, msg string)) {
	seen := make(map[string]bool)
	for _, r := range svc.HTTP {
		where := "http:" + r.Name
		if r.Name == "" {
			add("http", "route name is required")
			continue
		}
		if seen[r.Name] {
			add(where, "duplicate route name")
		}
		seen[r.Name] = true

		switch r.Method {
		case "GET", "POST", "PUT", "DELETE", "PATCH":
		default:
			add(where, fmt.Sprintf("invalid method %q (must be GET, POST, PUT, DELETE, or PATCH)", r.Method))
		}
		if !strings.HasPrefix(r.Path, "/") {
			add(where, fmt.Sprintf("path must start with /: %q", r.Path))
		}
		if r.Auth != "" && r.Auth != "jwt" && r.Auth != "admin" && r.Auth != "public" {
			add(where, fmt.Sprintf("invalid auth %q (must be jwt, admin, or public)", r.Auth))
		}
		if r.Request != "" && svc.GetMessage(r.Request) == nil {
			add(where, fmt.Sprintf("request message %q not defined", r.Request))
		}
		if r.Response == "" {
			add(where, "response message is required")
		} else if svc.GetMessage(r.Response) == nil {
			add(where, fmt.Sprintf("response message %q not defined", r.Response))
		}
		for _, code := range r.ErrorCodes {
			if !errorCodeRe.MatchString(code) {
				add(where, fmt.Sprintf("invalid error code %q (want UPPER_SNAKE)", code))
			}
		}
	}
}

func validateRPC(svc *mozi.ServiceIR, add func(where, msg string)) {
	seen := make(map[string]bool)
	for _, m := range svc.RPC {
		where := "rpc:" + m.Name
		if m.Name == "" {
			add("rpc", "method name is required")
			continue
		}
		if seen[m.Name] {
			add(where, "duplicate method name")
		}
		seen[m.Name] = true
		if m.Request == "" {
			add(where, "request message is required")
		} else if svc.GetMessage(m.Request) == nil {
			add(where, fmt.Sprintf("request message %q not defined", m.Request))
		}
		if m.Response == "" {
			add(where, "response message is required")
		} else if svc.GetMessage(m.Response) == nil {
			add(where, fmt.Sprintf("response message %q not defined", m.Response))
		}
	}
}

// checkNumber enforces proto field-number bounds and the descriptor range.
func checkNumber(n int32) error {
	if n < 1 || n > MaxFieldNumber {
		return fmt.Errorf("field number %d out of range [1, %d]", n, MaxFieldNumber)
	}
	if n >= DescriptorRangeStart && n <= DescriptorRangeEnd {
		return fmt.Errorf("field number %d is in the reserved descriptor range 19000-19999", n)
	}
	return nil
}
