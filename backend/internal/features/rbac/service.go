package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
	"go.yaml.in/yaml/v3"
)

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrPermissionMissing = errors.New("permission does not exist")
	ErrLastAccessRole    = errors.New("cannot remove last access role")
)

type Service struct {
	repository      Repository
	users           UserLookup
	keycloakGroups  KeycloakGroupMembershipManager
	frontendBaseURL string
}

type UserLookup interface {
	GetUser(id uuid.UUID) (*model.User, error)
	ListUsers() ([]model.User, error)
	UpdateUserRealmRoles(ctx context.Context, userID uuid.UUID, roles []string, adminID uuid.UUID) error
	UpdateUserClientRoles(ctx context.Context, userID uuid.UUID, clientID string, clientUUID string, roles []string, adminID uuid.UUID) error
}

type KeycloakGroupMembershipManager interface {
	GetUser(userID string) (*keycloak.UserInfo, error)
	ListGroups(ctx context.Context) ([]keycloak.GroupRep, error)
	EnsureGroupPath(ctx context.Context, groupPath string, displayName string, description string, attributes map[string][]string) (keycloak.GroupRep, error)
	ListGroupMembers(ctx context.Context, groupID string) ([]keycloak.KeycloakUser, error)
	AddUserToGroup(ctx context.Context, userID string, groupID string) error
	RemoveUserFromGroup(ctx context.Context, userID string, groupID string) error
}

func NewService(repository Repository, users ...UserLookup) *Service {
	var userLookup UserLookup
	if len(users) > 0 {
		userLookup = users[0]
	}
	return &Service{repository: repository, users: userLookup}
}

func (s *Service) SetKeycloakGroupMembershipManager(manager KeycloakGroupMembershipManager) {
	s.keycloakGroups = manager
}

func (s *Service) SetFrontendBaseURL(value string) {
	s.frontendBaseURL = strings.TrimSpace(value)
}

func (s *Service) ListSystems(ctx context.Context) ([]System, error) {
	return s.repository.ListSystems(ctx)
}

func (s *Service) GetSystem(ctx context.Context, clientID string) (SystemDetail, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return SystemDetail{}, fmt.Errorf("%w: clientId is required", ErrInvalidInput)
	}
	return s.repository.GetSystem(ctx, clientID)
}

func (s *Service) UpsertSystem(ctx context.Context, input UpsertSystemInput) (System, error) {
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	input.Icon = strings.TrimSpace(input.Icon)
	input.LaunchURL = strings.TrimSpace(input.LaunchURL)
	input.AuthenticatedLaunchURL = strings.TrimSpace(input.AuthenticatedLaunchURL)
	input.Category = strings.TrimSpace(input.Category)
	input.OwnerTeam = strings.TrimSpace(input.OwnerTeam)
	input.OwnerName = strings.TrimSpace(input.OwnerName)
	input.OwnerEmail = strings.TrimSpace(input.OwnerEmail)
	input.SupportURL = strings.TrimSpace(input.SupportURL)
	input.DocumentationURL = strings.TrimSpace(input.DocumentationURL)
	input.Environment = strings.TrimSpace(input.Environment)
	input.Criticality = strings.TrimSpace(input.Criticality)
	input.Navigation = strings.TrimSpace(input.Navigation)
	input.IntegrationType = strings.TrimSpace(input.IntegrationType)
	input.ContractVersion = strings.TrimSpace(input.ContractVersion)
	// Omitted new fields must preserve existing integration contracts.
	if input.IntegrationType == "" || input.ContractVersion == "" {
		current, err := s.repository.GetSystem(ctx, input.ClientID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return System{}, err
		}
		if input.IntegrationType == "" {
			input.IntegrationType = current.IntegrationType
		}
		if input.ContractVersion == "" {
			input.ContractVersion = current.ContractVersion
		}
	}

	if input.ContractVersion == "" {
		input.ContractVersion = "1"
	}
	if input.ContractVersion != "1" {
		return System{}, fmt.Errorf("%w: unsupported contractVersion", ErrInvalidInput)
	}
	behavior := systemrbac.NormalizeSystemBehavior(systemrbac.SeedSystem{
		LaunchURL:              input.LaunchURL,
		AuthenticatedLaunchURL: input.AuthenticatedLaunchURL,
		Navigation:             input.Navigation,
		SystemType:             input.SystemType,
		DisplayInLauncher:      input.DisplayInLauncher,
		DisplayInSideNav:       input.DisplayInSideNav,
		LaunchMode:             input.LaunchMode,
	})
	if err := systemrbac.ValidateSystemBehavior(behavior); err != nil {
		return System{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	input.SystemType = behavior.SystemType
	input.DisplayInLauncher = behavior.DisplayInLauncher
	input.DisplayInSideNav = behavior.DisplayInSideNav
	input.LaunchMode = behavior.LaunchMode
	if input.IntegrationType == "" {
		input.IntegrationType = "external"
		if behavior.SystemType == "platform" {
			input.IntegrationType = "hosted_module"
		}
	}
	if input.IntegrationType != "external" && input.IntegrationType != "hosted_module" {
		return System{}, fmt.Errorf("%w: unknown integrationType", ErrInvalidInput)
	}
	if input.ClientID == "" {
		return System{}, fmt.Errorf("%w: clientId is required", ErrInvalidInput)
	}
	if input.DisplayName == "" {
		return System{}, fmt.Errorf("%w: displayName is required", ErrInvalidInput)
	}
	if err := validateOptionalURL("supportUrl", input.SupportURL); err != nil {
		return System{}, err
	}
	if err := validateOptionalURL("documentationUrl", input.DocumentationURL); err != nil {
		return System{}, err
	}
	system, err := s.repository.UpsertSystem(ctx, input)
	if err != nil {
		return System{}, err
	}
	return system, nil
}

func (s *Service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repository.ListPermissions(ctx)
}

func (s *Service) ListAssignableUserAccess(ctx context.Context) (AssignableUserAccessResponse, error) {
	permissions, err := s.repository.ListPermissions(ctx)
	if err != nil {
		return AssignableUserAccessResponse{}, err
	}

	realmGroups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return AssignableUserAccessResponse{}, err
	}

	systems, err := s.repository.ListSystems(ctx)
	if err != nil {
		return AssignableUserAccessResponse{}, err
	}

	realmRoles := make([]AssignableRealmRole, 0, len(realmGroups))
	for _, group := range realmGroups {
		roleName := normalize(group.RealmRole)
		if roleName == "" {
			continue
		}
		realmRoles = append(realmRoles, AssignableRealmRole{
			Name:        roleName,
			DisplayName: roleName,
			Permissions: group.Permissions,
		})
	}

	assignableSystems := make([]AssignableSystemAccess, 0, len(systems))
	for _, system := range systems {
		if !system.Enabled || strings.TrimSpace(system.ClientID) == "" {
			continue
		}

		detail, err := s.repository.GetSystem(ctx, system.ClientID)
		if err != nil {
			continue
		}

		roles := make([]AssignableSystemRole, 0, len(detail.Roles))
		for _, role := range detail.Roles {
			if !role.Enabled {
				continue
			}
			roles = append(roles, AssignableSystemRole{
				Name:        normalize(role.Name),
				DisplayName: role.DisplayName,
				Description: role.Description,
				Enabled:     role.Enabled,
				Permissions: role.Permissions,
			})
		}

		sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })

		assignableSystems = append(assignableSystems, AssignableSystemAccess{
			ClientID:          detail.ClientID,
			DisplayName:       detail.DisplayName,
			LaunchURL:         detail.LaunchURL,
			Icon:              detail.Icon,
			Category:          detail.Category,
			SystemType:        detail.SystemType,
			DisplayInLauncher: detail.DisplayInLauncher,
			DisplayInSideNav:  detail.DisplayInSideNav,
			LaunchMode:        detail.LaunchMode,
			Roles:             roles,
			AccessRoles:       sortedStrings(detail.AccessRoles),
		})
	}

	sort.Slice(realmRoles, func(i, j int) bool { return realmRoles[i].Name < realmRoles[j].Name })
	sort.Slice(assignableSystems, func(i, j int) bool { return assignableSystems[i].DisplayName < assignableSystems[j].DisplayName })

	return AssignableUserAccessResponse{
		RealmRoles:  realmRoles,
		Systems:     assignableSystems,
		Permissions: permissions,
	}, nil
}

func (s *Service) GetUserAccessProfile(ctx context.Context, userID string) (UserAccessProfileResponse, error) {
	effective, err := s.GetEffectiveAccess(ctx, userID, "", "")
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	user, err := s.lookupUser(userID, "", "")
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	assignable, err := s.ListAssignableUserAccess(ctx)
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	return UserAccessProfileResponse{
		EffectiveAccess: effective,
		DirectAccess:    directUserAccess(user),
		Assignable:      assignable,
	}, nil
}

