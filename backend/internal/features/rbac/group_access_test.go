package rbac

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
)

func TestGetEffectiveAccessIncludesGroupDerivedRolesAndPermissions(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	users := &testUserLookup{
		user: &model.User{
			ID:          "11111111-1111-1111-1111-111111111111",
			Username:    "data.officer",
			Email:       "data.officer@example.org",
			FullName:    "Data Officer",
			RealmRoles:  []string{"user"},
			ClientRoles: map[string][]string{},
			Enabled:     true,
		},
	}
	service := NewService(repo, users)

	access, err := service.GetEffectiveAccess(ctx, users.user.ID, "", "")
	if err != nil {
		t.Fatalf("GetEffectiveAccess returned error: %v", err)
	}

	if !containsString(access.RealmRoles, "data_officer") {
		t.Fatalf("expected group realm role in effective roles, got %#v", access.RealmRoles)
	}
	if !containsString(access.ClientRoles["data-statistics"], "document_viewer") {
		t.Fatalf("expected group client role in effective client roles, got %#v", access.ClientRoles)
	}
	if !containsPermission(access.Permissions, "documents:read") {
		t.Fatalf("expected documents:read from group client role, got %#v", access.Permissions)
	}
	if !containsPermission(access.Permissions, "data_quality:read") {
		t.Fatalf("expected data_quality:read from group direct permission, got %#v", access.Permissions)
	}
	if len(access.Groups) != 1 || access.Groups[0].Path != "/Data Officers" {
		t.Fatalf("expected group summary, got %#v", access.Groups)
	}

	var foundGroupSource bool
	for _, source := range access.GrantSources {
		if source.PermissionKey == "documents:read" &&
			source.GrantedByType == "groupClientRole" &&
			source.GroupPath == "/Data Officers" &&
			source.SystemClientID == "data-statistics" {
			foundGroupSource = true
			break
		}
	}
	if !foundGroupSource {
		t.Fatalf("expected group client role grant source, got %#v", access.GrantSources)
	}
}

func TestGetUserAccessProfileSeparatesDirectAndInheritedAccess(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	users := &testUserLookup{
		user: &model.User{
			ID:          "11111111-1111-1111-1111-111111111111",
			Username:    "data.officer",
			Email:       "data.officer@example.org",
			RealmRoles:  []string{"user"},
			ClientRoles: map[string][]string{},
			Enabled:     true,
		},
	}
	service := NewService(repo, users)

	profile, err := service.GetUserAccessProfile(ctx, users.user.ID)
	if err != nil {
		t.Fatalf("GetUserAccessProfile returned error: %v", err)
	}

	if containsString(profile.DirectAccess.RealmRoles, "data_officer") {
		t.Fatalf("direct access should not include inherited group realm role: %#v", profile.DirectAccess.RealmRoles)
	}
	if len(profile.DirectAccess.ClientRoles["data-statistics"]) > 0 {
		t.Fatalf("direct access should not include inherited group client roles: %#v", profile.DirectAccess.ClientRoles)
	}
	if !containsString(profile.EffectiveAccess.RealmRoles, "data_officer") {
		t.Fatalf("effective access should include inherited group realm role: %#v", profile.EffectiveAccess.RealmRoles)
	}
}

func TestAssignGroupPermissionWritesSafeAuditDetails(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	service := NewService(repo, &testUserLookup{})

	if err := service.AssignGroupPermission(ctx, "group-data", "documents:read"); err != nil {
		t.Fatalf("AssignGroupPermission returned error: %v", err)
	}

	if len(repo.auditEvents) != 1 {
		t.Fatalf("expected one audit event, got %d", len(repo.auditEvents))
	}
	event := repo.auditEvents[0]
	if event.Action != "group.permission_assigned" || event.ResourceID != "group-data" || event.PermissionKey != "documents:read" {
		t.Fatalf("unexpected audit event: %#v", event)
	}
	var details map[string]any
	if err := json.Unmarshal(event.Details, &details); err != nil {
		t.Fatalf("unmarshal audit details: %v", err)
	}
	if details["groupPath"] != "/Data Officers" || details["groupName"] != "Data Officers" {
		t.Fatalf("expected safe group metadata in audit details, got %#v", details)
	}
}

