package rbac

import "encoding/json"

type System struct {
	ID                     string `json:"id"`
	ClientID               string `json:"clientId"`
	DisplayName            string `json:"displayName"`
	Description            string `json:"description,omitempty"`
	Icon                   string `json:"icon,omitempty"`
	LaunchURL              string `json:"launchUrl,omitempty"`
	AuthenticatedLaunchURL string `json:"authenticatedLaunchUrl,omitempty"`
	Category               string `json:"category,omitempty"`
	OwnerTeam              string `json:"ownerTeam,omitempty"`
	OwnerName              string `json:"ownerName,omitempty"`
	OwnerEmail             string `json:"ownerEmail,omitempty"`
	SupportURL             string `json:"supportUrl,omitempty"`
	DocumentationURL       string `json:"documentationUrl,omitempty"`
	Environment            string `json:"environment,omitempty"`
	Criticality            string `json:"criticality,omitempty"`
	Navigation             string `json:"navigation,omitempty"`
	SystemType             string `json:"systemType"`
	DisplayInLauncher      bool   `json:"displayInLauncher"`
	DisplayInSideNav       bool   `json:"displayInSideNav"`
	LaunchMode             string `json:"launchMode"`
	Enabled                bool   `json:"enabled"`
	SortOrder              int32  `json:"sortOrder"`
	LifecycleStatus        string `json:"lifecycleStatus"`
	IntegrationType        string `json:"integrationType"`
	ContractVersion        string `json:"contractVersion"`
}

type SystemDetail struct {
	System
	AccessRoles []string     `json:"accessRoles"`
	Roles       []SystemRole `json:"roles"`
}

type Permission struct {
	ID                   string `json:"id"`
	Key                  string `json:"key"`
	DisplayName          string `json:"displayName,omitempty"`
	Description          string `json:"description,omitempty"`
	Category             string `json:"category,omitempty"`
	Status               string `json:"status,omitempty"`
	SystemRoleUsageCount int    `json:"systemRoleUsageCount"`
	RealmRoleUsageCount  int    `json:"realmRoleUsageCount"`
}