func (s *Service) UpdateUserAccess(
	ctx context.Context,
	userID string,
	input UpdateUserAccessRequest,
	actorID uuid.UUID,
) (UserAccessProfileResponse, error) {
	if s == nil || s.users == nil {
		return UserAccessProfileResponse{}, errors.New("user access manager is not configured")
	}

	userUUID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return UserAccessProfileResponse{}, fmt.Errorf("%w: invalid user id", ErrInvalidInput)
	}

	assignable, err := s.ListAssignableUserAccess(ctx)
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	realmRoles := sortedStrings(input.RealmRoles)
	clientRoles := sortedClientRoles(input.ClientRoles)

	if err := validateAssignableUserAccess(assignable, realmRoles, clientRoles, input.Permissions); err != nil {
		return UserAccessProfileResponse{}, err
	}

	before, err := s.GetEffectiveAccess(ctx, userID, "", "")
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	if err := s.users.UpdateUserRealmRoles(ctx, userUUID, realmRoles, actorID); err != nil {
		return UserAccessProfileResponse{}, err
	}

	for _, system := range assignable.Systems {
		roles := clientRoles[system.ClientID]
		if err := s.users.UpdateUserClientRoles(ctx, userUUID, system.ClientID, "", roles, actorID); err != nil {
			return UserAccessProfileResponse{}, err
		}
	}

	after, err := s.GetEffectiveAccess(ctx, userID, "", "")
	if err != nil {
		return UserAccessProfileResponse{}, err
	}

	addedRealm := permissionDiff(after.RealmRoles, before.RealmRoles)
	removedRealm := permissionDiff(before.RealmRoles, after.RealmRoles)
	addedClient, removedClient := diffClientRoles(before.ClientRoles, after.ClientRoles)
	addedPermissions := permissionDiff(permissionKeys(after.Permissions), permissionKeys(before.Permissions))
	removedPermissions := permissionDiff(permissionKeys(before.Permissions), permissionKeys(after.Permissions))

	if err := s.recordAudit(ctx, "user_access.updated", "user", userID, "", "", "", map[string]any{
		"actor_id":             actorID.String(),
		"realm_roles_added":    addedRealm,
		"realm_roles_removed":  removedRealm,
		"client_roles_added":   addedClient,
		"client_roles_removed": removedClient,
		"permissions_added":    addedPermissions,
		"permissions_removed":  removedPermissions,
	}); err != nil {
		return UserAccessProfileResponse{}, err
	}

	return UserAccessProfileResponse{
		EffectiveAccess: after,
		DirectAccess: DirectUserAccessResponse{
			RealmRoles:  realmRoles,
			ClientRoles: clientRoles,
			Permissions: []Permission{},
		},
		Assignable: assignable,
	}, nil
}

func (s *Service) ListSystemRoles(ctx context.Context, clientID string) ([]SystemRole, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fmt.Errorf("%w: clientId is required", ErrInvalidInput)
	}
	return s.repository.ListSystemRoles(ctx, clientID)
}

func (s *Service) CreateSystemRole(ctx context.Context, clientID string, input RoleInput) (SystemRole, error) {
	clientID = strings.TrimSpace(clientID)
	input = normalizeRoleInput(input)
	if clientID == "" || input.Name == "" {
		return SystemRole{}, fmt.Errorf("%w: clientId and role name are required", ErrInvalidInput)
	}
	role, err := s.repository.CreateSystemRole(ctx, clientID, input)
	if err != nil {
		return SystemRole{}, err
	}
	if err := s.recordAudit(ctx, "system_role.created", "system-role", role.ID, clientID, role.Name, "", nil); err != nil {
		return SystemRole{}, err
	}
	return role, nil
}

func (s *Service) UpdateSystemRole(ctx context.Context, roleID string, input RoleInput) (SystemRole, error) {
	roleID = strings.TrimSpace(roleID)
	input = normalizeRoleInput(input)
	if roleID == "" || input.Name == "" {
		return SystemRole{}, fmt.Errorf("%w: roleId and role name are required", ErrInvalidInput)
	}
	role, err := s.repository.UpdateSystemRole(ctx, roleID, input)
	if err != nil {
		return SystemRole{}, err
	}
	if err := s.recordAudit(ctx, "system_role.updated", "system-role", role.ID, role.ClientID, role.Name, "", nil); err != nil {
		return SystemRole{}, err
	}
	return role, nil
}

func (s *Service) DeleteSystemRole(ctx context.Context, roleID string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return fmt.Errorf("%w: roleId is required", ErrInvalidInput)
	}
	role, _ := s.repository.GetSystemRole(ctx, roleID)
	if err := s.repository.DeleteSystemRole(ctx, roleID); err != nil {
		return err
	}
	return s.recordAudit(ctx, "system_role.deleted", "system-role", roleID, role.ClientID, role.Name, "", nil)
}

func (s *Service) AssignSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error {
	roleID = strings.TrimSpace(roleID)
	permissionKey = strings.TrimSpace(permissionKey)
	if roleID == "" || permissionKey == "" {
		return fmt.Errorf("%w: roleId and permissionKey are required", ErrInvalidInput)
	}
	if err := s.requirePermission(ctx, permissionKey); err != nil {
		return err
	}
	if err := s.repository.AssignSystemRolePermission(ctx, roleID, permissionKey); err != nil {
		return err
	}
	role, _ := s.repository.GetSystemRole(ctx, roleID)
	return s.recordAudit(ctx, "system_role.permission_assigned", "system-role", roleID, role.ClientID, role.Name, permissionKey, nil)
}

func (s *Service) RemoveSystemRolePermission(ctx context.Context, roleID string, permissionKey string) error {
	roleID = strings.TrimSpace(roleID)
	permissionKey = strings.TrimSpace(permissionKey)
	if roleID == "" || permissionKey == "" {
		return fmt.Errorf("%w: roleId and permissionKey are required", ErrInvalidInput)
	}
	role, _ := s.repository.GetSystemRole(ctx, roleID)
	if err := s.repository.RemoveSystemRolePermission(ctx, roleID, permissionKey); err != nil {
		return err
	}
	return s.recordAudit(ctx, "system_role.permission_removed", "system-role", roleID, role.ClientID, role.Name, permissionKey, nil)
}

func (s *Service) ListRealmRolePermissions(ctx context.Context) ([]RealmRolePermissionGroup, error) {
	return s.repository.ListRealmRolePermissions(ctx)
}

func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	return s.repository.ListGroups(ctx)
}

func (s *Service) GetGroup(ctx context.Context, groupID string) (Group, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return Group{}, fmt.Errorf("%w: groupId is required", ErrInvalidInput)
	}
	return s.repository.GetGroup(ctx, groupID)
}

func (s *Service) UpsertGroup(ctx context.Context, input GroupInput) (Group, error) {
	input.KeycloakGroupID = strings.TrimSpace(input.KeycloakGroupID)
	input.Path = strings.TrimSpace(input.Path)
	input.Name = strings.TrimSpace(input.Name)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	if input.Path == "" {
		return Group{}, fmt.Errorf("%w: group path is required", ErrInvalidInput)
	}
	if !strings.HasPrefix(input.Path, "/") {
		input.Path = "/" + input.Path
	}
	if input.Name == "" {
		parts := strings.Split(strings.Trim(input.Path, "/"), "/")
		input.Name = parts[len(parts)-1]
	}
	group, err := s.repository.UpsertGroup(ctx, input)
	if err != nil {
		return Group{}, err
	}
	if err := s.recordAudit(ctx, "group.upserted", "group", group.ID, "", "", "", groupAuditDetails(group, nil)); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (s *Service) ListGroupMembers(ctx context.Context, groupID string) ([]GroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("%w: groupId is required", ErrInvalidInput)
	}
	return s.repository.ListGroupMembers(ctx, groupID)
}

func (s *Service) ReplaceGroupMembers(ctx context.Context, groupID string, members []GroupMember) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("%w: groupId is required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	normalized := make([]GroupMember, 0, len(members))
	for _, member := range members {
		member.UserID = strings.TrimSpace(member.UserID)
		member.Username = strings.TrimSpace(member.Username)
		member.Email = strings.TrimSpace(member.Email)
		if member.UserID == "" {
			continue
		}
		normalized = append(normalized, member)
	}
	if err := s.repository.ReplaceGroupMembers(ctx, groupID, normalized); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.members_replaced", "group", groupID, "", "", "", groupAuditDetails(group, map[string]any{"memberCount": len(normalized)}))
}

