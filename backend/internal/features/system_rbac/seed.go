package system_rbac

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/moh-sso-dashboard/internal/authz"
	"go.yaml.in/yaml/v3"
)

type SeedFile struct {
	Systems          []SeedSystem          `json:"systems" yaml:"systems"`
	RealmRoles       []SeedRealmRole       `json:"realmRoles" yaml:"realmRoles"`
	Groups           []SeedGroup           `json:"groups,omitempty" yaml:"groups,omitempty"`
	GroupMemberships []SeedGroupMembership `json:"groupMemberships,omitempty" yaml:"groupMemberships,omitempty"`
}

type SeedSystem struct {
	ClientID               string     `json:"clientId" yaml:"clientId"`
	DisplayName            string     `json:"displayName" yaml:"displayName"`
	Description            string     `json:"description,omitempty" yaml:"description,omitempty"`
	Icon                   string     `json:"icon,omitempty" yaml:"icon,omitempty"`
	LaunchURL              string     `json:"launchUrl,omitempty" yaml:"launchUrl,omitempty"`
	AuthenticatedLaunchURL string     `json:"authenticatedLaunchUrl,omitempty" yaml:"authenticatedLaunchUrl,omitempty"`
	Category               string     `json:"category,omitempty" yaml:"category,omitempty"`
	OwnerTeam              string     `json:"ownerTeam,omitempty" yaml:"ownerTeam,omitempty"`
	OwnerName              string     `json:"ownerName,omitempty" yaml:"ownerName,omitempty"`
	OwnerEmail             string     `json:"ownerEmail,omitempty" yaml:"ownerEmail,omitempty"`
	SupportURL             string     `json:"supportUrl,omitempty" yaml:"supportUrl,omitempty"`
	DocumentationURL       string     `json:"documentationUrl,omitempty" yaml:"documentationUrl,omitempty"`
	Environment            string     `json:"environment,omitempty" yaml:"environment,omitempty"`
	Criticality            string     `json:"criticality,omitempty" yaml:"criticality,omitempty"`
	Navigation             string     `json:"navigation,omitempty" yaml:"navigation,omitempty"`
	SystemType             string     `json:"systemType,omitempty" yaml:"systemType,omitempty"`
	DisplayInLauncher      *bool      `json:"displayInLauncher,omitempty" yaml:"displayInLauncher,omitempty"`
	DisplayInSideNav       *bool      `json:"displayInSideNav,omitempty" yaml:"displayInSideNav,omitempty"`
	LaunchMode             string     `json:"launchMode,omitempty" yaml:"launchMode,omitempty"`
	Enabled                *bool      `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	SortOrder              int32      `json:"sortOrder,omitempty" yaml:"sortOrder,omitempty"`
	AccessRoles            []string   `json:"accessRoles,omitempty" yaml:"accessRoles,omitempty"`
	Roles                  []SeedRole `json:"roles,omitempty" yaml:"roles,omitempty"`
}

type NavigationItem struct {
	ID                     string           `json:"id"`
	Label                  string           `json:"label"`
	Path                   string           `json:"path,omitempty"`
	Permission             string           `json:"permission,omitempty"`
	RequiredPermissions    []string         `json:"requiredPermissions,omitempty"`
	RequiredAnyPermissions []string         `json:"requiredAnyPermissions,omitempty"`
	Order                  *int32           `json:"order,omitempty"`
	Icon                   string           `json:"icon,omitempty"`
	Description            string           `json:"description,omitempty"`
	DisplayInLauncher      *bool            `json:"displayInLauncher,omitempty"`
	DisplayInSideNav       *bool            `json:"displayInSideNav,omitempty"`
	LaunchMode             string           `json:"launchMode,omitempty"`
	Children               []NavigationItem `json:"children,omitempty"`
}

type SeedRole struct {
	Name        string   `json:"name" yaml:"name"`
	DisplayName string   `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}