type SystemRole struct {
	ID          string       `json:"id"`
	SystemID    string       `json:"systemId"`
	ClientID    string       `json:"clientId"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Permissions []Permission `json:"permissions"`
}

type RealmRolePermissionGroup struct {
	RealmRole   string       `json:"realmRole"`
	Permissions []Permission `json:"permissions"`
}

type RealmRoleSystemRole struct {
	RealmRole string `json:"realmRole"`
	ClientID  string `json:"clientId"`
	RoleName  string `json:"roleName"`
}

type Group struct {
	ID              string            `json:"id"`
	KeycloakGroupID string            `json:"keycloakGroupId,omitempty"`
	Path            string            `json:"path"`
	Name            string            `json:"name"`
	DisplayName     string            `json:"displayName,omitempty"`
	Description     string            `json:"description,omitempty"`
	Enabled         bool              `json:"enabled"`
	MemberCount     int               `json:"memberCount"`
	RealmRoles      []string          `json:"realmRoles"`
	SystemRoles     []GroupSystemRole `json:"systemRoles"`
	Permissions     []Permission      `json:"permissions"`
}

type GroupSummary struct {
	ID              string `json:"id"`
	KeycloakGroupID string `json:"keycloakGroupId,omitempty"`
	Path            string `json:"path"`
	Name            string `json:"name"`
	DisplayName     string `json:"displayName,omitempty"`
	Enabled         bool   `json:"enabled"`
}

type GroupSystemRole struct {
	RoleID      string `json:"roleId"`
	ClientID    string `json:"clientId"`
	RoleName    string `json:"roleName"`
	DisplayName string `json:"displayName,omitempty"`
}

type GroupMember struct {
	UserID   string `json:"userId"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type GroupInput struct {
	KeycloakGroupID string `json:"keycloakGroupId"`
	Path            string `json:"path"`
	Name            string `json:"name"`
	DisplayName     string `json:"displayName"`
	Description     string `json:"description"`
	Enabled         *bool  `json:"enabled"`
}

type GroupMembersInput struct {
	Members []GroupMember `json:"members"`
}

type GroupMemberInput struct {
	UserID string `json:"userId" binding:"required"`
}

type GroupMembersSyncResponse struct {
	Members     []GroupMember `json:"members"`
	MemberCount int           `json:"memberCount"`
	Warnings    []string      `json:"warnings,omitempty"`
}

type GroupRealmRoleInput struct {
	RealmRole string `json:"realmRole" binding:"required"`
}

type GroupSystemRoleInput struct {
	ClientID string `json:"clientId" binding:"required"`
	RoleName string `json:"roleName" binding:"required"`
}

type LifecycleTransitionInput struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

type UpsertSystemInput struct {
	RegisterOnly           bool   `json:"-"`
	ActorUserID            string `json:"-"`
	IntegrationType        string `json:"integrationType"`
	ContractVersion        string `json:"contractVersion"`
	ClientID               string `json:"clientId"`
	DisplayName            string `json:"displayName"`
	Description            string `json:"description"`
	Icon                   string `json:"icon"`
	LaunchURL              string `json:"launchUrl"`
	AuthenticatedLaunchURL string `json:"authenticatedLaunchUrl"`
	Category               string `json:"category"`
	OwnerTeam              string `json:"ownerTeam"`
	OwnerName              string `json:"ownerName"`
	OwnerEmail             string `json:"ownerEmail"`
	SupportURL             string `json:"supportUrl"`
	DocumentationURL       string `json:"documentationUrl"`
	Environment            string `json:"environment"`
	Criticality            string `json:"criticality"`
	Navigation             string `json:"navigation"`
	SystemType             string `json:"systemType"`
	DisplayInLauncher      *bool  `json:"displayInLauncher"`
	DisplayInSideNav       *bool  `json:"displayInSideNav"`
	LaunchMode             string `json:"launchMode"`
	Enabled                *bool  `json:"enabled"`
	SortOrder              int32  `json:"sortOrder"`
}

type RoleInput struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type PermissionInput struct {
	PermissionKey string `json:"permissionKey" binding:"required"`
}

type AccessRoleInput struct {
	RoleName string `json:"roleName" binding:"required"`
}

type KeycloakDiscoveredRole struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type KeycloakDiscoveredSystem struct {
	ClientID               string                   `json:"clientId"`
	DisplayName            string                   `json:"displayName"`
	Description            string                   `json:"description,omitempty"`
	Icon                   string                   `json:"icon,omitempty"`
	LaunchURL              string                   `json:"launchUrl,omitempty"`
	AuthenticatedLaunchURL string                   `json:"authenticatedLaunchUrl,omitempty"`
	Category               string                   `json:"category,omitempty"`
	Navigation             string                   `json:"navigation,omitempty"`
	SystemType             string                   `json:"systemType"`
	DisplayInLauncher      bool                     `json:"displayInLauncher"`
	DisplayInSideNav       bool                     `json:"displayInSideNav"`
	LaunchMode             string                   `json:"launchMode"`
	SortOrder              int32                    `json:"sortOrder"`
	AccessRoles            []string                 `json:"accessRoles,omitempty"`
	Enabled                bool                     `json:"enabled"`
	Roles                  []KeycloakDiscoveredRole `json:"roles"`
}

type RbacDriftRole struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type RbacDriftSystem struct {
	ClientID                 string          `json:"clientId"`
	DisplayName              string          `json:"displayName"`
	KeycloakStatus           string          `json:"keycloakStatus"`
	RBACStatus               string          `json:"rbacStatus"`
	MissingRolesInRBAC       []string        `json:"missingRolesInRbac"`
	StaleRolesInRBAC         []string        `json:"staleRolesInRbac"`
	MissingAccessRoles       []string        `json:"missingAccessRoles"`
	DiscoveredRoleDetails    []RbacDriftRole `json:"discoveredRoleDetails"`
	ConfigurationDifferences []string        `json:"configurationDifferences,omitempty"`
}

type RbacDriftRealmRole struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	RBACStatus string `json:"rbacStatus"`
}

