package rbac

import (
	"github.com/pangu-studio/mozi-builder/mozi"
	"testing"
)

func TestAllowedDenyFirstAndOwnScope(t *testing.T) {
	rules := []mozi.PermissionIR{{Effect: "allow", Principal: "user", Resource: "deck", Action: "read", Scope: "own"}, {Effect: "deny", Principal: "blocked", Resource: "deck", Action: "read", Scope: "all"}}
	if !Allowed(rules, Request{Principal: "user", Resource: "deck", Action: "read", ActorID: "u1", OwnerID: "u1"}) {
		t.Fatal("owner should be allowed")
	}
	if Allowed(rules, Request{Principal: "user", Resource: "deck", Action: "read", ActorID: "u1", OwnerID: "u2"}) {
		t.Fatal("non-owner should fail closed")
	}
}