func (s *Service) AddGroupMember(
	ctx context.Context,
	groupID string,
	userID string,
	actorID uuid.UUID,
) (GroupMembersSyncResponse, error) {
	group, keycloakGroupID, err := s.groupMembershipTarget(ctx, groupID, userID)
	if err != nil {
		return GroupMembersSyncResponse{}, err
	}
	user, err := s.keycloakGroups.GetUser(strings.TrimSpace(userID))
	if err != nil {
		return GroupMembersSyncResponse{}, fmt.Errorf("%w: user does not exist in Keycloak", ErrInvalidInput)
	}

	if err := s.keycloakGroups.AddUserToGroup(ctx, user.ID, keycloakGroupID); err != nil {
		return GroupMembersSyncResponse{}, err
	}

	response, err := s.SyncGroupMembers(ctx, group.ID)
	if err != nil {
		response.Warnings = append(response.Warnings, fmt.Sprintf("Keycloak membership updated, but local cache refresh failed: %v", err))
	}
	_ = s.recordAudit(ctx, "group.member_added", "group", group.ID, "", "", "", groupAuditDetails(group, map[string]any{
		"actor_id": actorID.String(),
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
		"source":   "portal-admin",
	}))
	return response, nil
}

func (s *Service) RemoveGroupMember(
	ctx context.Context,
	groupID string,
	userID string,
	actorID uuid.UUID,
) (GroupMembersSyncResponse, error) {
	group, keycloakGroupID, err := s.groupMembershipTarget(ctx, groupID, userID)
	if err != nil {
		return GroupMembersSyncResponse{}, err
	}
	user, err := s.keycloakGroups.GetUser(strings.TrimSpace(userID))
	if err != nil {
		return GroupMembersSyncResponse{}, fmt.Errorf("%w: user does not exist in Keycloak", ErrInvalidInput)
	}

	if err := s.keycloakGroups.RemoveUserFromGroup(ctx, user.ID, keycloakGroupID); err != nil {
		return GroupMembersSyncResponse{}, err
	}

	response, err := s.SyncGroupMembers(ctx, group.ID)
	if err != nil {
		response.Warnings = append(response.Warnings, fmt.Sprintf("Keycloak membership updated, but local cache refresh failed: %v", err))
	}
	_ = s.recordAudit(ctx, "group.member_removed", "group", group.ID, "", "", "", groupAuditDetails(group, map[string]any{
		"actor_id": actorID.String(),
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
		"source":   "portal-admin",
	}))
	return response, nil
}

func (s *Service) SyncGroupMembers(ctx context.Context, groupID string) (GroupMembersSyncResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return GroupMembersSyncResponse{}, fmt.Errorf("%w: groupId is required", ErrInvalidInput)
	}
	if s.keycloakGroups == nil {
		return GroupMembersSyncResponse{}, fmt.Errorf("%w: Keycloak group membership manager is not configured", ErrInvalidInput)
	}

	group, err := s.repository.GetGroup(ctx, groupID)
	if err != nil {
		return GroupMembersSyncResponse{}, err
	}
	keycloakGroupID := strings.TrimSpace(group.KeycloakGroupID)
	if keycloakGroupID == "" {
		resolvedGroupID, err := s.resolveKeycloakGroupID(ctx, group)
		if err != nil {
			return GroupMembersSyncResponse{}, err
		}
		keycloakGroupID = resolvedGroupID
	}

	keycloakMembers, err := s.keycloakGroups.ListGroupMembers(ctx, keycloakGroupID)
	if err != nil {
		return GroupMembersSyncResponse{}, err
	}

	members := make([]GroupMember, 0, len(keycloakMembers))
	for _, member := range keycloakMembers {
		memberID := strings.TrimSpace(member.ID)
		if memberID == "" {
			continue
		}
		members = append(members, GroupMember{
			UserID:   memberID,
			Username: strings.TrimSpace(member.Username),
			Email:    strings.TrimSpace(member.Email),
		})
	}
	if err := s.repository.ReplaceGroupMembers(ctx, group.ID, members); err != nil {
		return GroupMembersSyncResponse{}, err
	}
	if err := s.recordAudit(ctx, "group.members_synced", "group", group.ID, "", "", "", groupAuditDetails(group, map[string]any{
		"member_count": len(members),
		"source":       "keycloak",
	})); err != nil {
		return GroupMembersSyncResponse{}, err
	}
	return GroupMembersSyncResponse{
		Members:     members,
		MemberCount: len(members),
	}, nil
}

func (s *Service) groupMembershipTarget(ctx context.Context, groupID string, userID string) (Group, string, error) {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return Group{}, "", fmt.Errorf("%w: groupId and userId are required", ErrInvalidInput)
	}
	if s.keycloakGroups == nil {
		return Group{}, "", fmt.Errorf("%w: Keycloak group membership manager is not configured", ErrInvalidInput)
	}
	group, err := s.repository.GetGroup(ctx, groupID)
	if err != nil {
		return Group{}, "", err
	}
	keycloakGroupID := strings.TrimSpace(group.KeycloakGroupID)
	if keycloakGroupID == "" {
		resolvedGroupID, err := s.resolveKeycloakGroupID(ctx, group)
		if err != nil {
			return Group{}, "", err
		}
		keycloakGroupID = resolvedGroupID
	}
	return group, keycloakGroupID, nil
}

func (s *Service) resolveKeycloakGroupID(ctx context.Context, group Group) (string, error) {
	localPath := normalizeGroupPath(group.Path)
	localName := strings.TrimSpace(group.Name)
	if localName == "" {
		localName = strings.Trim(strings.TrimSpace(group.Path), "/")
		if strings.Contains(localName, "/") {
			parts := strings.Split(localName, "/")
			localName = parts[len(parts)-1]
		}
	}

	keycloakGroups, err := s.keycloakGroups.ListGroups(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: group is not linked to a Keycloak group and Keycloak lookup failed", ErrInvalidInput)
	}
	for _, candidate := range keycloakGroups {
		candidateID := strings.TrimSpace(candidate.ID)
		if candidateID == "" {
			continue
		}
		if localPath != "" && normalizeGroupPath(candidate.Path) == localPath {
			updated, err := s.repository.UpsertGroup(ctx, GroupInput{
				KeycloakGroupID: candidateID,
				Path:            group.Path,
				Name:            group.Name,
				DisplayName:     group.DisplayName,
				Description:     group.Description,
				Enabled:         &group.Enabled,
			})
			if err == nil {
				return strings.TrimSpace(updated.KeycloakGroupID), nil
			}
			return candidateID, nil
		}
		if localPath == "" && localName != "" && strings.EqualFold(strings.TrimSpace(candidate.Name), localName) {
			updated, err := s.repository.UpsertGroup(ctx, GroupInput{
				KeycloakGroupID: candidateID,
				Path:            group.Path,
				Name:            group.Name,
				DisplayName:     group.DisplayName,
				Description:     group.Description,
				Enabled:         &group.Enabled,
			})
			if err == nil {
				return strings.TrimSpace(updated.KeycloakGroupID), nil
			}
			return candidateID, nil
		}
	}

	if localPath == "" {
		return "", fmt.Errorf("%w: group is not linked to a Keycloak group and has no usable path", ErrInvalidInput)
	}

	created, err := s.keycloakGroups.EnsureGroupPath(ctx, localPath, firstNonEmpty(group.DisplayName, group.Name), group.Description, nil)
	if err != nil {
		return "", fmt.Errorf("%w: group is not linked to a Keycloak group and automatic Keycloak group creation failed: %v", ErrInvalidInput, err)
	}
	createdID := strings.TrimSpace(created.ID)
	if createdID == "" {
		return "", fmt.Errorf("%w: group is not linked to a Keycloak group and automatic Keycloak group creation returned no id", ErrInvalidInput)
	}
	updated, err := s.repository.UpsertGroup(ctx, GroupInput{
		KeycloakGroupID: createdID,
		Path:            group.Path,
		Name:            group.Name,
		DisplayName:     group.DisplayName,
		Description:     group.Description,
		Enabled:         &group.Enabled,
	})
	if err == nil {
		return strings.TrimSpace(updated.KeycloakGroupID), nil
	}
	return createdID, nil
}

