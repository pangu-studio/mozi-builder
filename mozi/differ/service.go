package differ

import (
	"fmt"
	"sort"

	"github.com/pangu-studio/mozi-builder/mozi"
)

// Service diff categories.
const (
	ServiceCategoryMeta         = "meta"
	ServiceCategoryMessage      = "message"
	ServiceCategoryMessageField = "message_field"
	ServiceCategoryHTTP         = "http"
	ServiceCategoryRPC          = "rpc"
)

// CompareServices compares two ServiceIR revisions and classifies each change
// by wire/API compatibility. Field-number stability rules:
//
//   - added field                      → safe
//   - removed field, number+name kept in reserved → conditional
//   - removed field without reserving  → breaking
//   - existing field number changed    → breaking (always)
//   - rename via renamed_from, number unchanged → safe
//   - type change: int→float, string→text → conditional; otherwise breaking
//   - route/method removal or path/method change → breaking
func CompareServices(from, to *mozi.ServiceIR) *DiffResult {
	ref := to.Module + "/" + to.Name
	result := &DiffResult{ModelRef: ref}

	changes := compareServiceMeta(from, to)
	changes = append(changes, compareMessages(from, to)...)
	changes = append(changes, compareHTTPRoutes(from.HTTP, to.HTTP)...)
	changes = append(changes, compareRPCMethods(from.RPC, to.RPC)...)

	result.Changes = changes
	result.HasChanges = len(changes) > 0
	return result
}

func compareServiceMeta(from, to *mozi.ServiceIR) []FieldChange {
	var changes []FieldChange
	if from.Label != to.Label {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: ServiceCategoryMeta, Name: "label",
			Compatibility: CompatibilitySafe,
			Detail:        fmt.Sprintf("~ label changed: %s → %s", from.Label, to.Label),
			OldValue:      from.Label, NewValue: to.Label,
		})
	}
	if from.Description != to.Description {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: ServiceCategoryMeta, Name: "description",
			Compatibility: CompatibilitySafe,
			Detail:        "~ description changed",
		})
	}
	if from.Domain != to.Domain {
		changes = append(changes, FieldChange{
			Type: ChangeModified, Category: ServiceCategoryMeta, Name: "domain",
			Compatibility: CompatibilityConditional,
			Detail:        fmt.Sprintf("~ domain changed: %s → %s", from.Domain, to.Domain),
			OldValue:      from.Domain, NewValue: to.Domain,
		})
	}
	return changes
}

func compareMessages(from, to *mozi.ServiceIR) []FieldChange {
	var changes []FieldChange
	fromMsgs := make(map[string]*mozi.MessageIR)
	for i := range from.Messages {
		fromMsgs[from.Messages[i].Name] = &from.Messages[i]
	}
	toMsgs := make(map[string]*mozi.MessageIR)
	for i := range to.Messages {
		toMsgs[to.Messages[i].Name] = &to.Messages[i]
	}

	for _, name := range sortedKeys(fromMsgs) {
		if _, ok := toMsgs[name]; !ok {
			changes = append(changes, FieldChange{
				Type: ChangeRemoved, Category: ServiceCategoryMessage, Name: name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("- message removed: %s", name),
			})
		}
	}
	for _, name := range sortedKeys(toMsgs) {
		fm, ok := fromMsgs[name]
		if !ok {
			changes = append(changes, FieldChange{
				Type: ChangeAdded, Category: ServiceCategoryMessage, Name: name,
				Compatibility: CompatibilitySafe,
				Detail:        fmt.Sprintf("+ message added: %s", name),
			})
			continue
		}
		changes = append(changes, compareMessageFields(name, fm, toMsgs[name])...)
	}
	return changes
}

