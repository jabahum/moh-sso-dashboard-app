package rbac

import "context"

type Repository interface {
	ListSystems(ctx context.Context) ([]System, error)
	TransitionSystem(ctx context.Context, clientID string, input LifecycleTransitionInput, actor string, validate func(System, int) error) (System, error)
	GetSystem(ctx context.Context, clientID string) (SystemDetail, error)
	UpsertSystem(ctx context.Context, input UpsertSystemInput) (System, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	UpdatePermissionMetadata(ctx context.Context, permissionKey string, input PermissionMetadataInput) (Permission, error)

	ListSystemRoles(ctx context.Context, clientID string) ([]SystemRole, error)
	CreateSystemRole(ctx context.Context, clientID string, input RoleInput) (SystemRole, error)
	UpsertSystemRole(ctx context.Context, clientID string, input RoleInput) (SystemRole, error)
	UpdateSystemRole(ctx context.Context, roleID string, input RoleInput) (SystemRole, error)
	DeleteSystemRole(ctx context.Context, roleID string) error

	AssignSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error
	RemoveSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error

	ListRealmRolePermissions(ctx context.Context) ([]RealmRolePermissionGroup, error)
	AssignRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error
	RemoveRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error
	ListRealmRoleSystemRoles(ctx context.Context) ([]RealmRoleSystemRole, error)

	ListGroups(ctx context.Context) ([]Group, error)
	GetGroup(ctx context.Context, groupID string) (Group, error)
	ListGroupsForUser(ctx context.Context, userID string, username string, email string) ([]Group, error)
	UpsertGroup(ctx context.Context, input GroupInput) (Group, error)
	ListGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error)
	ReplaceGroupMembers(ctx context.Context, groupID string, members []GroupMember) error
	AssignGroupPermission(ctx context.Context, groupID string, permissionKey string) error
	RemoveGroupPermission(ctx context.Context, groupID string, permissionKey string) error
	AssignGroupRealmRole(ctx context.Context, groupID string, realmRole string) error
	RemoveGroupRealmRole(ctx context.Context, groupID string, realmRole string) error
	AssignGroupSystemRole(ctx context.Context, groupID string, clientID string, roleName string) error
	RemoveGroupSystemRole(ctx context.Context, groupID string, clientID string, roleName string) error

	AddSystemAccessRole(ctx context.Context, clientID string, roleName string) error
	RemoveSystemAccessRole(ctx context.Context, clientID string, roleName string) error
	CountSystemAccessRoles(ctx context.Context, clientID string) (int, error)
	PermissionExists(ctx context.Context, permissionKey string) (bool, error)
	GetSystemRole(ctx context.Context, roleID string) (SystemRole, error)
	ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
	RecordAuditEvent(ctx context.Context, event AuditEvent) error
	CreateAccessRequest(ctx context.Context, input AccessRequestInput) (AccessRequest, error)
	ListAccessRequests(ctx context.Context) ([]AccessRequest, error)
	UpdateAccessRequestStatus(ctx context.Context, id string, status string, note string, reviewer string) (AccessRequest, error)
	CreateChangeRequest(ctx context.Context, input ChangeRequestInput) (ChangeRequest, error)
	ListChangeRequests(ctx context.Context) ([]ChangeRequest, error)
	UpdateChangeRequestStatus(ctx context.Context, id string, status string, note string, reviewer string) (ChangeRequest, error)
}