func (s *Service) AssignGroupPermission(ctx context.Context, groupID string, permissionKey string) error {
	groupID = strings.TrimSpace(groupID)
	permissionKey = strings.TrimSpace(permissionKey)
	if groupID == "" || permissionKey == "" {
		return fmt.Errorf("%w: groupId and permissionKey are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.requirePermission(ctx, permissionKey); err != nil {
		return err
	}
	if err := s.repository.AssignGroupPermission(ctx, groupID, permissionKey); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.permission_assigned", "group", groupID, "", "", permissionKey, groupAuditDetails(group, nil))
}

func (s *Service) RemoveGroupPermission(ctx context.Context, groupID string, permissionKey string) error {
	groupID = strings.TrimSpace(groupID)
	permissionKey = strings.TrimSpace(permissionKey)
	if groupID == "" || permissionKey == "" {
		return fmt.Errorf("%w: groupId and permissionKey are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.repository.RemoveGroupPermission(ctx, groupID, permissionKey); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.permission_removed", "group", groupID, "", "", permissionKey, groupAuditDetails(group, nil))
}

func (s *Service) AssignGroupRealmRole(ctx context.Context, groupID string, realmRole string) error {
	groupID = strings.TrimSpace(groupID)
	realmRole = normalize(realmRole)
	if groupID == "" || realmRole == "" {
		return fmt.Errorf("%w: groupId and realmRole are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.repository.AssignGroupRealmRole(ctx, groupID, realmRole); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.realm_role_assigned", "group", groupID, "", realmRole, "", groupAuditDetails(group, nil))
}

func (s *Service) RemoveGroupRealmRole(ctx context.Context, groupID string, realmRole string) error {
	groupID = strings.TrimSpace(groupID)
	realmRole = normalize(realmRole)
	if groupID == "" || realmRole == "" {
		return fmt.Errorf("%w: groupId and realmRole are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.repository.RemoveGroupRealmRole(ctx, groupID, realmRole); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.realm_role_removed", "group", groupID, "", realmRole, "", groupAuditDetails(group, nil))
}

func (s *Service) AssignGroupSystemRole(ctx context.Context, groupID string, input GroupSystemRoleInput) error {
	groupID = strings.TrimSpace(groupID)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.RoleName = normalize(input.RoleName)
	if groupID == "" || input.ClientID == "" || input.RoleName == "" {
		return fmt.Errorf("%w: groupId, clientId, and roleName are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.repository.AssignGroupSystemRole(ctx, groupID, input.ClientID, input.RoleName); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.system_role_assigned", "group", groupID, input.ClientID, input.RoleName, "", groupAuditDetails(group, nil))
}

func (s *Service) RemoveGroupSystemRole(ctx context.Context, groupID string, clientID string, roleName string) error {
	groupID = strings.TrimSpace(groupID)
	clientID = strings.TrimSpace(clientID)
	roleName = normalize(roleName)
	if groupID == "" || clientID == "" || roleName == "" {
		return fmt.Errorf("%w: groupId, clientId, and roleName are required", ErrInvalidInput)
	}
	group, _ := s.repository.GetGroup(ctx, groupID)
	if err := s.repository.RemoveGroupSystemRole(ctx, groupID, clientID, roleName); err != nil {
		return err
	}
	return s.recordAudit(ctx, "group.system_role_removed", "group", groupID, clientID, roleName, "", groupAuditDetails(group, nil))
}

func (s *Service) AssignRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error {
	realmRole = strings.ToLower(strings.TrimSpace(realmRole))
	permissionKey = strings.TrimSpace(permissionKey)
	if realmRole == "" || permissionKey == "" {
		return fmt.Errorf("%w: realmRole and permissionKey are required", ErrInvalidInput)
	}
	if err := s.requirePermission(ctx, permissionKey); err != nil {
		return err
	}
	if err := s.repository.AssignRealmRolePermission(ctx, realmRole, permissionKey); err != nil {
		return err
	}
	return s.recordAudit(ctx, "realm_role.permission_assigned", "realm-role", realmRole, "", realmRole, permissionKey, nil)
}

func (s *Service) RemoveRealmRolePermission(ctx context.Context, realmRole string, permissionKey string) error {
	realmRole = strings.ToLower(strings.TrimSpace(realmRole))
	permissionKey = strings.TrimSpace(permissionKey)
	if realmRole == "" || permissionKey == "" {
		return fmt.Errorf("%w: realmRole and permissionKey are required", ErrInvalidInput)
	}
	if err := s.repository.RemoveRealmRolePermission(ctx, realmRole, permissionKey); err != nil {
		return err
	}
	return s.recordAudit(ctx, "realm_role.permission_removed", "realm-role", realmRole, "", realmRole, permissionKey, nil)
}

func (s *Service) AddSystemAccessRole(ctx context.Context, clientID string, roleName string) error {
	clientID = strings.TrimSpace(clientID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if clientID == "" || roleName == "" {
		return fmt.Errorf("%w: clientId and roleName are required", ErrInvalidInput)
	}
	if err := s.repository.AddSystemAccessRole(ctx, clientID, roleName); err != nil {
		return err
	}
	return s.recordAudit(ctx, "system.access_role_added", "system", clientID, clientID, roleName, "", nil)
}

func (s *Service) RemoveSystemAccessRole(ctx context.Context, clientID string, roleName string, force bool) error {
	clientID = strings.TrimSpace(clientID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))
	if clientID == "" || roleName == "" {
		return fmt.Errorf("%w: clientId and roleName are required", ErrInvalidInput)
	}
	if !force {
		count, err := s.repository.CountSystemAccessRoles(ctx, clientID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastAccessRole
		}
	}
	if err := s.repository.RemoveSystemAccessRole(ctx, clientID, roleName); err != nil {
		return err
	}
	return s.recordAudit(ctx, "system.access_role_removed", "system", clientID, clientID, roleName, "", map[string]any{"force": force})
}

func (s *Service) GetEffectiveAccess(ctx context.Context, userID string, username string, email string) (EffectiveAccessResponse, error) {
	user, err := s.lookupUser(userID, username, email)
	if err != nil {
		return EffectiveAccessResponse{}, err
	}

	realmGroups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return EffectiveAccessResponse{}, err
	}

	userGroups, err := s.repository.ListGroupsForUser(ctx, user.ID, user.Username, user.Email)
	if err != nil {
		return EffectiveAccessResponse{}, err
	}

	permissionsByKey := map[string]Permission{}
	sources := make([]PermissionGrantSource, 0)

	userRealmRoles := normalizeStringSet(user.RealmRoles)
	effectiveRealmRoles := normalizeStringSet(user.RealmRoles)
	groupRealmRoleSources := make(map[string][]Group)
	groupSystemRoleSources := make(map[string][]Group)
	effectiveClientRoles := sortedClientRoles(user.ClientRoles)

	for _, group := range userGroups {
		for _, permission := range group.Permissions {
			permissionsByKey[permission.Key] = permission
			sources = append(sources, PermissionGrantSource{
				PermissionKey: permission.Key,
				GrantedByType: "groupPermission",
				GroupID:       group.ID,
				GroupPath:     group.Path,
				GroupName:     firstNonEmpty(group.DisplayName, group.Name, group.Path),
			})
		}
		for _, role := range group.RealmRoles {
			role = normalize(role)
			if role == "" {
				continue
			}
			effectiveRealmRoles[role] = true
			groupRealmRoleSources[role] = append(groupRealmRoleSources[role], group)
		}
		for _, role := range group.SystemRoles {
			clientID := strings.TrimSpace(role.ClientID)
			roleName := normalize(role.RoleName)
			if clientID == "" || roleName == "" {
				continue
			}
			effectiveClientRoles[clientID] = sortedStrings(append(effectiveClientRoles[clientID], roleName))
			sourceKey := clientID + "\x00" + roleName
			groupSystemRoleSources[sourceKey] = append(groupSystemRoleSources[sourceKey], group)
		}
	}

	for _, group := range realmGroups {
		role := normalize(group.RealmRole)
		if !effectiveRealmRoles[role] {
			continue
		}
		for _, permission := range group.Permissions {
			permissionsByKey[permission.Key] = permission
			if userRealmRoles[role] {
				sources = append(sources, PermissionGrantSource{
					PermissionKey: permission.Key,
					GrantedByType: "realmRole",
					Role:          role,
				})
			}
			for _, sourceGroup := range groupRealmRoleSources[role] {
				sources = append(sources, PermissionGrantSource{
					PermissionKey: permission.Key,
					GrantedByType: "groupRealmRole",
					Role:          role,
					GroupID:       sourceGroup.ID,
					GroupPath:     sourceGroup.Path,
					GroupName:     firstNonEmpty(sourceGroup.DisplayName, sourceGroup.Name, sourceGroup.Path),
				})
			}
		}
	}

	accessible := make([]SystemAccessSummary, 0)
	for clientID, roles := range effectiveClientRoles {
		clientID = strings.TrimSpace(clientID)
		if clientID == "" || len(roles) == 0 {
			continue
		}

		detail, err := s.repository.GetSystem(ctx, clientID)
		if err != nil || !detail.Enabled {
			continue
		}

		roleSet := normalizeStringSet(roles)
		accessRoles := intersectRoleNames(detail.AccessRoles, roleSet)
		if len(accessRoles) > 0 {
			accessible = append(accessible, SystemAccessSummary{
				ClientID:          detail.ClientID,
				DisplayName:       detail.DisplayName,
				LaunchURL:         detail.LaunchURL,
				Icon:              detail.Icon,
				Category:          detail.Category,
				Navigation:        detail.Navigation,
				SystemType:        detail.SystemType,
				DisplayInLauncher: detail.DisplayInLauncher,
				DisplayInSideNav:  detail.DisplayInSideNav,
				LaunchMode:        detail.LaunchMode,
				SortOrder:         detail.SortOrder,
				Roles:             accessRoles,
			})
		}

		for _, role := range detail.Roles {
			roleName := normalize(role.Name)
			if !roleSet[roleName] || !role.Enabled {
				continue
			}
			for _, permission := range role.Permissions {
				permissionsByKey[permission.Key] = permission
				if normalizeStringSet(user.ClientRoles[clientID])[roleName] {
					sources = append(sources, PermissionGrantSource{
						PermissionKey:  permission.Key,
						GrantedByType:  "clientRole",
						Role:           roleName,
						SystemClientID: clientID,
						SystemName:     firstNonEmpty(detail.DisplayName, clientID),
					})
				}
				sourceKey := clientID + "\x00" + roleName
				for _, sourceGroup := range groupSystemRoleSources[sourceKey] {
					sources = append(sources, PermissionGrantSource{
						PermissionKey:  permission.Key,
						GrantedByType:  "groupClientRole",
						Role:           roleName,
						SystemClientID: clientID,
						SystemName:     firstNonEmpty(detail.DisplayName, clientID),
						GroupID:        sourceGroup.ID,
						GroupPath:      sourceGroup.Path,
						GroupName:      firstNonEmpty(sourceGroup.DisplayName, sourceGroup.Name, sourceGroup.Path),
					})
				}
			}
		}
	}

	permissions := make([]Permission, 0, len(permissionsByKey))
	for _, permission := range permissionsByKey {
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool { return permissions[i].Key < permissions[j].Key })
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].PermissionKey == sources[j].PermissionKey {
			return sources[i].Role < sources[j].Role
		}
		return sources[i].PermissionKey < sources[j].PermissionKey
	})
	sortSystemAccessSummaries(accessible)

	return EffectiveAccessResponse{
		User: EffectiveAccessUser{
			ID:         user.ID,
			Username:   user.Username,
			Email:      user.Email,
			FullName:   user.FullName,
			Enabled:    user.Enabled,
			IsAdmin:    user.IsAdmin,
			IsResolved: true,
		},
		Groups:            groupSummaries(userGroups),
		RealmRoles:        setToSortedStrings(effectiveRealmRoles),
		ClientRoles:       sortedClientRoles(effectiveClientRoles),
		Permissions:       permissions,
		AccessibleSystems: accessible,
		GrantSources:      sources,
	}, nil
}

func (s *Service) UpdatePermissionMetadata(ctx context.Context, permissionKey string, input PermissionMetadataInput) (Permission, error) {
	permissionKey = strings.TrimSpace(permissionKey)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	input.Category = strings.TrimSpace(input.Category)
	input.Status = strings.TrimSpace(input.Status)
	if permissionKey == "" {
		return Permission{}, fmt.Errorf("%w: permissionKey is required", ErrInvalidInput)
	}
	if input.Status == "" {
		input.Status = "active"
	}
	permission, err := s.repository.UpdatePermissionMetadata(ctx, permissionKey, input)
	if err != nil {
		return Permission{}, err
	}
	if err := s.recordAudit(ctx, "permission.metadata_updated", "permission", permission.Key, "", "", permission.Key, map[string]any{"status": input.Status}); err != nil {
		return Permission{}, err
	}
	return permission, nil
}

func (s *Service) GetRoleUsage(ctx context.Context, roleID string) (RoleUsageResponse, error) {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return RoleUsageResponse{}, fmt.Errorf("%w: roleId is required", ErrInvalidInput)
	}
	role, err := s.repository.GetSystemRole(ctx, roleID)
	if err != nil {
		return RoleUsageResponse{}, err
	}
	return RoleUsageResponse{
		Role:             role,
		PermissionsCount: len(role.Permissions),
		Warnings:         []string{"Assigned user counts require live Keycloak role-member lookup and are reported as zero in this preview."},
	}, nil
}

