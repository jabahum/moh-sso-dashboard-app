package core_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type authorizationFunc func(context.Context, core.Identity) (core.Access, error)

func (f authorizationFunc) Resolve(ctx context.Context, id core.Identity) (core.Access, error) {
	return f(ctx, id)
}

func TestAuthorizationPreservesVerifiedClaimsAndExplicitDecisions(t *testing.T) {
	type requestKey struct{}
	ctx := context.WithValue(context.Background(), requestKey{}, "request")
	identity := core.Identity{Subject: "subject", RealmRoles: []string{"admin"}, ClientRoles: map[string][]string{"app": {"viewer"}}}
	expected := core.Access{Permissions: []string{"documents:read"}, Systems: []core.SystemAccess{{ClientID: "app", Navigation: "[]", Roles: []string{"viewer"}}}}
	called := false
	resolver := authorizationFunc(func(actual context.Context, id core.Identity) (core.Access, error) {
		called = true
		if actual != ctx || !reflect.DeepEqual(id, identity) {
			t.Fatal("request context or verified claims changed")
		}
		return expected, nil
	})
	access, err := core.ResolveAuthorization(ctx, resolver, identity)
	if err != nil || !called || !reflect.DeepEqual(access, expected) {
		t.Fatalf("decision changed: %+v %v", access, err)
	}
	if core.HasPermission(access.Permissions, "documents:write") {
		t.Fatal("admin claim inferred an implicit grant")
	}
}
func TestAuthorizationFailuresDiscardPartialGrants(t *testing.T) {
	cause := errors.New("provider unavailable")
	partial := authorizationFunc(func(context.Context, core.Identity) (core.Access, error) {
		return core.Access{Permissions: []string{"admin:write"}, Systems: []core.SystemAccess{{ClientID: "app"}}}, cause
	})
	access, err := core.ResolveAuthorization(context.Background(), partial, core.Identity{RealmRoles: []string{"admin"}})
	if !errors.Is(err, cause) || len(access.Permissions) != 0 || len(access.Systems) != 0 {
		t.Fatalf("partial grants leaked: %+v %v", access, err)
	}
	access, err = core.ResolveAuthorization(context.Background(), nil, core.Identity{})
	if !errors.Is(err, core.ErrAuthorizationUnavailable) || len(access.Permissions) != 0 {
		t.Fatal("missing provider became a grant")
	}
}
func TestAccessPredicatesPreserveExactPermissionAndRoleSystemSemantics(t *testing.T) {
	type permission string
	grants := []permission{"documents:read"}
	if !core.HasPermission(grants, permission("documents:read")) || core.HasPermission(grants, permission("Documents:read")) || core.HasPermission(grants, permission(" documents:read ")) {
		t.Fatal("permission matching widened")
	}
	if core.HasAnyPermission(grants) || !core.HasAnyPermission(grants, permission("missing"), permission("documents:read")) {
		t.Fatal("OR permission semantics changed")
	}
	if !core.HasRole([]string{" ADMIN "}, " admin ") || core.HasRole([]string{"admin"}, "") {
		t.Fatal("role matching changed")
	}
	if !core.HasSystem([]string{"Data-Statistics"}, " data-statistics ") || core.HasSystem([]string{"data-statistics"}, "") {
		t.Fatal("system matching changed")
	}
	if core.HasSystem([]string{" data-statistics "}, "data-statistics") {
		t.Fatal("system values were implicitly normalized")
	}
}