func TestSyncLiveKeycloakGroupsPersistsMembershipRolesAndAudit(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	service := NewService(repo, &testUserLookup{})
	source := &testKeycloakGroupSource{}

	synced, warnings := service.syncLiveKeycloakGroups(ctx, source, []KeycloakDiscoveredSystem{
		{ClientID: "data-statistics"},
	})
	if synced != 1 {
		t.Fatalf("expected one group synced, got %d", synced)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(repo.groupMembers) != 1 || repo.groupMembers[0].Username != "data.officer" {
		t.Fatalf("expected synced group member, got %#v", repo.groupMembers)
	}
	if !containsString(repo.groupRealmRoles, "data_officer") {
		t.Fatalf("expected synced group realm role, got %#v", repo.groupRealmRoles)
	}
	if len(repo.groupSystemRoles) != 1 ||
		repo.groupSystemRoles[0].ClientID != "data-statistics" ||
		repo.groupSystemRoles[0].RoleName != "document_viewer" {
		t.Fatalf("expected synced group system role, got %#v", repo.groupSystemRoles)
	}
	if len(repo.auditEvents) != 1 || repo.auditEvents[0].Action != "rbac.groups_synced" {
		t.Fatalf("expected group sync audit event, got %#v", repo.auditEvents)
	}
}

func TestGroupMemberMutationsWriteThroughKeycloakAndRefreshCache(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	service := NewService(repo, &testUserLookup{})
	source := &testKeycloakGroupSource{
		members: []keycloak.KeycloakUser{{
			ID:       "11111111-1111-1111-1111-111111111111",
			Username: "data.officer",
			Email:    "data.officer@example.org",
		}},
	}
	service.SetKeycloakGroupMembershipManager(source)

	result, err := service.AddGroupMember(ctx, "group-data", "11111111-1111-1111-1111-111111111111", uuid.Nil)
	if err != nil {
		t.Fatalf("AddGroupMember returned error: %v", err)
	}
	if source.addedUserID != "11111111-1111-1111-1111-111111111111" || source.addedGroupID != "kc-group-data" {
		t.Fatalf("expected Keycloak add call, got user=%q group=%q", source.addedUserID, source.addedGroupID)
	}
	if result.MemberCount != 1 || len(repo.groupMembers) != 1 || repo.groupMembers[0].Username != "data.officer" {
		t.Fatalf("expected local group cache refresh, got result=%#v repo=%#v", result, repo.groupMembers)
	}
	if len(repo.auditEvents) < 2 || repo.auditEvents[len(repo.auditEvents)-1].Action != "group.member_added" {
		t.Fatalf("expected group.member_added audit event, got %#v", repo.auditEvents)
	}

	source.members = []keycloak.KeycloakUser{}
	result, err = service.RemoveGroupMember(ctx, "group-data", "11111111-1111-1111-1111-111111111111", uuid.Nil)
	if err != nil {
		t.Fatalf("RemoveGroupMember returned error: %v", err)
	}
	if source.removedUserID != "11111111-1111-1111-1111-111111111111" || source.removedGroupID != "kc-group-data" {
		t.Fatalf("expected Keycloak remove call, got user=%q group=%q", source.removedUserID, source.removedGroupID)
	}
	if result.MemberCount != 0 || len(repo.groupMembers) != 0 {
		t.Fatalf("expected empty local group cache after remove, got result=%#v repo=%#v", result, repo.groupMembers)
	}
	if repo.auditEvents[len(repo.auditEvents)-1].Action != "group.member_removed" {
		t.Fatalf("expected group.member_removed audit event, got %#v", repo.auditEvents)
	}
}

func TestGroupMemberMutationCreatesMissingKeycloakGroup(t *testing.T) {
	ctx := context.Background()
	repo := newTestRBACRepository()
	repo.group.KeycloakGroupID = ""
	repo.group.Path = "/MOH/Data and Statistics/Report Viewers"
	repo.group.Name = "Report Viewers"
	service := NewService(repo, &testUserLookup{})
	source := &testKeycloakGroupSource{
		groups:   []keycloak.GroupRep{},
		ensureID: "kc-report-viewers",
		members: []keycloak.KeycloakUser{{
			ID:       "11111111-1111-1111-1111-111111111111",
			Username: "data.officer",
			Email:    "data.officer@example.org",
		}},
	}
	service.SetKeycloakGroupMembershipManager(source)

	result, err := service.AddGroupMember(ctx, "group-data", "11111111-1111-1111-1111-111111111111", uuid.Nil)
	if err != nil {
		t.Fatalf("AddGroupMember returned error: %v", err)
	}
	if source.ensurePath != "/MOH/Data and Statistics/Report Viewers" {
		t.Fatalf("expected missing Keycloak group path to be ensured, got %q", source.ensurePath)
	}
	if source.addedGroupID != "kc-report-viewers" {
		t.Fatalf("expected user added to created Keycloak group, got %q", source.addedGroupID)
	}
	if repo.group.KeycloakGroupID != "kc-report-viewers" {
		t.Fatalf("expected local group to cache created Keycloak id, got %q", repo.group.KeycloakGroupID)
	}
	if result.MemberCount != 1 {
		t.Fatalf("expected member sync after create, got %#v", result)
	}
}

type testUserLookup struct {
	user *model.User
}

func (l *testUserLookup) GetUser(id uuid.UUID) (*model.User, error) {
	if l.user == nil {
		return nil, ErrInvalidInput
	}
	return l.user, nil
}

func (l *testUserLookup) ListUsers() ([]model.User, error) {
	if l.user == nil {
		return []model.User{}, nil
	}
	return []model.User{*l.user}, nil
}

func (l *testUserLookup) UpdateUserRealmRoles(context.Context, uuid.UUID, []string, uuid.UUID) error {
	return nil
}

func (l *testUserLookup) UpdateUserClientRoles(context.Context, uuid.UUID, string, string, []string, uuid.UUID) error {
	return nil
}

type testRBACRepository struct {
	group            Group
	groupMembers     []GroupMember
	groupRealmRoles  []string
	groupSystemRoles []GroupSystemRole
	auditEvents      []AuditEvent
}

func newTestRBACRepository() *testRBACRepository {
	return &testRBACRepository{
		group: Group{
			ID:              "group-data",
			KeycloakGroupID: "kc-group-data",
			Path:            "/Data Officers",
			Name:            "Data Officers",
			DisplayName:     "Data Officers",
			Enabled:         true,
			RealmRoles:      []string{"data_officer"},
			SystemRoles: []GroupSystemRole{
				{ClientID: "data-statistics", RoleName: "document_viewer"},
			},
			Permissions: []Permission{
				{Key: "data_quality:read", DisplayName: "Read data quality", Category: "data_quality", Status: "active"},
			},
		},
	}
}

func (r *testRBACRepository) ListSystems(context.Context) ([]System, error) {
	return []System{{
		ID:                "system-data",
		ClientID:          "data-statistics",
		DisplayName:       "Data & Statistics",
		Enabled:           true,
		DisplayInLauncher: true,
		DisplayInSideNav:  true,
		LaunchMode:        "internal",
	}}, nil
}

func (r *testRBACRepository) GetSystem(_ context.Context, clientID string) (SystemDetail, error) {
	return SystemDetail{
		System: System{
			ID:                "system-data",
			ClientID:          clientID,
			DisplayName:       "Data & Statistics",
			Enabled:           true,
			DisplayInLauncher: true,
			DisplayInSideNav:  true,
			LaunchMode:        "internal",
		},
		AccessRoles: []string{"data-statistics_access", "document_viewer"},
		Roles: []SystemRole{{
			ID:       "role-document-viewer",
			ClientID: clientID,
			Name:     "document_viewer",
			Enabled:  true,
			Permissions: []Permission{
				{Key: "documents:read", DisplayName: "Read documents", Category: "documents", Status: "active"},
			},
		}},
	}, nil
}

func (r *testRBACRepository) TransitionSystem(context.Context, string, LifecycleTransitionInput, string, func(System, int) error) (System, error) {
	return System{}, nil
}
func (r *testRBACRepository) UpsertSystem(context.Context, UpsertSystemInput) (System, error) {
	return System{}, nil
}
func (r *testRBACRepository) ListPermissions(context.Context) ([]Permission, error) {
	return []Permission{
		{Key: "documents:read", Status: "active"},
		{Key: "data_quality:read", Status: "active"},
	}, nil
}
func (r *testRBACRepository) UpdatePermissionMetadata(context.Context, string, PermissionMetadataInput) (Permission, error) {
	return Permission{}, nil
}
func (r *testRBACRepository) ListSystemRoles(context.Context, string) ([]SystemRole, error) {
	return nil, nil
}
func (r *testRBACRepository) CreateSystemRole(context.Context, string, RoleInput) (SystemRole, error) {
	return SystemRole{}, nil
}
func (r *testRBACRepository) UpsertSystemRole(context.Context, string, RoleInput) (SystemRole, error) {
	return SystemRole{}, nil
}
func (r *testRBACRepository) UpdateSystemRole(context.Context, string, RoleInput) (SystemRole, error) {
	return SystemRole{}, nil
}
func (r *testRBACRepository) DeleteSystemRole(context.Context, string) error { return nil }
func (r *testRBACRepository) AssignSystemRolePermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) RemoveSystemRolePermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) ListRealmRolePermissions(context.Context) ([]RealmRolePermissionGroup, error) {
	return []RealmRolePermissionGroup{{
		RealmRole: "data_officer",
		Permissions: []Permission{
			{Key: "documents:read", DisplayName: "Read documents", Category: "documents", Status: "active"},
		},
	}}, nil
}
func (r *testRBACRepository) AssignRealmRolePermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) RemoveRealmRolePermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) ListRealmRoleSystemRoles(context.Context) ([]RealmRoleSystemRole, error) {
	return nil, nil
}
func (r *testRBACRepository) ListGroups(context.Context) ([]Group, error) {
	return []Group{r.group}, nil
}
func (r *testRBACRepository) GetGroup(context.Context, string) (Group, error) {
	return r.group, nil
}
func (r *testRBACRepository) ListGroupsForUser(context.Context, string, string, string) ([]Group, error) {
	return []Group{r.group}, nil
}
func (r *testRBACRepository) UpsertGroup(_ context.Context, input GroupInput) (Group, error) {
	group := r.group
	group.KeycloakGroupID = input.KeycloakGroupID
	group.Path = input.Path
	group.Name = input.Name
	group.DisplayName = input.DisplayName
	r.group = group
	return group, nil
}
func (r *testRBACRepository) ListGroupMembers(context.Context, string) ([]GroupMember, error) {
	return nil, nil
}
func (r *testRBACRepository) ReplaceGroupMembers(_ context.Context, _ string, members []GroupMember) error {
	r.groupMembers = members
	return nil
}
func (r *testRBACRepository) AssignGroupPermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) RemoveGroupPermission(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) AssignGroupRealmRole(_ context.Context, _ string, realmRole string) error {
	r.groupRealmRoles = append(r.groupRealmRoles, realmRole)
	return nil
}
func (r *testRBACRepository) RemoveGroupRealmRole(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) AssignGroupSystemRole(_ context.Context, _ string, clientID string, roleName string) error {
	r.groupSystemRoles = append(r.groupSystemRoles, GroupSystemRole{ClientID: clientID, RoleName: roleName})
	return nil
}
func (r *testRBACRepository) RemoveGroupSystemRole(context.Context, string, string, string) error {
	return nil
}
func (r *testRBACRepository) AddSystemAccessRole(context.Context, string, string) error { return nil }
func (r *testRBACRepository) RemoveSystemAccessRole(context.Context, string, string) error {
	return nil
}
func (r *testRBACRepository) CountSystemAccessRoles(context.Context, string) (int, error) {
	return 1, nil
}
func (r *testRBACRepository) PermissionExists(_ context.Context, permissionKey string) (bool, error) {
	return permissionKey == "documents:read" || permissionKey == "data_quality:read", nil
}
func (r *testRBACRepository) GetSystemRole(context.Context, string) (SystemRole, error) {
	return SystemRole{}, nil
}
func (r *testRBACRepository) ListAuditEvents(context.Context, AuditFilter) ([]AuditEvent, error) {
	return r.auditEvents, nil
}
func (r *testRBACRepository) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	r.auditEvents = append(r.auditEvents, event)
	return nil
}
func (r *testRBACRepository) CreateAccessRequest(context.Context, AccessRequestInput) (AccessRequest, error) {
	return AccessRequest{}, nil
}
func (r *testRBACRepository) ListAccessRequests(context.Context) ([]AccessRequest, error) {
	return nil, nil
}
func (r *testRBACRepository) UpdateAccessRequestStatus(context.Context, string, string, string, string) (AccessRequest, error) {
	return AccessRequest{}, nil
}
func (r *testRBACRepository) CreateChangeRequest(context.Context, ChangeRequestInput) (ChangeRequest, error) {
	return ChangeRequest{}, nil
}
func (r *testRBACRepository) ListChangeRequests(context.Context) ([]ChangeRequest, error) {
	return nil, nil
}
func (r *testRBACRepository) UpdateChangeRequestStatus(context.Context, string, string, string, string) (ChangeRequest, error) {
	return ChangeRequest{}, nil
}