func (s *Service) GetRealmRoleUsage(ctx context.Context, realmRole string) (RealmRoleUsageResponse, error) {
	realmRole = normalize(realmRole)
	if realmRole == "" {
		return RealmRoleUsageResponse{}, fmt.Errorf("%w: realmRole is required", ErrInvalidInput)
	}
	groups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return RealmRoleUsageResponse{}, err
	}
	for _, group := range groups {
		if normalize(group.RealmRole) == realmRole {
			return RealmRoleUsageResponse{
				RealmRole:        realmRole,
				Permissions:      group.Permissions,
				PermissionsCount: len(group.Permissions),
				Warnings:         []string{"Assigned user counts require live Keycloak role-member lookup and are reported as zero in this preview."},
			}, nil
		}
	}
	return RealmRoleUsageResponse{RealmRole: realmRole}, nil
}

func (s *Service) PreviewChange(_ context.Context, input ChangePreviewRequest) (ChangePreviewResponse, error) {
	input.Action = normalize(input.Action)
	input.ResourceType = normalize(input.ResourceType)
	response := ChangePreviewResponse{
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		RiskLevel:    "medium",
		Warnings:     make([]string, 0),
	}
	switch input.Action {
	case "delete-role", "disable-system", "bulk-remove-permission", "import-prune":
		response.HighRisk = true
		response.RiskLevel = "high"
		response.Warnings = append(response.Warnings, "This change can remove access or permissions for existing users. Review effective access before applying.")
	case "remove-permission":
		response.PermissionsRemoved = append(response.PermissionsRemoved, input.PermissionKey)
	case "add-permission":
		response.PermissionsAdded = append(response.PermissionsAdded, input.PermissionKey)
	default:
		response.Warnings = append(response.Warnings, "No specific risk rule matched this change.")
	}
	if input.SystemClientID != "" {
		response.AffectedSystems = append(response.AffectedSystems, input.SystemClientID)
	}
	return response, nil
}

func (s *Service) ExportSeed(ctx context.Context) (systemrbac.SeedFile, error) {
	systems, err := s.repository.ListSystems(ctx)
	if err != nil {
		return systemrbac.SeedFile{}, err
	}
	realmGroups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return systemrbac.SeedFile{}, err
	}
	realmSystemRoles, err := s.repository.ListRealmRoleSystemRoles(ctx)
	if err != nil {
		return systemrbac.SeedFile{}, err
	}

	seed := systemrbac.SeedFile{
		Systems:    make([]systemrbac.SeedSystem, 0, len(systems)),
		RealmRoles: make([]systemrbac.SeedRealmRole, 0, len(realmGroups)),
	}
	for _, system := range systems {
		detail, err := s.repository.GetSystem(ctx, system.ClientID)
		if err != nil {
			return systemrbac.SeedFile{}, err
		}
		enabled := system.Enabled
		seedSystem := systemrbac.SeedSystem{
			ClientID:               system.ClientID,
			DisplayName:            system.DisplayName,
			Description:            system.Description,
			Icon:                   system.Icon,
			LaunchURL:              system.LaunchURL,
			AuthenticatedLaunchURL: system.AuthenticatedLaunchURL,
			Category:               system.Category,
			OwnerTeam:              system.OwnerTeam,
			OwnerName:              system.OwnerName,
			OwnerEmail:             system.OwnerEmail,
			SupportURL:             system.SupportURL,
			DocumentationURL:       system.DocumentationURL,
			Environment:            system.Environment,
			Criticality:            system.Criticality,
			Navigation:             system.Navigation,
			SystemType:             system.SystemType,
			DisplayInLauncher:      boolPointer(system.DisplayInLauncher),
			DisplayInSideNav:       boolPointer(system.DisplayInSideNav),
			LaunchMode:             system.LaunchMode,
			Enabled:                &enabled,
			SortOrder:              system.SortOrder,
			AccessRoles:            detail.AccessRoles,
			Roles:                  make([]systemrbac.SeedRole, 0, len(detail.Roles)),
		}
		for _, role := range detail.Roles {
			permissions := make([]string, 0, len(role.Permissions))
			for _, permission := range role.Permissions {
				permissions = append(permissions, permission.Key)
			}
			seedSystem.Roles = append(seedSystem.Roles, systemrbac.SeedRole{
				Name:        role.Name,
				DisplayName: role.DisplayName,
				Description: role.Description,
				Permissions: permissions,
			})
		}
		seed.Systems = append(seed.Systems, seedSystem)
	}
	defaultRolesByRealm := map[string]map[string][]string{}
	for _, mapping := range realmSystemRoles {
		if defaultRolesByRealm[mapping.RealmRole] == nil {
			defaultRolesByRealm[mapping.RealmRole] = map[string][]string{}
		}
		defaultRolesByRealm[mapping.RealmRole][mapping.ClientID] = append(
			defaultRolesByRealm[mapping.RealmRole][mapping.ClientID],
			mapping.RoleName,
		)
	}
	for _, group := range realmGroups {
		permissions := make([]string, 0, len(group.Permissions))
		for _, permission := range group.Permissions {
			permissions = append(permissions, permission.Key)
		}
		seed.RealmRoles = append(seed.RealmRoles, systemrbac.SeedRealmRole{
			Name:        group.RealmRole,
			Permissions: permissions,
			SystemRoles: defaultRolesByRealm[group.RealmRole],
		})
	}
	return seed, nil
}

