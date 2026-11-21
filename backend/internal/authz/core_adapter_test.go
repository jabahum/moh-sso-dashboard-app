package authz

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

func TestCoreAdapterPreservesSystemMetadataNilCollectionsAndCauses(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	for _, expected := range []ResolvedAccess{
		{},
		{Permissions: []Permission{}, Systems: []SystemAccess{}},
		{Permissions: []Permission{PermissionDocumentsRead, PermissionEmailRead}, Systems: []SystemAccess{{
			ClientID: "app", DisplayName: "Application", LaunchURL: "https://app.example.org",
			AuthenticatedLaunchURL: "https://app.example.org/login", Icon: "icon", Category: "category",
			Navigation: "[]", SystemType: "platform", DisplayInLauncher: true, DisplayInSideNav: true,
			LaunchMode: "internal", Roles: []string{"viewer"},
		}}},
	} {
		roles := []string{"user"}
		clients := map[string][]string{"app": {"viewer"}}
		host := resolverFunc(func(actual context.Context, actualRoles []string, actualClients map[string][]string) (ResolvedAccess, error) {
			if actual != ctx || !reflect.DeepEqual(actualRoles, roles) || !reflect.DeepEqual(actualClients, clients) {
				t.Fatal("adapter lost context/claims")
			}
			return expected, nil
		})
		result, err := core.ResolveAuthorization(ctx, NewCoreAuthorizationResolver(host), core.Identity{Subject: "subject", RealmRoles: roles, ClientRoles: clients})
		if err != nil || !reflect.DeepEqual(fromCoreAccess(result), expected) {
			t.Fatalf("contract roundtrip changed: %+v %v", result, err)
		}
	}
	cause := errors.New("authoritative resolver unavailable")
	host := resolverFunc(func(context.Context, []string, map[string][]string) (ResolvedAccess, error) {
		return ResolvedAccess{Permissions: []Permission{PermissionRBACWrite}}, cause
	})
	result, err := core.ResolveAuthorization(ctx, NewCoreAuthorizationResolver(host), core.Identity{})
	if !errors.Is(err, cause) || len(result.Permissions) != 0 {
		t.Fatal("error cause or partial-grant denial changed")
	}
	if NewCoreAuthorizationResolver(nil) != nil {
		t.Fatal("nil provider became a configured adapter")
	}
}
func TestIdentityAdapterNormalizesWithoutChangingHostPolicyOrSchema(t *testing.T) {
	identity := core.Identity{Subject: " user-id ", RealmRoles: []string{" USER ", "user"}, ClientRoles: map[string][]string{" app ": {" VIEWER ", "viewer"}}}
	called := false
	resolver := resolverFunc(func(_ context.Context, roles []string, clients map[string][]string) (ResolvedAccess, error) {
		called = true
		if !reflect.DeepEqual(roles, []string{"user"}) || !reflect.DeepEqual(clients, map[string][]string{"app": {"viewer"}}) {
			t.Fatalf("host normalization changed: %v %v", roles, clients)
		}
		return ResolvedAccess{Permissions: []Permission{PermissionDocumentsRead, "", PermissionDocumentsRead}, Systems: []SystemAccess{{ClientID: " app "}}}, nil
	})
	access := NewContextForIdentity(context.Background(), resolver, identity)
	if !called || access.UserID != "user-id" || !access.IsUser || access.IsAdmin || !access.HasPermission(PermissionDocumentsRead) || !access.HasSystem("APP") || len(access.Permissions) != 1 {
		t.Fatalf("host policy changed: %+v", access)
	}
	if identity.RealmRoles[0] != " USER " {
		t.Fatal("verified input claims mutated")
	}
	if reflect.TypeOf(access).PkgPath() != "github.com/moh-sso-dashboard/internal/authz" {
		t.Fatal("public API schema owner changed")
	}
}