func compareMessageFields(msgName string, from, to *mozi.MessageIR) []FieldChange {
	var changes []FieldChange
	prefix := msgName + "."

	toReservedNum := make(map[int32]bool)
	for _, n := range to.ReservedNumbers {
		toReservedNum[n] = true
	}
	toReservedName := make(map[string]bool)
	for _, n := range to.ReservedNames {
		toReservedName[n] = true
	}

	fromByName := make(map[string]*mozi.MessageFieldIR)
	for i := range from.Fields {
		fromByName[from.Fields[i].Name] = &from.Fields[i]
	}
	toByName := make(map[string]*mozi.MessageFieldIR)
	for i := range to.Fields {
		toByName[to.Fields[i].Name] = &to.Fields[i]
	}

	// Fields present in the new revision: added, renamed, or modified.
	for i := range to.Fields {
		f := &to.Fields[i]
		if f.RenamedFrom != "" {
			if old, ok := fromByName[f.RenamedFrom]; ok {
				if old.Number != f.Number {
					changes = append(changes, FieldChange{
						Type: ChangeModified, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
						Compatibility: CompatibilityBreaking,
						Detail:        fmt.Sprintf("~ field %s%s (renamed from %s) changed number: %d → %d", prefix, f.Name, f.RenamedFrom, old.Number, f.Number),
						OldValue:      fmt.Sprintf("%d", old.Number), NewValue: fmt.Sprintf("%d", f.Number),
					})
				} else {
					changes = append(changes, FieldChange{
						Type: ChangeModified, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
						Compatibility: CompatibilitySafe,
						Detail:        fmt.Sprintf("~ field renamed: %s → %s (number %d unchanged)", f.RenamedFrom, f.Name, f.Number),
						OldValue:      f.RenamedFrom, NewValue: f.Name,
					})
				}
				changes = append(changes, classifyTypeChange(prefix+f.Name, old, f)...)
				continue
			}
		}
		old, ok := fromByName[f.Name]
		if !ok {
			changes = append(changes, FieldChange{
				Type: ChangeAdded, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
				Compatibility: CompatibilitySafe,
				Detail:        fmt.Sprintf("+ field added: %s%s (number %d)", prefix, f.Name, f.Number),
			})
			continue
		}
		if old.Number != f.Number {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("~ field %s%s changed number: %d → %d ⚠️ WIRE BREAKING", prefix, f.Name, old.Number, f.Number),
				OldValue:      fmt.Sprintf("%d", old.Number), NewValue: fmt.Sprintf("%d", f.Number),
			})
		}
		changes = append(changes, classifyTypeChange(prefix+f.Name, old, f)...)
		if old.Repeated != f.Repeated {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("~ field %s%s changed repeated: %v → %v", prefix, f.Name, old.Repeated, f.Repeated),
			})
		}
	}

	// Fields removed from the new revision, excluding names consumed by a rename.
	renamedAway := make(map[string]bool)
	for _, f := range to.Fields {
		if f.RenamedFrom != "" {
			renamedAway[f.RenamedFrom] = true
		}
	}
	for _, f := range from.Fields {
		if renamedAway[f.Name] {
			continue
		}
		if _, ok := toByName[f.Name]; ok {
			continue
		}
		compat := CompatibilityBreaking
		detail := fmt.Sprintf("- field removed: %s%s (number %d) ⚠️ number not reserved", prefix, f.Name, f.Number)
		if toReservedNum[f.Number] && toReservedName[f.Name] {
			compat = CompatibilityConditional
			detail = fmt.Sprintf("- field removed: %s%s (number %d reserved)", prefix, f.Name, f.Number)
		}
		changes = append(changes, FieldChange{
			Type: ChangeRemoved, Category: ServiceCategoryMessageField, Name: prefix + f.Name,
			Compatibility: compat,
			Detail:        detail,
		})
	}
	return changes
}

// classifyTypeChange rates a scalar/reference type change by wire compatibility.
func classifyTypeChange(name string, old, new *mozi.MessageFieldIR) []FieldChange {
	if old.Type == new.Type {
		return nil
	}
	compat := CompatibilityBreaking
	switch {
	case old.Type == string(mozi.FieldTypeInt) && new.Type == string(mozi.FieldTypeFloat),
		old.Type == string(mozi.FieldTypeString) && new.Type == string(mozi.FieldTypeText):
		compat = CompatibilityConditional
	}
	return []FieldChange{{
		Type: ChangeModified, Category: ServiceCategoryMessageField, Name: name,
		Compatibility: compat,
		Detail:        fmt.Sprintf("~ field %s changed type: %s → %s", name, old.Type, new.Type),
		OldValue:      old.Type, NewValue: new.Type,
	}}
}