type RbacDriftSummary struct {
	SystemsInSync        int `json:"systemsInSync"`
	SystemsMissingInRBAC int `json:"systemsMissingInRbac"`
	SystemsStaleInRBAC   int `json:"systemsStaleInRbac"`
	RolesMissingInRBAC   int `json:"rolesMissingInRbac"`
	RolesStaleInRBAC     int `json:"rolesStaleInRbac"`
	RealmRolesMissing    int `json:"realmRolesMissing"`
}

type RbacDriftReport struct {
	Source     string               `json:"source"`
	Summary    RbacDriftSummary     `json:"summary"`
	Systems    []RbacDriftSystem    `json:"systems"`
	RealmRoles []RbacDriftRealmRole `json:"realmRoles"`
	Warnings   []string             `json:"warnings,omitempty"`
}

type SyncPreviewRequest struct {
	Source      string          `json:"source"`
	RealmExport json.RawMessage `json:"realmExport"`
}

type SyncPreviewResponse struct {
	Report            RbacDriftReport `json:"report"`
	SystemsToCreate   int             `json:"systemsToCreate"`
	SystemsToUpdate   int             `json:"systemsToUpdate"`
	RolesToCreate     int             `json:"rolesToCreate"`
	AccessRolesToAdd  int             `json:"accessRolesToAdd"`
	RealmRolesToTrack int             `json:"realmRolesToTrack"`
}

type SyncApplyRequest struct {
	Source      string          `json:"source"`
	RealmExport json.RawMessage `json:"realmExport"`
}

type SyncApplyResponse struct {
	Preview       SyncPreviewResponse `json:"preview"`
	SystemsSynced int                 `json:"systemsSynced"`
	RolesSynced   int                 `json:"rolesSynced"`
	AccessRoles   int                 `json:"accessRolesSynced"`
	GroupsSynced  int                 `json:"groupsSynced"`
}

type EffectiveAccessUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	FullName   string `json:"fullName"`
	Enabled    bool   `json:"enabled"`
	IsAdmin    bool   `json:"isAdmin"`
	IsResolved bool   `json:"isResolved"`
}

type PermissionGrantSource struct {
	PermissionKey  string `json:"permissionKey"`
	GrantedByType  string `json:"grantedByType"`
	Role           string `json:"role"`
	SystemClientID string `json:"systemClientId,omitempty"`
	SystemName     string `json:"systemName,omitempty"`
	GroupID        string `json:"groupId,omitempty"`
	GroupPath      string `json:"groupPath,omitempty"`
	GroupName      string `json:"groupName,omitempty"`
}

type EffectiveAccessResponse struct {
	User              EffectiveAccessUser     `json:"user"`
	Groups            []GroupSummary          `json:"groups,omitempty"`
	RealmRoles        []string                `json:"realmRoles"`
	ClientRoles       map[string][]string     `json:"clientRoles"`
	Permissions       []Permission            `json:"permissions"`
	AccessibleSystems []SystemAccessSummary   `json:"accessibleSystems"`
	GrantSources      []PermissionGrantSource `json:"grantSources"`
}

type AssignableRealmRole struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Permissions []Permission `json:"permissions"`
}

type AssignableSystemRole struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Permissions []Permission `json:"permissions"`
}

type AssignableSystemAccess struct {
	ClientID          string                 `json:"clientId"`
	DisplayName       string                 `json:"displayName"`
	LaunchURL         string                 `json:"launchUrl,omitempty"`
	Icon              string                 `json:"icon,omitempty"`
	Category          string                 `json:"category,omitempty"`
	SystemType        string                 `json:"systemType"`
	DisplayInLauncher bool                   `json:"displayInLauncher"`
	DisplayInSideNav  bool                   `json:"displayInSideNav"`
	LaunchMode        string                 `json:"launchMode"`
	Roles             []AssignableSystemRole `json:"roles"`
	AccessRoles       []string               `json:"accessRoles"`
}