func (s *Service) PreviewImport(ctx context.Context, input ImportPreviewRequest) (ImportPreviewResponse, systemrbac.SeedFile, error) {
	seed, err := parseImportSeed(input.Payload)
	if err != nil {
		return ImportPreviewResponse{}, systemrbac.SeedFile{}, err
	}
	if err := systemrbac.ValidateSeed(seed); err != nil {
		return ImportPreviewResponse{}, systemrbac.SeedFile{}, err
	}
	currentSystems, err := s.repository.ListSystems(ctx)
	if err != nil {
		return ImportPreviewResponse{}, systemrbac.SeedFile{}, err
	}
	currentByClient := map[string]bool{}
	for _, system := range currentSystems {
		currentByClient[system.ClientID] = true
	}
	preview := ImportPreviewResponse{Warnings: []string{"Import preview does not delete unmapped RBAC records unless prune is implemented and explicitly enabled."}}
	for _, system := range seed.Systems {
		if currentByClient[system.ClientID] {
			preview.SystemsToUpdate++
		} else {
			preview.SystemsToCreate++
		}
		preview.RolesToCreate += len(system.Roles)
	}
	if input.Prune {
		preview.Warnings = append(preview.Warnings, "Prune was requested but destructive pruning is not enabled in this implementation.")
	}
	return preview, seed, nil
}

func (s *Service) ApplyImport(ctx context.Context, input ImportPreviewRequest) (ImportApplyResponse, error) {
	preview, seed, err := s.PreviewImport(ctx, input)
	if err != nil {
		return ImportApplyResponse{}, err
	}
	for _, system := range seed.Systems {
		_, err := s.UpsertSystem(ctx, UpsertSystemInput{
			ClientID:               system.ClientID,
			DisplayName:            system.DisplayName,
			Description:            system.Description,
			Icon:                   system.Icon,
			LaunchURL:              system.LaunchURL,
			AuthenticatedLaunchURL: system.AuthenticatedLaunchURL,
			Category:               system.Category,
			OwnerTeam:              system.OwnerTeam,
			OwnerName:              system.OwnerName,
			OwnerEmail:             system.OwnerEmail,
			SupportURL:             system.SupportURL,
			DocumentationURL:       system.DocumentationURL,
			Environment:            system.Environment,
			Criticality:            system.Criticality,
			Navigation:             system.Navigation,
			SystemType:             system.SystemType,
			DisplayInLauncher:      system.DisplayInLauncher,
			DisplayInSideNav:       system.DisplayInSideNav,
			LaunchMode:             system.LaunchMode,
			Enabled:                system.Enabled,
			SortOrder:              system.SortOrder,
		})
		if err != nil {
			return ImportApplyResponse{}, err
		}
		for _, accessRole := range system.AccessRoles {
			if err := s.AddSystemAccessRole(ctx, system.ClientID, accessRole); err != nil {
				return ImportApplyResponse{}, err
			}
		}
		for _, role := range system.Roles {
			enabled := true
			systemRole, err := s.repository.UpsertSystemRole(ctx, system.ClientID, RoleInput{
				Name:        role.Name,
				DisplayName: role.DisplayName,
				Description: role.Description,
				Enabled:     &enabled,
			})
			if err != nil {
				return ImportApplyResponse{}, err
			}
			if err := s.recordAudit(ctx, "system_role.imported", "system-role", systemRole.ID, system.ClientID, systemRole.Name, "", nil); err != nil {
				return ImportApplyResponse{}, err
			}
			for _, permission := range role.Permissions {
				if permission != "*" {
					if err := s.AssignSystemRolePermission(ctx, systemRole.ID, permission); err != nil {
						return ImportApplyResponse{}, err
					}
				}
			}
		}
	}
	for _, role := range seed.RealmRoles {
		for _, permission := range role.Permissions {
			if permission != "*" {
				if err := s.AssignRealmRolePermission(ctx, role.Name, permission); err != nil {
					return ImportApplyResponse{}, err
				}
			}
		}
	}

	groupsSynced, err := s.applyDiscoveredGroups(ctx, discoveredGroupsFromSeed(seed.Groups, seed.GroupMemberships), "seed-import")
	if err != nil {
		return ImportApplyResponse{}, err
	}

	if err := s.recordAudit(ctx, "rbac.import_applied", "import", "seed", "", "", "", map[string]any{"systemsToCreate": preview.SystemsToCreate, "systemsToUpdate": preview.SystemsToUpdate, "rolesToCreate": preview.RolesToCreate, "groupsSynced": groupsSynced, "prune": input.Prune}); err != nil {
		return ImportApplyResponse{}, err
	}
	return ImportApplyResponse{Preview: preview, Applied: true, GroupsSynced: groupsSynced}, nil
}

func (s *Service) RoleTemplates() []RoleTemplate {
	return []RoleTemplate{
		{Name: "viewer", DisplayName: "Viewer", Description: "Read-only baseline access", Permissions: []string{"systems:read"}},
		{Name: "data_entry", DisplayName: "Data Entry", Description: "Data entry baseline access", Permissions: []string{"systems:read", "documents:write"}},
		{Name: "manager", DisplayName: "Manager", Description: "Operational manager baseline access", Permissions: []string{"systems:read", "documents:read", "data_quality:read"}},
		{Name: "admin", DisplayName: "Admin", Description: "System administrator baseline access", Permissions: []string{"systems:read", "systems:launch"}},
		{Name: "super_admin", DisplayName: "Super Admin", Description: "High-trust system administrator template", Permissions: []string{"systems:read", "systems:launch"}},
	}
}

func (s *Service) CreateRoleFromTemplate(ctx context.Context, clientID string, input RoleFromTemplateInput) (SystemRole, error) {
	templateName := normalize(input.TemplateName)
	roleName := normalize(firstNonEmpty(input.RoleName, input.TemplateName))
	for _, template := range s.RoleTemplates() {
		if template.Name != templateName {
			continue
		}
		enabled := true
		role, err := s.repository.UpsertSystemRole(ctx, clientID, RoleInput{
			Name:        roleName,
			DisplayName: firstNonEmpty(input.DisplayName, template.DisplayName),
			Description: template.Description,
			Enabled:     &enabled,
		})
		if err != nil {
			return SystemRole{}, err
		}
		for _, permission := range template.Permissions {
			_ = s.AssignSystemRolePermission(ctx, role.ID, permission)
		}
		role, err = s.repository.GetSystemRole(ctx, role.ID)
		if err != nil {
			return SystemRole{}, err
		}
		if err := s.recordAudit(ctx, "system_role.created_from_template", "system-role", role.ID, clientID, role.Name, "", map[string]any{"template": template.Name}); err != nil {
			return SystemRole{}, err
		}
		return role, nil
	}
	return SystemRole{}, fmt.Errorf("%w: unknown template", ErrInvalidInput)
}

func (s *Service) CopyPermissions(ctx context.Context, roleID string, input CopyPermissionsInput) (SystemRole, error) {
	target, err := s.repository.GetSystemRole(ctx, roleID)
	if err != nil {
		return SystemRole{}, err
	}
	source, err := s.repository.GetSystemRole(ctx, input.SourceRoleID)
	if err != nil {
		return SystemRole{}, err
	}
	for _, permission := range source.Permissions {
		if err := s.AssignSystemRolePermission(ctx, target.ID, permission.Key); err != nil {
			return SystemRole{}, err
		}
	}
	role, err := s.repository.GetSystemRole(ctx, target.ID)
	if err != nil {
		return SystemRole{}, err
	}
	if err := s.recordAudit(ctx, "system_role.permissions_copied", "system-role", target.ID, target.ClientID, target.Name, "", map[string]any{"sourceRoleId": source.ID}); err != nil {
		return SystemRole{}, err
	}
	return role, nil
}

