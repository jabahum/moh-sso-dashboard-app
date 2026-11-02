package authz

import (
	"context"
	"errors"
	"testing"
)

type resolverFunc func(context.Context, []string, map[string][]string) (ResolvedAccess, error)

func (f resolverFunc) Resolve(ctx context.Context, roles []string, clients map[string][]string) (ResolvedAccess, error) {
	return f(ctx, roles, clients)
}

func TestAuthoritativeResolverCannotRestoreDisabledApplicationViaFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		fallbackCalled := false
		primary := resolverFunc(func(context.Context, []string, map[string][]string) (ResolvedAccess, error) {
			if fail {
				return ResolvedAccess{}, errors.New("database unavailable")
			}
			return ResolvedAccess{}, nil
		})
		fallback := resolverFunc(func(context.Context, []string, map[string][]string) (ResolvedAccess, error) {
			fallbackCalled = true
			return ResolvedAccess{Systems: []SystemAccess{{ClientID: "disabled"}}, Permissions: []Permission{PermissionRBACWrite}}, nil
		})
		resolver := NewCompositeResolver(primary, fallback)
		access := ResolveAccess(context.Background(), resolver, []string{RoleAdmin}, map[string][]string{SystemDashboardWeb: {DashboardWebAccess}})
		if fallbackCalled || len(access.Systems) != 0 || len(access.Permissions) != 0 {
			t.Fatalf("fallback restored disabled application access: %+v", access)
		}
	}
}

func TestStaticResolverStillWorksWithoutDatabaseResolver(t *testing.T) {
	access := ResolveAccess(context.Background(), NewCompositeResolver(nil, NewStaticResolver()), []string{RoleAdmin}, nil)
	if len(access.Permissions) == 0 {
		t.Fatal("static mode lost its permissions")
	}
}