type AssignableUserAccessResponse struct {
	RealmRoles  []AssignableRealmRole    `json:"realmRoles"`
	Systems     []AssignableSystemAccess `json:"systems"`
	Permissions []Permission             `json:"permissions"`
}

type UserAccessProfileResponse struct {
	EffectiveAccess EffectiveAccessResponse      `json:"effectiveAccess"`
	DirectAccess    DirectUserAccessResponse     `json:"directAccess"`
	Assignable      AssignableUserAccessResponse `json:"assignable"`
}

type DirectUserAccessResponse struct {
	RealmRoles  []string            `json:"realmRoles"`
	ClientRoles map[string][]string `json:"clientRoles"`
	Permissions []Permission        `json:"permissions"`
}

type UpdateUserAccessRequest struct {
	RealmRoles  []string            `json:"realmRoles"`
	ClientRoles map[string][]string `json:"clientRoles"`
	Permissions []string            `json:"permissions,omitempty"`
}

type SystemAccessSummary struct {
	ClientID          string   `json:"clientId"`
	DisplayName       string   `json:"displayName"`
	LaunchURL         string   `json:"launchUrl,omitempty"`
	Icon              string   `json:"icon,omitempty"`
	Category          string   `json:"category,omitempty"`
	Navigation        string   `json:"navigation,omitempty"`
	SystemType        string   `json:"systemType"`
	DisplayInLauncher bool     `json:"displayInLauncher"`
	DisplayInSideNav  bool     `json:"displayInSideNav"`
	LaunchMode        string   `json:"launchMode"`
	SortOrder         int32    `json:"sortOrder"`
	Roles             []string `json:"roles"`
}

type PermissionMetadataInput struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Status      string `json:"status"`
}

type RoleUsageResponse struct {
	Role              SystemRole `json:"role"`
	PermissionsCount  int        `json:"permissionsCount"`
	AssignedUserCount int        `json:"assignedUserCount"`
	Warnings          []string   `json:"warnings"`
}

type OperationResultResponse struct {
	Deleted  bool `json:"deleted,omitempty"`
	Assigned bool `json:"assigned,omitempty"`
	Removed  bool `json:"removed,omitempty"`
	Added    bool `json:"added,omitempty"`
}

type ApiMessageResponse struct {
	Message string `json:"message"`
}

type RealmRoleUsageResponse struct {
	RealmRole        string       `json:"realmRole"`
	Permissions      []Permission `json:"permissions"`
	PermissionsCount int          `json:"permissionsCount"`
	Warnings         []string     `json:"warnings"`
}

type ChangePreviewRequest struct {
	Action         string `json:"action"`
	ResourceType   string `json:"resourceType"`
	ResourceID     string `json:"resourceId"`
	SystemClientID string `json:"systemClientId"`
	RoleName       string `json:"roleName"`
	PermissionKey  string `json:"permissionKey"`
}

type ChangePreviewResponse struct {
	Action             string   `json:"action"`
	ResourceType       string   `json:"resourceType"`
	ResourceID         string   `json:"resourceId"`
	HighRisk           bool     `json:"highRisk"`
	RiskLevel          string   `json:"riskLevel"`
	Warnings           []string `json:"warnings"`
	PermissionsAdded   []string `json:"permissionsAdded"`
	PermissionsRemoved []string `json:"permissionsRemoved"`
	AffectedSystems    []string `json:"affectedSystems"`
	AffectedUsersCount int      `json:"affectedUsersCount"`
}

type ImportPreviewRequest struct {
	Payload json.RawMessage `json:"payload"`
	Format  string          `json:"format"`
	Prune   bool            `json:"prune"`
}

type ImportPreviewResponse struct {
	SystemsToCreate int      `json:"systemsToCreate"`
	SystemsToUpdate int      `json:"systemsToUpdate"`
	RolesToCreate   int      `json:"rolesToCreate"`
	Warnings        []string `json:"warnings"`
}