func compareHTTPRoutes(from, to []mozi.HTTPRouteIR) []FieldChange {
	var changes []FieldChange
	fromByName := make(map[string]mozi.HTTPRouteIR)
	for _, r := range from {
		fromByName[r.Name] = r
	}
	toByName := make(map[string]mozi.HTTPRouteIR)
	for _, r := range to {
		toByName[r.Name] = r
	}

	for _, r := range from {
		if _, ok := toByName[r.Name]; !ok {
			changes = append(changes, FieldChange{
				Type: ChangeRemoved, Category: ServiceCategoryHTTP, Name: r.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("- route removed: %s %s", r.Method, r.Path),
			})
		}
	}
	for _, r := range to {
		old, ok := fromByName[r.Name]
		if !ok {
			changes = append(changes, FieldChange{
				Type: ChangeAdded, Category: ServiceCategoryHTTP, Name: r.Name,
				Compatibility: CompatibilitySafe,
				Detail:        fmt.Sprintf("+ route added: %s %s", r.Method, r.Path),
			})
			continue
		}
		if old.Method != r.Method || old.Path != r.Path {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryHTTP, Name: r.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("~ route endpoint changed: %s %s → %s %s", old.Method, old.Path, r.Method, r.Path),
				OldValue:      old.Method + " " + old.Path, NewValue: r.Method + " " + r.Path,
			})
		}
		if old.Auth != r.Auth || old.Idempotency != r.Idempotency {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryHTTP, Name: r.Name,
				Compatibility: CompatibilityConditional,
				Detail:        fmt.Sprintf("~ route policy changed (auth/idempotency): %s", r.Name),
			})
		}
		if old.Request != r.Request || old.Response != r.Response {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryHTTP, Name: r.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("~ route message changed: request %q→%q response %q→%q", old.Request, r.Request, old.Response, r.Response),
			})
		}
	}
	return changes
}

func compareRPCMethods(from, to []mozi.RPCMethodIR) []FieldChange {
	var changes []FieldChange
	fromByName := make(map[string]mozi.RPCMethodIR)
	for _, m := range from {
		fromByName[m.Name] = m
	}
	toByName := make(map[string]mozi.RPCMethodIR)
	for _, m := range to {
		toByName[m.Name] = m
	}

	for _, m := range from {
		if _, ok := toByName[m.Name]; !ok {
			changes = append(changes, FieldChange{
				Type: ChangeRemoved, Category: ServiceCategoryRPC, Name: m.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("- rpc method removed: %s", m.Name),
			})
		}
	}
	for _, m := range to {
		old, ok := fromByName[m.Name]
		if !ok {
			changes = append(changes, FieldChange{
				Type: ChangeAdded, Category: ServiceCategoryRPC, Name: m.Name,
				Compatibility: CompatibilitySafe,
				Detail:        fmt.Sprintf("+ rpc method added: %s", m.Name),
			})
			continue
		}
		if old.Request != m.Request || old.Response != m.Response {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryRPC, Name: m.Name,
				Compatibility: CompatibilityBreaking,
				Detail:        fmt.Sprintf("~ rpc messages changed: request %q→%q response %q→%q", old.Request, m.Request, old.Response, m.Response),
			})
		}
		if old.Idempotency != m.Idempotency {
			changes = append(changes, FieldChange{
				Type: ChangeModified, Category: ServiceCategoryRPC, Name: m.Name,
				Compatibility: CompatibilityConditional,
				Detail:        fmt.Sprintf("~ rpc idempotency changed: %s → %s", old.Idempotency, m.Idempotency),
			})
		}
	}
	return changes
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