type testKeycloakGroupSource struct {
	members        []keycloak.KeycloakUser
	addedUserID    string
	addedGroupID   string
	removedUserID  string
	removedGroupID string
	ensurePath     string
	ensureID       string
	groups         []keycloak.GroupRep
}

func (s *testKeycloakGroupSource) ListGroups(context.Context) ([]keycloak.GroupRep, error) {
	if s.groups != nil {
		return s.groups, nil
	}
	return []keycloak.GroupRep{{
		ID:   "kc-group-data",
		Name: "Data Officers",
		Path: "/Data Officers",
	}}, nil
}

func (s *testKeycloakGroupSource) EnsureGroupPath(_ context.Context, groupPath string, _ string, _ string, _ map[string][]string) (keycloak.GroupRep, error) {
	s.ensurePath = groupPath
	if s.ensureID == "" {
		s.ensureID = "kc-created-group"
	}
	return keycloak.GroupRep{
		ID:   s.ensureID,
		Name: "Created Group",
		Path: groupPath,
	}, nil
}

func (s *testKeycloakGroupSource) ListGroupMembers(context.Context, string) ([]keycloak.KeycloakUser, error) {
	if s.members != nil {
		return s.members, nil
	}
	return []keycloak.KeycloakUser{{
		ID:       "11111111-1111-1111-1111-111111111111",
		Username: "data.officer",
		Email:    "data.officer@example.org",
	}}, nil
}

func (s *testKeycloakGroupSource) GetUser(userID string) (*keycloak.UserInfo, error) {
	return &keycloak.UserInfo{
		ID:       userID,
		Username: "data.officer",
		Email:    "data.officer@example.org",
		Enabled:  true,
	}, nil
}

func (s *testKeycloakGroupSource) AddUserToGroup(_ context.Context, userID string, groupID string) error {
	s.addedUserID = userID
	s.addedGroupID = groupID
	return nil
}

func (s *testKeycloakGroupSource) RemoveUserFromGroup(_ context.Context, userID string, groupID string) error {
	s.removedUserID = userID
	s.removedGroupID = groupID
	return nil
}

func (s *testKeycloakGroupSource) ListGroupRealmRoles(context.Context, string) ([]keycloak.RoleRep, error) {
	return []keycloak.RoleRep{{Name: "data_officer"}}, nil
}

func (s *testKeycloakGroupSource) ListGroupClientRoles(context.Context, string, string) ([]keycloak.ClientRoleRep, error) {
	return []keycloak.ClientRoleRep{{Name: "document_viewer"}}, nil
}

func containsPermission(permissions []Permission, key string) bool {
	for _, permission := range permissions {
		if permission.Key == key {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