type ImportApplyResponse struct {
	Preview      ImportPreviewResponse `json:"preview"`
	Applied      bool                  `json:"applied"`
	GroupsSynced int                   `json:"groupsSynced"`
}

type RoleTemplate struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type RoleFromTemplateInput struct {
	TemplateName string `json:"templateName"`
	RoleName     string `json:"roleName"`
	DisplayName  string `json:"displayName"`
}

type CopyPermissionsInput struct {
	SourceRoleID string `json:"sourceRoleId"`
}

type BulkPermissionInput struct {
	RoleIDs       []string `json:"roleIds"`
	PermissionKey string   `json:"permissionKey"`
}

type AuditEvent struct {
	ID             string          `json:"id"`
	ActorUserID    string          `json:"actorUserId,omitempty"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resourceType"`
	ResourceID     string          `json:"resourceId,omitempty"`
	SystemClientID string          `json:"systemClientId,omitempty"`
	RoleName       string          `json:"roleName,omitempty"`
	PermissionKey  string          `json:"permissionKey,omitempty"`
	Details        json.RawMessage `json:"details,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

type AuditFilter struct {
	ActorUserID    string
	SystemClientID string
	RoleName       string
	PermissionKey  string
	Action         string
	From           string
	To             string
	Limit          int
}

type AccessRequestInput struct {
	UserID         string `json:"userId"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	SystemClientID string `json:"systemClientId"`
	RequestedRole  string `json:"requestedRole"`
	Reason         string `json:"reason"`
}

type AccessRequestDecisionInput struct {
	Note string `json:"note"`
}

type AccessRequest struct {
	ID             string `json:"id"`
	UserID         string `json:"userId,omitempty"`
	Username       string `json:"username,omitempty"`
	Email          string `json:"email,omitempty"`
	SystemClientID string `json:"systemClientId"`
	RequestedRole  string `json:"requestedRole"`
	Reason         string `json:"reason,omitempty"`
	Status         string `json:"status"`
	RequestedBy    string `json:"requestedBy,omitempty"`
	ReviewedBy     string `json:"reviewedBy,omitempty"`
	ReviewedAt     string `json:"reviewedAt,omitempty"`
	DecisionNote   string `json:"decisionNote,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ChangeRequestInput struct {
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Payload      json.RawMessage `json:"payload"`
	RiskLevel    string          `json:"riskLevel"`
	Reason       string          `json:"reason"`
}

type ChangeRequest struct {
	ID           string          `json:"id"`
	RequestedBy  string          `json:"requestedBy,omitempty"`
	ReviewedBy   string          `json:"reviewedBy,omitempty"`
	Status       string          `json:"status"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	RiskLevel    string          `json:"riskLevel"`
	Reason       string          `json:"reason,omitempty"`
	DecisionNote string          `json:"decisionNote,omitempty"`
	ReviewedAt   string          `json:"reviewedAt,omitempty"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
}

type SimulationRequest struct {
	UserID            string              `json:"userId"`
	Username          string              `json:"username"`
	Email             string              `json:"email"`
	RealmRoles        []string            `json:"realmRoles"`
	ClientRoles       map[string][]string `json:"clientRoles"`
	AddRealmRoles     []string            `json:"addRealmRoles"`
	RemoveRealmRoles  []string            `json:"removeRealmRoles"`
	AddClientRoles    map[string][]string `json:"addClientRoles"`
	RemoveClientRoles map[string][]string `json:"removeClientRoles"`
	AddPermissions    []string            `json:"addPermissions"`
	RemovePermissions []string            `json:"removePermissions"`
}

type SimulationResponse struct {
	BaselinePermissions []Permission            `json:"baselinePermissions"`
	Permissions         []Permission            `json:"permissions"`
	AddedPermissions    []string                `json:"addedPermissions"`
	RemovedPermissions  []string                `json:"removedPermissions"`
	AccessibleSystems   []SystemAccessSummary   `json:"accessibleSystems"`
	GrantSources        []PermissionGrantSource `json:"grantSources"`
	Warnings            []string                `json:"warnings"`
}
