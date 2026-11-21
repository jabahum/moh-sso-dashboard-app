package authz

import (
	"context"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

type Context struct {
	UserID                  string              `json:"userId"`
	RealmRoles              []string            `json:"realmRoles"`
	ClientRoles             map[string][]string `json:"clientRoles"`
	Permissions             []Permission        `json:"permissions"`
	AccessibleSystems       []string            `json:"systems"`
	AccessibleSystemDetails []SystemAccess      `json:"accessibleSystems"`
	IsAdmin                 bool                `json:"isAdmin"`
	IsUser                  bool                `json:"isUser"`
}

func NewContext(userID string, realmRoles []string, clientRoles map[string][]string) Context {
	return NewContextForIdentity(context.Background(), nil, core.Identity{Subject: userID, RealmRoles: realmRoles, ClientRoles: clientRoles})
}

func NewContextWithResolver(ctx context.Context, resolver PermissionResolver, userID string, realmRoles []string, clientRoles map[string][]string) Context {
	return NewContextForIdentity(ctx, resolver, core.Identity{Subject: userID, RealmRoles: realmRoles, ClientRoles: clientRoles})
}

// NewContextForIdentity adapts verified core claims to the existing host API.
// Role normalization, defaults, admin flags and resolver selection stay host policy.
func NewContextForIdentity(ctx context.Context, resolver PermissionResolver, identity core.Identity) Context {
	normalizedRealmRoles := NormalizeRoles(identity.RealmRoles)
	normalizedClientRoles := normalizeClientRoles(identity.ClientRoles)
	access := ResolveAccess(ctx, resolver, normalizedRealmRoles, normalizedClientRoles)
	return Context{
		UserID:     strings.TrimSpace(identity.Subject),
		RealmRoles: normalizedRealmRoles, ClientRoles: normalizedClientRoles,
		Permissions:       dedupePermissions(access.Permissions),
		AccessibleSystems: systemIDs(access.Systems), AccessibleSystemDetails: access.Systems,
		IsAdmin: hasRole(normalizedRealmRoles, RoleAdmin), IsUser: hasRole(normalizedRealmRoles, RoleUser),
	}
}

func (c Context) HasPermission(permission Permission) bool {
	return core.HasPermission(c.Permissions, permission)
}
func (c Context) HasAnyPermission(permissions ...Permission) bool {
	return core.HasAnyPermission(c.Permissions, permissions...)
}

func (c Context) HasRealmRole(role string) bool {
	return hasRole(c.RealmRoles, role)
}

func (c Context) HasClientRole(clientID string, role string) bool {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return false
	}

	return hasRole(c.ClientRoles[clientID], role)
}

func (c Context) HasSystem(system string) bool { return core.HasSystem(c.AccessibleSystems, system) }

func (c Context) HasSystemRole(system string, role string) bool {
	return c.HasClientRole(system, role)
}

func (c Context) PermissionStrings() []string {
	values := make([]string, 0, len(c.Permissions))
	for _, permission := range c.Permissions {
		values = append(values, string(permission))
	}

	return values
}

func (c Context) SystemStrings() []string {
	values := make([]string, 0, len(c.AccessibleSystems))
	values = append(values, c.AccessibleSystems...)
	return values
}

func (c Context) SystemAccess() []SystemAccess {
	values := make([]SystemAccess, 0, len(c.AccessibleSystemDetails))
	values = append(values, c.AccessibleSystemDetails...)
	return values
}

func normalizeClientRoles(clientRoles map[string][]string) map[string][]string {
	normalized := make(map[string][]string, len(clientRoles))
	for clientID, roles := range clientRoles {
		key := strings.TrimSpace(clientID)
		if key == "" {
			continue
		}
		normalized[key] = NormalizeRoles(roles)
	}

	return normalized
}

func dedupePermissions(permissions []Permission) []Permission {
	seen := map[Permission]bool{}
	values := make([]Permission, 0, len(permissions))
	for _, permission := range permissions {
		if permission == "" || seen[permission] {
			continue
		}
		seen[permission] = true
		values = append(values, permission)
	}
	return values
}

func systemIDs(systems []SystemAccess) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(systems))
	for _, system := range systems {
		clientID := strings.TrimSpace(system.ClientID)
		if clientID == "" || seen[clientID] {
			continue
		}
		seen[clientID] = true
		values = append(values, clientID)
	}
	return values
}
