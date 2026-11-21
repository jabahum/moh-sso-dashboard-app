package authz

import (
	"context"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type coreAuthorizationResolver struct{ host PermissionResolver }

// NewCoreAuthorizationResolver adapts the existing trusted DB/static resolver
// without changing its fallback policy or exposing host dependencies to core.
func NewCoreAuthorizationResolver(resolver PermissionResolver) core.AuthorizationResolver {
	if resolver == nil {
		return nil
	}
	return coreAuthorizationResolver{host: resolver}
}
func (a coreAuthorizationResolver) Resolve(ctx context.Context, identity core.Identity) (core.Access, error) {
	access, err := a.host.Resolve(ctx, identity.RealmRoles, identity.ClientRoles)
	if err != nil {
		return core.Access{}, err
	}
	var out core.Access
	if access.Permissions != nil {
		out.Permissions = make([]string, len(access.Permissions))
		for i, permission := range access.Permissions {
			out.Permissions[i] = string(permission)
		}
	}
	if access.Systems != nil {
		out.Systems = make([]core.SystemAccess, len(access.Systems))
		for i, system := range access.Systems {
			out.Systems[i] = core.SystemAccess(system)
		}
	}
	return out, nil
}
func fromCoreAccess(access core.Access) ResolvedAccess {
	var out ResolvedAccess
	if access.Permissions != nil {
		out.Permissions = make([]Permission, len(access.Permissions))
		for i, permission := range access.Permissions {
			out.Permissions[i] = Permission(permission)
		}
	}
	if access.Systems != nil {
		out.Systems = make([]SystemAccess, len(access.Systems))
		for i, system := range access.Systems {
			out.Systems[i] = SystemAccess(system)
		}
	}
	return out
}