type SeedRealmRole struct {
	Name        string              `json:"name" yaml:"name"`
	Permissions []string            `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	SystemRoles map[string][]string `json:"systemRoles,omitempty" yaml:"systemRoles,omitempty"`
}

type SeedGroup struct {
	Name        string              `json:"name" yaml:"name"`
	Path        string              `json:"path,omitempty" yaml:"path,omitempty"`
	DisplayName string              `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string              `json:"type,omitempty" yaml:"type,omitempty"`
	Protected   bool                `json:"protected,omitempty" yaml:"protected,omitempty"`
	Attributes  map[string][]string `json:"attributes,omitempty" yaml:"attributes,omitempty"`
	RealmRoles  []string            `json:"realmRoles,omitempty" yaml:"realmRoles,omitempty"`
	SystemRoles map[string][]string `json:"systemRoles,omitempty" yaml:"systemRoles,omitempty"`
	Permissions []string            `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Subgroups   []SeedGroup         `json:"subgroups,omitempty" yaml:"subgroups,omitempty"`
	SubGroups   []SeedGroup         `json:"subGroups,omitempty" yaml:"subGroups,omitempty"`
}

type SeedGroupMembership struct {
	UserID   string   `json:"userId,omitempty" yaml:"userId,omitempty"`
	Username string   `json:"username,omitempty" yaml:"username,omitempty"`
	Email    string   `json:"email,omitempty" yaml:"email,omitempty"`
	Groups   []string `json:"groups" yaml:"groups"`
}

const dataStatisticsNavigation = `[
  {"id":"dashboards","label":"Dashboards","path":"/apps/dwh/dashboards","permission":"report_browser:read","icon":"dashboard","description":"Browse approved data and reporting dashboards.","order":10,"displayInLauncher":true},
  {"id":"data-validation","label":"Data Validation","path":"/apps/dwh/data-validation","permission":"data_quality:read","icon":"action","description":"Define and run data quality validation rules.","order":20,"displayInLauncher":true},
  {"id":"data-visualizer","label":"Data Visualizer","path":"/apps/dwh/data-visualizer","permission":"data_quality:read","icon":"chart","order":30,"displayInLauncher":false},
  {"id":"documents","label":"Document Management","path":"/apps/dwh/documents","permission":"documents:read","icon":"documents","description":"Upload and manage health data documents.","order":40,"displayInLauncher":true},
  {"id":"surveillance","label":"Surveillance","path":"/apps/dwh/surveillance","permission":"surveillance:read","icon":"warning-alt","description":"Review surveillance indicators, alerts, and reports.","order":50,"displayInLauncher":true},
  {"id":"issue-tracker","label":"Issue Tracking","path":"/apps/dwh/issue-tracker","permission":"issue_tracker:read","icon":"tracker","description":"Track and resolve data quality issues.","order":60,"displayInLauncher":true},
  {"id":"report-scheduler","label":"Report Scheduler","path":"/apps/dwh/report-scheduler","permission":"report_scheduler:read","icon":"reporting","description":"Schedule report generation and delivery.","order":70,"displayInLauncher":true,"displayInSideNav":true}
]`

const utilitiesNavigation = `[
  {"id":"self-service","label":"Self Service","children":[
    {"id":"my-timesheet","label":"My Timesheet","path":"/apps/utilities/self-service/timesheet"},
    {"id":"elearning","label":"eLearning","path":"/apps/utilities/self-service/elearning"},
    {"id":"leave-plan","label":"Leave Plan","path":"/apps/utilities/self-service/leave-plan"},
    {"id":"absence-requests","label":"Absence Requests","path":"/apps/utilities/self-service/absence-requests"},
    {"id":"absence-dashboard","label":"My Absence Dashboard","path":"/apps/utilities/self-service/absence-dashboard"},
    {"id":"eservice-requests","label":"eService Requests","children":[
      {"id":"document-upload","label":"Document Upload","path":"/apps/utilities/self-service/eservice/document-upload","permission":"documents:write"},
      {"id":"service-access","label":"Service Access","path":"/apps/utilities/self-service/eservice/service-access"},
      {"id":"equipment-request","label":"Equipment Request","path":"/apps/utilities/self-service/eservice/equipment-request"}
    ]}
  ]}
]`
const caseRegistersNavigation = `[
  {"id":"external-referrals","label":"External Referrals","path":"/apps/case-registers/external-referrals"},
  {"id":"disease-registers","label":"Disease Registers","path":"/apps/case-registers/disease-registers"}
]`

const outbreakManagementNavigation = `[
  {"id":"signals-alerts","label":"Signals & Alerts","path":"/apps/outbreak-management/signals-alerts"},
  {"id":"poe-management","label":"PoE Management","path":"/apps/outbreak-management/poe-management"},
  {"id":"case-management","label":"Case Management","path":"/apps/outbreak-management/case-management"}
]`

const referenceRegistersNavigation = `[
  {"id":"facility-register","label":"Facility Register","path":"/apps/reference-registers/facility-register"},
  {"id":"terminology-service","label":"Terminology Service","children":[
    {"id":"test-menu","label":"Test Menu","path":"/apps/reference-registers/terminology/test-menu"},
    {"id":"pharmaceuticals","label":"Pharmaceuticals","path":"/apps/reference-registers/terminology/pharmaceuticals"},
    {"id":"procedures","label":"Procedures","path":"/apps/reference-registers/terminology/procedures"},
    {"id":"equipment","label":"Equipment","path":"/apps/reference-registers/terminology/equipment"}
  ]}
]`

const eServicesNavigation = `[
  {"id":"ihris","label":"iHRIS","path":"/apps/eservices/ihris"},
  {"id":"meeting-manager","label":"Meeting Manager","path":"/apps/eservices/meeting-manager"},
  {"id":"action-tracker","label":"Action Tracker","path":"/apps/eservices/action-tracker"},
  {"id":"clinician-outputs","label":"Clinician Outputs","path":"/apps/eservices/clinician-outputs"},
  {"id":"leave-absence","label":"Leave & Absence Management","path":"/apps/eservices/leave-absence"},
  {"id":"workplans","label":"Workplans","path":"/apps/eservices/workplans"},
  {"id":"budget-tracker","label":"Budget Tracker","path":"/apps/eservices/budget-tracker"},
  {"id":"activity-reporting","label":"Activity Reporting","path":"/apps/eservices/activity-reporting"},
  {"id":"partner-management","label":"Partner Management","path":"/apps/eservices/partner-management"},
  {"id":"observatory-uploads","label":"Observatory Uploads","path":"/apps/eservices/observatory-uploads"}
]`

const researchStudiesNavigation = `[
  {"id":"studies","label":"Studies","path":"/apps/research-studies/studies"},
  {"id":"datasets","label":"Datasets","path":"/apps/research-studies/datasets"},
  {"id":"ethics-approvals","label":"Ethics & Approvals","path":"/apps/research-studies/ethics"},
  {"id":"publications","label":"Publications","path":"/apps/research-studies/publications"}
]`

func LoadSeedFile(path string) (SeedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SeedFile{}, err
	}

	var seed SeedFile
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		err = json.Unmarshal(data, &seed)
	default:
		err = yaml.Unmarshal(data, &seed)
	}
	if err != nil {
		return SeedFile{}, err
	}

	return seed, nil
}

func WriteSeedFile(path string, seed SeedFile) error {
	data, err := yaml.Marshal(seed)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func ValidateSeed(seed SeedFile) error {
	knownPermissions := map[string]bool{"*": true}
	for _, permission := range authz.AllPermissions {
		knownPermissions[string(permission)] = true
	}

	seenSystems := map[string]bool{}
	systemRoles := map[string]map[string]bool{}
	systemAccessRoles := map[string]map[string]bool{}
	for _, system := range seed.Systems {
		system = NormalizeSystemBehavior(system)
		clientID := strings.ToLower(strings.TrimSpace(system.ClientID))
		if clientID == "" {
			return fmt.Errorf("system clientId is required")
		}
		if seenSystems[clientID] {
			return fmt.Errorf("duplicate system clientId %q", clientID)
		}
		seenSystems[clientID] = true

		if strings.TrimSpace(system.DisplayName) == "" {
			return fmt.Errorf("system %q displayName is required", clientID)
		}
		if err := ValidateSystemBehavior(system); err != nil {
			return fmt.Errorf("system %q: %w", clientID, err)
		}

		seenRoles := map[string]bool{}
		for _, role := range system.Roles {
			roleName := strings.ToLower(strings.TrimSpace(role.Name))
			if roleName == "" {
				return fmt.Errorf("system %q has a role without a name", clientID)
			}
			if seenRoles[roleName] {
				return fmt.Errorf("system %q has duplicate role %q", clientID, roleName)
			}
			seenRoles[roleName] = true
			if err := validatePermissions(knownPermissions, role.Permissions); err != nil {
				return fmt.Errorf("system %q role %q: %w", clientID, roleName, err)
			}
		}
		systemRoles[clientID] = seenRoles
		accessRoles := map[string]bool{}
		for _, roleName := range system.AccessRoles {
			accessRoles[strings.ToLower(strings.TrimSpace(roleName))] = true
		}
		systemAccessRoles[clientID] = accessRoles
	}

	seenRealmRoles := map[string]bool{}
	for _, role := range seed.RealmRoles {
		roleName := strings.TrimSpace(role.Name)
		if roleName == "" {
			return fmt.Errorf("realm role name is required")
		}
		if seenRealmRoles[roleName] {
			return fmt.Errorf("duplicate realm role %q", roleName)
		}
		seenRealmRoles[roleName] = true
		if err := validatePermissions(knownPermissions, role.Permissions); err != nil {
			return fmt.Errorf("realm role %q: %w", roleName, err)
		}
		for clientID, roles := range role.SystemRoles {
			clientID = strings.ToLower(strings.TrimSpace(clientID))
			if !seenSystems[clientID] {
				return fmt.Errorf("realm role %q references unknown system %q", roleName, clientID)
			}
			for _, systemRole := range roles {
				systemRole = strings.ToLower(strings.TrimSpace(systemRole))
				if !systemRoles[clientID][systemRole] {
					return fmt.Errorf("realm role %q references unknown role %q for system %q", roleName, systemRole, clientID)
				}
				if !systemAccessRoles[clientID][systemRole] {
					return fmt.Errorf("realm role %q default role %q is not an access role for system %q", roleName, systemRole, clientID)
				}
			}
		}
	}

	seenGroups := map[string]bool{}
	for _, group := range FlattenGroups(seed.Groups) {
		groupName := strings.TrimSpace(group.Name)
		groupPath := NormalizeGroupPath(firstNonEmpty(group.Path, groupName))
		if groupName == "" {
			return fmt.Errorf("group name is required")
		}
		if groupPath == "" {
			return fmt.Errorf("group %q path is required", groupName)
		}
		if seenGroups[groupPath] {
			return fmt.Errorf("duplicate group path %q", groupPath)
		}
		seenGroups[groupPath] = true
		if err := validatePermissions(knownPermissions, group.Permissions); err != nil {
			return fmt.Errorf("group %q: %w", groupPath, err)
		}
		for clientID, roles := range group.SystemRoles {
			clientID = strings.ToLower(strings.TrimSpace(clientID))
			if !seenSystems[clientID] {
				return fmt.Errorf("group %q references unknown system %q", groupPath, clientID)
			}
			for _, systemRole := range roles {
				systemRole = strings.ToLower(strings.TrimSpace(systemRole))
				if !systemRoles[clientID][systemRole] {
					return fmt.Errorf("group %q references unknown role %q for system %q", groupPath, systemRole, clientID)
				}
			}
		}
	}

	for _, membership := range seed.GroupMemberships {
		if strings.TrimSpace(membership.UserID) == "" &&
			strings.TrimSpace(membership.Username) == "" &&
			strings.TrimSpace(membership.Email) == "" {
			return fmt.Errorf("group membership requires userId, username, or email")
		}
		for _, groupPath := range membership.Groups {
			groupPath = NormalizeGroupPath(groupPath)
			if groupPath == "" {
				return fmt.Errorf("group membership references empty group path")
			}
			if !seenGroups[groupPath] {
				return fmt.Errorf("group membership references unknown group %q", groupPath)
			}
		}
	}

	return nil
}

func FlattenGroups(groups []SeedGroup) []SeedGroup {
	flattened := make([]SeedGroup, 0)
	var walk func(values []SeedGroup, parentPath string)
	walk = func(values []SeedGroup, parentPath string) {
		for _, group := range values {
			name := strings.TrimSpace(group.Name)
			group.Path = NormalizeGroupPath(firstNonEmpty(group.Path, parentPath+"/"+name))
			flattened = append(flattened, group)
			walk(group.Subgroups, group.Path)
			walk(group.SubGroups, group.Path)
		}
	}
	walk(groups, "")
	return flattened
}

func NormalizeGroupPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '/' })
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return "/" + strings.Join(cleaned, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func validatePermissions(knownPermissions map[string]bool, permissions []string) error {
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			return fmt.Errorf("permission cannot be empty")
		}
		if !knownPermissions[permission] {
			return fmt.Errorf("unknown permission %q", permission)
		}
	}

	return nil
}

func NormalizeSystemBehavior(system SeedSystem) SeedSystem {
	system.SystemType = strings.ToLower(strings.TrimSpace(system.SystemType))
	system.LaunchMode = strings.ToLower(strings.TrimSpace(system.LaunchMode))
	system.LaunchURL = strings.TrimSpace(system.LaunchURL)
	system.AuthenticatedLaunchURL = strings.TrimSpace(system.AuthenticatedLaunchURL)
	system.Navigation = strings.TrimSpace(system.Navigation)

	if system.SystemType == "" {
		if isHTTPURL(system.LaunchURL) && system.Navigation == "" {
			system.SystemType = "external"
		} else {
			system.SystemType = "platform"
		}
	}
	if system.LaunchMode == "" {
		if system.SystemType == "external" {
			system.LaunchMode = "new_tab"
		} else {
			system.LaunchMode = "internal"
		}
	}
	if system.DisplayInLauncher == nil {
		value := true
		system.DisplayInLauncher = &value
	}
	if system.DisplayInSideNav == nil {
		value := system.SystemType == "platform" && system.Navigation != ""
		system.DisplayInSideNav = &value
	}
	return system
}

func ValidateSystemBehavior(system SeedSystem) error {
	system = NormalizeSystemBehavior(system)
	if system.SystemType != "platform" && system.SystemType != "external" {
		return fmt.Errorf("systemType must be platform or external")
	}
	if system.LaunchMode != "internal" && system.LaunchMode != "new_tab" && system.LaunchMode != "same_tab" {
		return fmt.Errorf("launchMode must be internal, new_tab, or same_tab")
	}
	if system.SystemType == "platform" {
		if system.LaunchMode != "internal" {
			return fmt.Errorf("platform systems must use internal launchMode")
		}
		if system.LaunchURL != "" && !isPortalPath(system.LaunchURL) {
			return fmt.Errorf("platform launchUrl must begin with /portal or /apps")
		}
		if system.AuthenticatedLaunchURL != "" && !isAuthLaunchURL(system.AuthenticatedLaunchURL) {
			return fmt.Errorf("authenticatedLaunchUrl must be an /api/v1/auth/launch URL or an absolute HTTP or HTTPS URL")
		}
	} else {
		if system.LaunchMode == "internal" {
			return fmt.Errorf("external systems must use new_tab or same_tab launchMode")
		}
		if !isHTTPURL(system.LaunchURL) {
			return fmt.Errorf("external launchUrl must be an absolute HTTP or HTTPS URL")
		}
		if system.Navigation != "" || (system.DisplayInSideNav != nil && *system.DisplayInSideNav) {
			return fmt.Errorf("external systems cannot define portal side navigation")
		}
	}
	items, err := parseNavigation(system.Navigation)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		if system.SystemType != "platform" {
			return fmt.Errorf("navigation requires a platform system")
		}
		if err := validateNavigationItems(items, map[string]struct{}{}); err != nil {
			return err
		}
	}
	if system.DisplayInSideNav != nil && *system.DisplayInSideNav {
		if system.SystemType != "platform" {
			return fmt.Errorf("side navigation requires a platform system")
		}
		if len(items) == 0 {
			return fmt.Errorf("side navigation requires a non-empty navigation JSON array")
		}
	}
	return nil
}

func parseNavigation(raw string) ([]NavigationItem, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var items []NavigationItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("navigation must be a valid JSON array: %w", err)
	}
	if items == nil {
		return nil, fmt.Errorf("navigation must be a JSON array")
	}
	return items, nil
}

func validateNavigationItems(items []NavigationItem, seen map[string]struct{}) error {
	for index, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Label = strings.TrimSpace(item.Label)
		item.Path = strings.TrimSpace(item.Path)
		item.LaunchMode = strings.ToLower(strings.TrimSpace(item.LaunchMode))

		if item.ID == "" {
			return fmt.Errorf("navigation item %d has an empty id", index)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("navigation item id %q is duplicated", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Label == "" {
			return fmt.Errorf("navigation item %q has an empty label", item.ID)
		}
		if strings.TrimSpace(item.Permission) == "" && item.Permission != "" {
			return fmt.Errorf("navigation item %q has an empty permission", item.ID)
		}
		if err := validateNavigationPermissions(item.ID, "requiredPermissions", item.RequiredPermissions); err != nil {
			return err
		}
		if err := validateNavigationPermissions(item.ID, "requiredAnyPermissions", item.RequiredAnyPermissions); err != nil {
			return err
		}
		if item.LaunchMode != "" && item.LaunchMode != "internal" && item.LaunchMode != "new_tab" && item.LaunchMode != "same_tab" {
			return fmt.Errorf("navigation item %q has invalid launchMode %q", item.ID, item.LaunchMode)
		}
		if item.DisplayInLauncher != nil && *item.DisplayInLauncher && item.Path == "" {
			return fmt.Errorf("launcher navigation item %q requires a path", item.ID)
		}
		if item.Path != "" {
			external := isHTTPURL(item.Path)
			if !external && !isPortalPath(item.Path) {
				return fmt.Errorf("navigation item %q path must begin with /portal or /apps, or be an absolute HTTP or HTTPS URL", item.ID)
			}
			if external && item.LaunchMode != "new_tab" && item.LaunchMode != "same_tab" {
				return fmt.Errorf("external navigation item %q must use new_tab or same_tab launchMode", item.ID)
			}
			if !external && item.LaunchMode != "" && item.LaunchMode != "internal" {
				return fmt.Errorf("internal navigation item %q must use internal launchMode", item.ID)
			}
		}
		if err := validateNavigationItems(item.Children, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateNavigationPermissions(itemID, field string, permissions []string) error {
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == "" {
			return fmt.Errorf("navigation item %q has an empty %s value", itemID, field)
		}
	}
	return nil
}

func isPortalPath(value string) bool {
	return value == "/portal" || strings.HasPrefix(value, "/portal/") || value == "/apps" || strings.HasPrefix(value, "/apps/")
}

func isHTTPURL(value string) bool {
	if strings.HasPrefix(value, "//") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func isAuthLaunchURL(value string) bool {
	if strings.HasPrefix(value, "/api/v1/auth/launch") {
		return true
	}
	return isHTTPURL(value)
}

func DefaultSeed() SeedFile {
	enabled := true

	seed := SeedFile{
		Systems: []SeedSystem{
			{
				ClientID:               authz.SystemDashboardWeb,
				DisplayName:            "Integrated Health Portal",
				Description:            "Central portal shell",
				Icon:                   "dashboard",
				LaunchURL:              "/portal",
				AuthenticatedLaunchURL: "/api/v1/auth/launch?returnTo=%2Fportal%2Fapps%2Fnews",
				Category:               "platform",
				Enabled:                &enabled,
				AccessRoles:            []string{authz.DashboardWebAccess},
				Roles: []SeedRole{
					{
						Name:        authz.DashboardWebAccess,
						DisplayName: "Dashboard Access",
						Permissions: []string{
							string(authz.PermissionPortalAccess),
							string(authz.PermissionSystemsRead),
							string(authz.PermissionSystemsLaunch),
						},
					},
					{
						Name:        authz.DashboardWebAdmin,
						DisplayName: "Portal Admin",
						Permissions: []string{
							"*",
						},
					},
					{
						Name:        authz.DashboardWebManager,
						DisplayName: "Portal Manager",
						Permissions: []string{
							string(authz.PermissionPortalAccess),
							string(authz.PermissionSystemsRead),
							string(authz.PermissionSystemsLaunch),
							string(authz.PermissionUsersRead),
							string(authz.PermissionClientsRead),
							string(authz.PermissionAnnouncementsRead),
							string(authz.PermissionDocumentsRead),
							string(authz.PermissionDocumentTemplatesRead),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionReportBrowserRead),
							string(authz.PermissionNotificationsRead),
						},
					},
					{
						Name:        authz.DashboardWebUser,
						DisplayName: "Portal User",
						Permissions: []string{
							string(authz.PermissionPortalAccess),
							string(authz.PermissionSystemsRead),
							string(authz.PermissionSystemsLaunch),
						},
					},
				},
			},
			{
				ClientID:    authz.SystemOutbreakManagement,
				DisplayName: "Outbreak Management",
				Description: "Signals, alerts, surveillance, points of entry, and case management.",
				Icon:        "warning-alt",
				LaunchURL:   "/portal/apps/outbreak-management",
				Category:    "surveillance",
				Navigation:  outbreakManagementNavigation,
				Enabled:     &enabled,
				AccessRoles: []string{
					authz.OutbreakManagementAccess,
					authz.OutbreakManagementViewer,
					authz.OutbreakManagementDataEntry,
					authz.OutbreakManagementSurveillanceOfficer,
					authz.OutbreakManagementManager,
					authz.OutbreakManagementAdmin,
					authz.OutbreakManagementSuperAdmin,
				},
				Roles: []SeedRole{
					{
						Name:        authz.OutbreakManagementAccess,
						DisplayName: "Outbreak Management Access",
						Permissions: []string{
							string(authz.PermissionPortalAccess),
							string(authz.PermissionSystemsRead),
							string(authz.PermissionSystemsLaunch),
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
						},
					},
					{
						Name: authz.OutbreakManagementSuperAdmin,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionOutbreakManage),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionSurveillanceImport),
							string(authz.PermissionSurveillanceManageLocations),
							string(authz.PermissionSurveillanceManageAlerts),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDataQualityWrite),
							string(authz.PermissionDataQualityResolve),
							string(authz.PermissionDocumentsRead),
							string(authz.PermissionDocumentsWrite),
							string(authz.PermissionDocumentsProcess),
						},
					},
					{
						Name: authz.OutbreakManagementAdmin,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionOutbreakManage),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionSurveillanceImport),
							string(authz.PermissionSurveillanceManageLocations),
							string(authz.PermissionSurveillanceManageAlerts),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDataQualityWrite),
							string(authz.PermissionDataQualityResolve),
						},
					},
					{
						Name: authz.OutbreakManagementManager,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDataQualityWrite),
							string(authz.PermissionDocumentsRead),
						},
					},
					{
						Name: authz.OutbreakManagementViewer,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDocumentsRead),
						},
					},
					{
						Name: authz.OutbreakManagementDataEntry,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionSurveillanceImport),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDataQualityWrite),
						},
					},
					{
						Name: authz.OutbreakManagementLabTechnician,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionDataQualityRead),
							string(authz.PermissionDataQualityWrite),
						},
					},
					{
						Name: authz.OutbreakManagementSurveillanceOfficer,
						Permissions: []string{
							string(authz.PermissionOutbreakAccess),
							string(authz.PermissionSurveillanceRead),
							string(authz.PermissionSurveillanceImport),
							string(authz.PermissionSurveillanceManageAlerts),
						},
					},
				},
			},
			defaultDataStatisticsSystem(enabled),
			defaultPortalSystem("eservices", "eServices", "Digital service requests and operational tools.", "application", "/portal/apps/eservices", "services", eServicesNavigation, nil, enabled),
			defaultPortalSystem("research-studies", "Research & Studies", "Research studies, datasets, ethics approvals, and publications.", "microscope", "/portal/apps/research-studies", "research", researchStudiesNavigation, nil, enabled),
			defaultPortalSystem("case-registers", "Case Registers", "Case registers and referral tracking.", "document", "/portal/apps/case-registers", "clinical", caseRegistersNavigation, []string{
				string(authz.PermissionDocumentsRead),
			}, enabled),
			defaultPortalSystem("reference-registers", "Reference Registers", "Facility, terminology, and reference data registers.", "catalog", "/portal/apps/reference-registers", "registry", referenceRegistersNavigation, []string{
				string(authz.PermissionSystemsRead),
			}, enabled),
			defaultPortalSystem(authz.SystemUtilities, "Utilities", "Self-service utilities and staff tools.", "tools", "/portal/apps/utilities/self-service", "utilities", utilitiesNavigation, nil, enabled),
			defaultSettingsSystem(enabled),
		},
		RealmRoles: []SeedRealmRole{
			{Name: authz.RoleAdmin, Permissions: []string{"*"}},
			{
				Name: authz.RoleUser,
				Permissions: []string{
					string(authz.PermissionPortalAccess),
					string(authz.PermissionSystemsRead),
					string(authz.PermissionSystemsLaunch),
					string(authz.PermissionDataQualityRead),
					string(authz.PermissionIssueTrackerRead),
					string(authz.PermissionDocumentsRead),
					string(authz.PermissionDocumentTemplatesRead),
					string(authz.PermissionStorageLocationsRead),
					string(authz.PermissionSurveillanceRead),
					string(authz.PermissionReportBrowserRead),
				},
				SystemRoles: map[string][]string{
					authz.SystemDataStatistics: {authz.DataStatisticsAccess, authz.DocumentViewer},
					authz.SystemUtilities:      {authz.UtilitiesAccess},
					authz.SystemSettings:       {authz.SettingsAccess},
				},
			},
			{
				Name: authz.RoleManager,
				Permissions: []string{
					string(authz.PermissionPortalAccess),
					string(authz.PermissionSystemsRead),
					string(authz.PermissionSystemsLaunch),
					string(authz.PermissionUsersRead),
					string(authz.PermissionClientsRead),
					string(authz.PermissionAnnouncementsRead),
					string(authz.PermissionDocumentsRead),
					string(authz.PermissionDocumentsWrite),
					string(authz.PermissionDocumentsProcess),
					string(authz.PermissionDocumentTemplatesRead),
					string(authz.PermissionDocumentTemplatesWrite),
					string(authz.PermissionDocumentTemplatesPublish),
					string(authz.PermissionStorageLocationsRead),
					string(authz.PermissionSurveillanceRead),
					string(authz.PermissionDataQualityRead),
					string(authz.PermissionDataQualityWrite),
					string(authz.PermissionIssueTrackerRead),
					string(authz.PermissionIssueTrackerWrite),
					string(authz.PermissionIssueTrackerManage),
					string(authz.PermissionIssueTrackerAssign),
					string(authz.PermissionIssueTrackerClose),
					string(authz.PermissionIssueTrackerReopen),
					string(authz.PermissionIssueTrackerComment),
					string(authz.PermissionNotificationsRead),
				},
				SystemRoles: map[string][]string{
					authz.SystemDataStatistics: {
						authz.DataStatisticsAccess,
						authz.SurveillanceManager,
						authz.ReportBrowserManager,
						authz.IssueTrackerManager,
						authz.DocumentManager,
					},
					authz.SystemUtilities: {authz.UtilitiesAccess},
					authz.SystemSettings:  {authz.SettingsAccess},
				},
			},
		},
	}
	applyDefaultSortOrder(seed.Systems)
	for index := range seed.Systems {
		seed.Systems[index] = NormalizeSystemBehavior(seed.Systems[index])
	}
	return seed
}

func applyDefaultSortOrder(systems []SeedSystem) {
	sortOrders := map[string]int32{
		authz.SystemDashboardWeb:    0,
		"outbreak-management":       10,
		authz.SystemDataStatistics:  20,
		"eservices":                 30,
		"research-studies":          40,
		"case-registers":            50,
		"reference-registers":       60,
		authz.SystemUtilities:       90,
		"settings":                  100,
		"demo-platform-system":      900,
		"external-knowledge-system": 910,
		"external-lab-portal":       920,
	}

	for index := range systems {
		if systems[index].SortOrder != 0 {
			continue
		}
		if sortOrder, ok := sortOrders[systems[index].ClientID]; ok {
			systems[index].SortOrder = sortOrder
		}
	}
}

func defaultPortalSystem(
	clientID string,
	displayName string,
	description string,
	icon string,
	launchURL string,
	category string,
	navigation string,
	extraPermissions []string,
	enabled bool,
) SeedSystem {
	accessRole := clientID + "_access"
	permissions := []string{
		string(authz.PermissionPortalAccess),
		string(authz.PermissionSystemsRead),
		string(authz.PermissionSystemsLaunch),
	}
	permissions = append(permissions, extraPermissions...)

	return SeedSystem{
		ClientID:    clientID,
		DisplayName: displayName,
		Description: description,
		Icon:        icon,
		LaunchURL:   launchURL,
		Category:    category,
		Navigation:  navigation,
		Enabled:     &enabled,
		AccessRoles: []string{accessRole},
		Roles: []SeedRole{
			{
				Name:        accessRole,
				DisplayName: displayName + " Access",
				Permissions: permissions,
			},
		},
	}
}

func defaultSettingsSystem(enabled bool) SeedSystem {
	system := defaultPortalSystem(
		"settings",
		"Settings",
		"User profile, session, and security settings.",
		"settings",
		"/portal/apps/settings",
		"platform",
		"[]",
		nil,
		enabled,
	)
	displayInLauncher := false
	displayInSideNav := false
	system.DisplayInLauncher = &displayInLauncher
	system.DisplayInSideNav = &displayInSideNav
	return system
}

func defaultDataStatisticsSystem(enabled bool) SeedSystem {
	system := defaultPortalSystem(
		authz.SystemDataStatistics,
		"Data & Statistics",
		"Data quality, document upload, reports, and surveillance tools.",
		"home",
		"/portal/apps/dwh",
		"platform",
		dataStatisticsNavigation,
		[]string{
			string(authz.PermissionDataQualityRead),
			string(authz.PermissionDocumentsRead),
			string(authz.PermissionSurveillanceRead),
			string(authz.PermissionReportBrowserRead),
		},
		enabled,
	)
	system.AccessRoles = append(system.AccessRoles,
		authz.ReportSchedulerAccess,
		authz.ReportBrowserViewer,
		authz.ReportBrowserAnalyst,
		authz.ReportBrowserManager,
		authz.ReportBrowserAdmin,
		authz.SurveillanceViewer,
		authz.SurveillanceOfficer,
		authz.SurveillanceDataEntry,
		authz.SurveillanceManager,
		authz.IssueTrackerViewer,
		authz.IssueTrackerContributor,
		authz.IssueTrackerEditor,
		authz.IssueTrackerManager,
		authz.DocumentViewer,
		authz.DocumentEditor,
		authz.DocumentProcessor,
		authz.DocumentManager,
		authz.DocumentTemplateViewer,
		authz.DocumentTemplateEditor,
		authz.DocumentTemplatePublisher,
	)
	system.Roles = append(system.Roles,
			SeedRole{
				Name:        authz.ReportSchedulerAccess,
				DisplayName: "Report Scheduler Access",
				Permissions: []string{
					string(authz.PermissionPortalAccess), string(authz.PermissionSystemsRead), string(authz.PermissionSystemsLaunch),
					string(authz.PermissionReportSchedulerRead), string(authz.PermissionReportSchedulerCreate),
					string(authz.PermissionReportSchedulerUpdate), string(authz.PermissionReportSchedulerDelete),
					string(authz.PermissionReportSchedulerExecute), string(authz.PermissionReportSchedulerHistory),
				},
		},
		SeedRole{
			Name:        authz.ReportBrowserAdmin,
			DisplayName: "Report Admin",
				Permissions: []string{
					string(authz.PermissionReportBrowserRead),
					string(authz.PermissionMetricsRead),
					string(authz.PermissionAuditRead),
					string(authz.PermissionReportSchedulerRead),
					string(authz.PermissionReportSchedulerCreate),
					string(authz.PermissionReportSchedulerUpdate),
					string(authz.PermissionReportSchedulerDelete),
					string(authz.PermissionReportSchedulerExecute),
					string(authz.PermissionReportSchedulerHistory),
					string(authz.PermissionReportSchedulerManage),
				},
		},
		SeedRole{
			Name:        authz.ReportBrowserManager,
			DisplayName: "Report Manager",
			Permissions: []string{
				string(authz.PermissionReportBrowserRead),
				string(authz.PermissionMetricsRead),
			},
		},
		SeedRole{
			Name:        authz.ReportBrowserAnalyst,
			DisplayName: "Report Analyst",
			Permissions: []string{
				string(authz.PermissionReportBrowserRead),
				string(authz.PermissionDataQualityRead),
			},
		},
		SeedRole{
			Name:        authz.ReportBrowserViewer,
			DisplayName: "Report Viewer",
			Permissions: []string{
				string(authz.PermissionReportBrowserRead),
			},
		},
		SeedRole{
			Name:        authz.SurveillanceViewer,
			DisplayName: "Surveillance Viewer",
			Permissions: []string{
				string(authz.PermissionSurveillanceRead),
			},
		},
		SeedRole{
			Name:        authz.SurveillanceOfficer,
			DisplayName: "Surveillance Officer",
			Permissions: []string{
				string(authz.PermissionSurveillanceRead),
				string(authz.PermissionSurveillanceImport),
				string(authz.PermissionSurveillanceManageAlerts),
			},
		},
		SeedRole{
			Name:        authz.SurveillanceDataEntry,
			DisplayName: "Surveillance Data Entry",
			Permissions: []string{
				string(authz.PermissionSurveillanceRead),
				string(authz.PermissionSurveillanceImport),
			},
		},
		SeedRole{
			Name:        authz.SurveillanceManager,
			DisplayName: "Surveillance Manager",
			Permissions: []string{
				string(authz.PermissionSurveillanceRead),
				string(authz.PermissionSurveillanceImport),
				string(authz.PermissionSurveillanceManageLocations),
				string(authz.PermissionSurveillanceManageAlerts),
			},
		},
		SeedRole{
			Name:        authz.IssueTrackerViewer,
			DisplayName: "Issue Tracker Viewer",
			Permissions: []string{
				string(authz.PermissionIssueTrackerRead),
			},
		},
		SeedRole{
			Name:        authz.IssueTrackerContributor,
			DisplayName: "Issue Tracker Contributor",
			Permissions: []string{
				string(authz.PermissionIssueTrackerRead),
				string(authz.PermissionIssueTrackerComment),
			},
		},
		SeedRole{
			Name:        authz.IssueTrackerEditor,
			DisplayName: "Issue Tracker Editor",
			Permissions: []string{
				string(authz.PermissionIssueTrackerRead),
				string(authz.PermissionIssueTrackerWrite),
				string(authz.PermissionIssueTrackerComment),
			},
		},
		SeedRole{
			Name:        authz.IssueTrackerManager,
			DisplayName: "Issue Tracker Manager",
			Permissions: []string{
				string(authz.PermissionIssueTrackerRead),
				string(authz.PermissionIssueTrackerWrite),
				string(authz.PermissionIssueTrackerManage),
				string(authz.PermissionIssueTrackerAssign),
				string(authz.PermissionIssueTrackerClose),
				string(authz.PermissionIssueTrackerReopen),
				string(authz.PermissionIssueTrackerComment),
			},
		},
		SeedRole{
			Name:        authz.DocumentViewer,
			DisplayName: "Document Viewer",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentEditor,
			DisplayName: "Document Editor",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentsWrite),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentProcessor,
			DisplayName: "Document Processor",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentsProcess),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentManager,
			DisplayName: "Document Manager",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentsWrite),
				string(authz.PermissionDocumentsProcess),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionDocumentTemplatesWrite),
				string(authz.PermissionDocumentTemplatesPublish),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentTemplateViewer,
			DisplayName: "Document Template Viewer",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentTemplateEditor,
			DisplayName: "Document Template Editor",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionDocumentTemplatesWrite),
				string(authz.PermissionStorageLocationsRead),
			},
		},
		SeedRole{
			Name:        authz.DocumentTemplatePublisher,
			DisplayName: "Document Template Publisher",
			Permissions: []string{
				string(authz.PermissionDocumentsRead),
				string(authz.PermissionDocumentTemplatesRead),
				string(authz.PermissionDocumentTemplatesWrite),
				string(authz.PermissionDocumentTemplatesPublish),
				string(authz.PermissionStorageLocationsRead),
			},
		},
	)
	return system
}