func (s *Service) BulkAssignPermission(ctx context.Context, input BulkPermissionInput) error {
	for _, roleID := range input.RoleIDs {
		if err := s.AssignSystemRolePermission(ctx, roleID, input.PermissionKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) BulkRemovePermission(ctx context.Context, input BulkPermissionInput) error {
	for _, roleID := range input.RoleIDs {
		if err := s.RemoveSystemRolePermission(ctx, roleID, input.PermissionKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	filter.ActorUserID = strings.TrimSpace(filter.ActorUserID)
	filter.SystemClientID = strings.TrimSpace(filter.SystemClientID)
	filter.RoleName = strings.TrimSpace(filter.RoleName)
	filter.PermissionKey = strings.TrimSpace(filter.PermissionKey)
	filter.Action = strings.TrimSpace(filter.Action)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	return s.repository.ListAuditEvents(ctx, filter)
}

func (s *Service) CreateAccessRequest(ctx context.Context, input AccessRequestInput) (AccessRequest, error) {
	input.SystemClientID = strings.TrimSpace(input.SystemClientID)
	input.RequestedRole = normalize(input.RequestedRole)
	if input.SystemClientID == "" || input.RequestedRole == "" {
		return AccessRequest{}, fmt.Errorf("%w: systemClientId and requestedRole are required", ErrInvalidInput)
	}
	request, err := s.repository.CreateAccessRequest(ctx, input)
	if err != nil {
		return AccessRequest{}, err
	}
	if err := s.recordAudit(ctx, "access_request.created", "access-request", request.ID, request.SystemClientID, request.RequestedRole, "", map[string]any{"username": input.Username, "email": input.Email}); err != nil {
		return AccessRequest{}, err
	}
	return request, nil
}

func (s *Service) ListAccessRequests(ctx context.Context) ([]AccessRequest, error) {
	return s.repository.ListAccessRequests(ctx)
}

func (s *Service) DecideAccessRequest(ctx context.Context, id string, status string, input AccessRequestDecisionInput) (AccessRequest, error) {
	status = normalize(status)
	if status != "approved" && status != "rejected" && status != "cancelled" {
		return AccessRequest{}, fmt.Errorf("%w: invalid access request status", ErrInvalidInput)
	}
	request, err := s.repository.UpdateAccessRequestStatus(ctx, id, status, input.Note, "")
	if err != nil {
		return AccessRequest{}, err
	}
	if err := s.recordAudit(ctx, "access_request."+status, "access-request", request.ID, request.SystemClientID, request.RequestedRole, "", map[string]any{"note": input.Note}); err != nil {
		return AccessRequest{}, err
	}
	return request, nil
}

func (s *Service) CreateChangeRequest(ctx context.Context, input ChangeRequestInput) (ChangeRequest, error) {
	input.Action = normalize(input.Action)
	input.ResourceType = normalize(input.ResourceType)
	if input.Action == "" || input.ResourceType == "" {
		return ChangeRequest{}, fmt.Errorf("%w: action and resourceType are required", ErrInvalidInput)
	}
	if len(input.Payload) == 0 {
		input.Payload = json.RawMessage(`{}`)
	}
	request, err := s.repository.CreateChangeRequest(ctx, input)
	if err != nil {
		return ChangeRequest{}, err
	}
	if err := s.recordAudit(ctx, "change_request.created", "change-request", request.ID, "", "", "", map[string]any{"action": input.Action, "resourceType": input.ResourceType, "riskLevel": input.RiskLevel}); err != nil {
		return ChangeRequest{}, err
	}
	return request, nil
}

func (s *Service) ListChangeRequests(ctx context.Context) ([]ChangeRequest, error) {
	return s.repository.ListChangeRequests(ctx)
}

func (s *Service) DecideChangeRequest(ctx context.Context, id string, status string, input AccessRequestDecisionInput) (ChangeRequest, error) {
	status = normalize(status)
	if status != "approved" && status != "rejected" && status != "applied" {
		return ChangeRequest{}, fmt.Errorf("%w: invalid change request status", ErrInvalidInput)
	}
	request, err := s.repository.UpdateChangeRequestStatus(ctx, id, status, input.Note, "")
	if err != nil {
		return ChangeRequest{}, err
	}
	if err := s.recordAudit(ctx, "change_request."+status, "change-request", request.ID, "", "", "", map[string]any{"note": input.Note, "action": request.Action, "resourceType": request.ResourceType}); err != nil {
		return ChangeRequest{}, err
	}
	return request, nil
}

func (s *Service) Simulate(ctx context.Context, input SimulationRequest) (SimulationResponse, error) {
	realmRoles := append([]string{}, input.RealmRoles...)
	clientRoles := cloneClientRoles(input.ClientRoles)
	var baselinePermissions []Permission
	warnings := []string{"Simulation is read-only and does not mutate Keycloak or portal RBAC."}

	if strings.TrimSpace(input.UserID) != "" || strings.TrimSpace(input.Username) != "" || strings.TrimSpace(input.Email) != "" {
		baseline, err := s.GetEffectiveAccess(ctx, input.UserID, input.Username, input.Email)
		if err != nil {
			return SimulationResponse{}, err
		}
		baselinePermissions = baseline.Permissions
		if len(realmRoles) == 0 {
			realmRoles = baseline.RealmRoles
		}
		if len(clientRoles) == 0 {
			clientRoles = cloneClientRoles(baseline.ClientRoles)
		}
	}

	realmRoles = applyStringAddsRemoves(realmRoles, input.AddRealmRoles, input.RemoveRealmRoles)
	clientRoles = applyClientRoleAddsRemoves(clientRoles, input.AddClientRoles, input.RemoveClientRoles)

	response, err := s.resolveAccessForRoles(ctx, realmRoles, clientRoles)
	if err != nil {
		return SimulationResponse{}, err
	}
	permissions := append([]Permission{}, response.Permissions...)
	permissions = applyPermissionOverrides(permissions, input.AddPermissions, input.RemovePermissions)

	return SimulationResponse{
		BaselinePermissions: baselinePermissions,
		Permissions:         permissions,
		AddedPermissions:    permissionDiff(permissionKeys(permissions), permissionKeys(baselinePermissions)),
		RemovedPermissions:  permissionDiff(permissionKeys(baselinePermissions), permissionKeys(permissions)),
		AccessibleSystems:   response.AccessibleSystems,
		GrantSources:        response.GrantSources,
		Warnings:            warnings,
	}, nil
}

func cloneClientRoles(input map[string][]string) map[string][]string {
	out := make(map[string][]string, len(input))
	for clientID, roles := range input {
		out[clientID] = append([]string{}, roles...)
	}
	return out
}

func applyStringAddsRemoves(base []string, add []string, remove []string) []string {
	values := normalizeStringSet(base)
	for _, value := range add {
		value = normalize(value)
		if value != "" {
			values[value] = true
		}
	}
	for _, value := range remove {
		delete(values, normalize(value))
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func applyClientRoleAddsRemoves(base map[string][]string, add map[string][]string, remove map[string][]string) map[string][]string {
	out := cloneClientRoles(base)
	for clientID, roles := range add {
		out[clientID] = applyStringAddsRemoves(out[clientID], roles, nil)
	}
	for clientID, roles := range remove {
		out[clientID] = applyStringAddsRemoves(out[clientID], nil, roles)
	}
	return out
}

func applyPermissionOverrides(base []Permission, add []string, remove []string) []Permission {
	byKey := make(map[string]Permission, len(base)+len(add))
	for _, permission := range base {
		byKey[permission.Key] = permission
	}
	for _, key := range add {
		key = strings.TrimSpace(key)
		if key != "" {
			byKey[key] = Permission{Key: key, DisplayName: key, Status: "simulated"}
		}
	}
	for _, key := range remove {
		delete(byKey, strings.TrimSpace(key))
	}
	out := make([]Permission, 0, len(byKey))
	for _, permission := range byKey {
		out = append(out, permission)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func permissionKeys(permissions []Permission) []string {
	out := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, permission.Key)
	}
	sort.Strings(out)
	return out
}

func permissionDiff(left []string, right []string) []string {
	rightSet := normalizeStringSet(right)
	out := make([]string, 0)
	for _, value := range left {
		value = strings.TrimSpace(value)
		if value != "" && !rightSet[value] {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Service) recordAudit(ctx context.Context, action string, resourceType string, resourceID string, systemClientID string, roleName string, permissionKey string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return s.repository.RecordAuditEvent(ctx, AuditEvent{
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		SystemClientID: systemClientID,
		RoleName:       roleName,
		PermissionKey:  permissionKey,
		Details:        payload,
	})
}

func directUserAccess(user *model.User) DirectUserAccessResponse {
	if user == nil {
		return DirectUserAccessResponse{
			RealmRoles:  []string{},
			ClientRoles: map[string][]string{},
			Permissions: []Permission{},
		}
	}
	return DirectUserAccessResponse{
		RealmRoles:  sortedStrings(user.RealmRoles),
		ClientRoles: sortedClientRoles(user.ClientRoles),
		Permissions: []Permission{},
	}
}

func groupAuditDetails(group Group, extra map[string]any) map[string]any {
	details := map[string]any{}
	if strings.TrimSpace(group.ID) != "" {
		details["groupId"] = group.ID
	}
	if strings.TrimSpace(group.Path) != "" {
		details["groupPath"] = group.Path
	}
	if strings.TrimSpace(group.Name) != "" {
		details["groupName"] = group.Name
	}
	if strings.TrimSpace(group.DisplayName) != "" {
		details["groupDisplayName"] = group.DisplayName
	}
	for key, value := range extra {
		details[key] = value
	}
	return details
}

func (s *Service) requirePermission(ctx context.Context, permissionKey string) error {
	exists, err := s.repository.PermissionExists(ctx, permissionKey)
	if err != nil {
		return err
	}
	if !exists {
		return ErrPermissionMissing
	}
	return nil
}

func normalizeRoleInput(input RoleInput) RoleInput {
	input.Name = strings.ToLower(strings.TrimSpace(input.Name))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func boolPointer(value bool) *bool {
	return &value
}

func validateSystemLaunchURL(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "/portal") || strings.HasPrefix(value, "/apps") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return nil
	}
	return fmt.Errorf("%w: launchUrl must be a /portal path, /apps path, or absolute URL", ErrInvalidInput)
}

func validateOptionalURL(field string, value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return nil
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return nil
	}
	return fmt.Errorf("%w: %s must be an absolute URL or root-relative path", ErrInvalidInput, field)
}

func (s *Service) lookupUser(userID string, username string, email string) (*model.User, error) {
	if s.users == nil {
		return nil, fmt.Errorf("%w: user lookup is not configured", ErrInvalidInput)
	}

	userID = strings.TrimSpace(userID)
	if userID != "" {
		id, err := uuid.Parse(userID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid userId", ErrInvalidInput)
		}
		return s.users.GetUser(id)
	}

	username = strings.ToLower(strings.TrimSpace(username))
	email = strings.ToLower(strings.TrimSpace(email))
	if username == "" && email == "" {
		return nil, fmt.Errorf("%w: userId, username, or email is required", ErrInvalidInput)
	}

	users, err := s.users.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if username != "" && strings.EqualFold(user.Username, username) {
			return &user, nil
		}
		if email != "" && strings.EqualFold(user.Email, email) {
			return &user, nil
		}
	}
	return nil, fmt.Errorf("%w: user not found", ErrInvalidInput)
}

func (s *Service) resolveAccessForRoles(ctx context.Context, realmRoles []string, clientRoles map[string][]string) (EffectiveAccessResponse, error) {
	realmGroups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return EffectiveAccessResponse{}, err
	}

	permissionsByKey := map[string]Permission{}
	sources := make([]PermissionGrantSource, 0)
	userRealmRoles := normalizeStringSet(realmRoles)
	for _, group := range realmGroups {
		role := normalize(group.RealmRole)
		if !userRealmRoles[role] {
			continue
		}
		for _, permission := range group.Permissions {
			permissionsByKey[permission.Key] = permission
			sources = append(sources, PermissionGrantSource{
				PermissionKey: permission.Key,
				GrantedByType: "realmRole",
				Role:          role,
			})
		}
	}

	accessible := make([]SystemAccessSummary, 0)
	for clientID, roles := range clientRoles {
		clientID = strings.TrimSpace(clientID)
		if clientID == "" || len(roles) == 0 {
			continue
		}
		detail, err := s.repository.GetSystem(ctx, clientID)
		if err != nil || !detail.Enabled {
			continue
		}
		roleSet := normalizeStringSet(roles)
		accessRoles := intersectRoleNames(detail.AccessRoles, roleSet)
		if len(accessRoles) > 0 {
			accessible = append(accessible, SystemAccessSummary{
				ClientID:          detail.ClientID,
				DisplayName:       detail.DisplayName,
				LaunchURL:         detail.LaunchURL,
				Icon:              detail.Icon,
				Category:          detail.Category,
				Navigation:        detail.Navigation,
				SystemType:        detail.SystemType,
				DisplayInLauncher: detail.DisplayInLauncher,
				DisplayInSideNav:  detail.DisplayInSideNav,
				LaunchMode:        detail.LaunchMode,
				SortOrder:         detail.SortOrder,
				Roles:             accessRoles,
			})
		}
		for _, role := range detail.Roles {
			roleName := normalize(role.Name)
			if !roleSet[roleName] || !role.Enabled {
				continue
			}
			for _, permission := range role.Permissions {
				permissionsByKey[permission.Key] = permission
				sources = append(sources, PermissionGrantSource{
					PermissionKey:  permission.Key,
					GrantedByType:  "clientRole",
					Role:           roleName,
					SystemClientID: clientID,
					SystemName:     firstNonEmpty(detail.DisplayName, clientID),
				})
			}
		}
	}

	permissions := make([]Permission, 0, len(permissionsByKey))
	for _, permission := range permissionsByKey {
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool { return permissions[i].Key < permissions[j].Key })
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].PermissionKey == sources[j].PermissionKey {
			return sources[i].Role < sources[j].Role
		}
		return sources[i].PermissionKey < sources[j].PermissionKey
	})
	sortSystemAccessSummaries(accessible)

	return EffectiveAccessResponse{
		RealmRoles:        sortedStrings(realmRoles),
		ClientRoles:       sortedClientRoles(clientRoles),
		Permissions:       permissions,
		AccessibleSystems: accessible,
		GrantSources:      sources,
	}, nil
}

func sortSystemAccessSummaries(accessible []SystemAccessSummary) {
	sort.Slice(accessible, func(i, j int) bool {
		if accessible[i].SortOrder != accessible[j].SortOrder {
			return accessible[i].SortOrder < accessible[j].SortOrder
		}
		if accessible[i].DisplayName != accessible[j].DisplayName {
			return accessible[i].DisplayName < accessible[j].DisplayName
		}
		return accessible[i].ClientID < accessible[j].ClientID
	})
}

func parseImportSeed(payload json.RawMessage) (systemrbac.SeedFile, error) {
	if len(payload) == 0 {
		return systemrbac.SeedFile{}, fmt.Errorf("%w: import payload is required", ErrInvalidInput)
	}

	if json.Valid(payload) {
		var rawSeed string
		if err := json.Unmarshal(payload, &rawSeed); err == nil {
			var seed systemrbac.SeedFile
			if err := yaml.Unmarshal([]byte(rawSeed), &seed); err != nil {
				return systemrbac.SeedFile{}, fmt.Errorf("%w: invalid seed payload", ErrInvalidInput)
			}
			return seed, nil
		}
	}

	if json.Valid(payload) {
		var seed systemrbac.SeedFile
		if err := json.Unmarshal(payload, &seed); err != nil {
			return systemrbac.SeedFile{}, err
		}
		return seed, nil
	}

	var seed systemrbac.SeedFile
	if err := yaml.Unmarshal(payload, &seed); err != nil {
		return systemrbac.SeedFile{}, fmt.Errorf("%w: invalid seed payload", ErrInvalidInput)
	}
	return seed, nil
}

func normalizeStringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		value = normalize(value)
		if value != "" {
			set[value] = true
		}
	}
	return set
}

func setToSortedStrings(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value, ok := range values {
		if ok && strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func groupSummaries(groups []Group) []GroupSummary {
	out := make([]GroupSummary, 0, len(groups))
	for _, group := range groups {
		out = append(out, GroupSummary{
			ID:              group.ID,
			KeycloakGroupID: group.KeycloakGroupID,
			Path:            group.Path,
			Name:            group.Name,
			DisplayName:     group.DisplayName,
			Enabled:         group.Enabled,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func intersectRoleNames(values []string, allowed map[string]bool) []string {
	out := make([]string, 0)
	for _, value := range values {
		value = normalize(value)
		if value != "" && allowed[value] {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalize(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortedClientRoles(values map[string][]string) map[string][]string {
	out := make(map[string][]string, len(values))
	for clientID, roles := range values {
		clientID = strings.TrimSpace(clientID)
		if clientID == "" {
			continue
		}
		out[clientID] = sortedStrings(roles)
	}
	return out
}

func validateAssignableUserAccess(
	assignable AssignableUserAccessResponse,
	realmRoles []string,
	clientRoles map[string][]string,
	directPermissions []string,
) error {
	if len(directPermissions) > 0 {
		return fmt.Errorf("%w: direct user permissions are not supported; assign permissions through realm or system roles", ErrInvalidInput)
	}

	realmSet := map[string]bool{}
	for _, role := range assignable.RealmRoles {
		realmSet[normalize(role.Name)] = true
	}

	for _, role := range realmRoles {
		if !realmSet[normalize(role)] {
			return fmt.Errorf("%w: realm role %q is not assignable", ErrInvalidInput, role)
		}
	}

	systemRoles := map[string]map[string]bool{}
	for _, system := range assignable.Systems {
		roleSet := map[string]bool{}
		for _, role := range system.Roles {
			roleSet[normalize(role.Name)] = true
		}
		systemRoles[system.ClientID] = roleSet
	}

	for clientID, roles := range clientRoles {
		roleSet, ok := systemRoles[clientID]
		if !ok {
			return fmt.Errorf("%w: system %q is not assignable", ErrInvalidInput, clientID)
		}

		for _, role := range roles {
			if !roleSet[normalize(role)] {
				return fmt.Errorf("%w: role %q is not assignable for system %q", ErrInvalidInput, role, clientID)
			}
		}
	}

	return nil
}

func diffClientRoles(before map[string][]string, after map[string][]string) (map[string][]string, map[string][]string) {
	added := map[string][]string{}
	removed := map[string][]string{}

	seen := map[string]bool{}
	for clientID := range before {
		seen[clientID] = true
	}
	for clientID := range after {
		seen[clientID] = true
	}

	for clientID := range seen {
		addedRoles := permissionDiff(after[clientID], before[clientID])
		removedRoles := permissionDiff(before[clientID], after[clientID])
		if len(addedRoles) > 0 {
			added[clientID] = addedRoles
		}
		if len(removedRoles) > 0 {
			removed[clientID] = removedRoles
		}
	}

	return added, removed
}
