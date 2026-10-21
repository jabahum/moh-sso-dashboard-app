package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
)

type realmExportFile struct {
	Roles struct {
		Realm []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"realm"`
		Client map[string][]struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"client"`
	} `json:"roles"`
	Clients []struct {
		ClientID               string            `json:"clientId"`
		Name                   string            `json:"name"`
		Description            string            `json:"description"`
		BaseURL                string            `json:"baseUrl"`
		RootURL                string            `json:"rootUrl"`
		Enabled                bool              `json:"enabled"`
		ServiceAccountsEnabled bool              `json:"serviceAccountsEnabled"`
		StandardFlowEnabled    bool              `json:"standardFlowEnabled"`
		Attributes             map[string]string `json:"attributes"`
	} `json:"clients"`
	Groups []realmExportGroup `json:"groups"`
	Users  []realmExportUser  `json:"users"`
}

type realmExportGroup struct {
	Name        string              `json:"name"`
	Path        string              `json:"path"`
	Attributes  map[string][]string `json:"attributes"`
	RealmRoles  []string            `json:"realmRoles"`
	ClientRoles map[string][]string `json:"clientRoles"`
	SubGroups   []realmExportGroup  `json:"subGroups"`
}

type realmExportUser struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
}

type discoveredGroup struct {
	Path        string
	Name        string
	DisplayName string
	Description string
	RealmRoles  []string
	ClientRoles map[string][]string
	Permissions []string
	Members     []GroupMember
}

type discoveredRBAC struct {
	Systems    []KeycloakDiscoveredSystem
	RealmRoles []string
	Groups     []discoveredGroup
}

func (s *Service) DriftFromDefaultRealmExport(ctx context.Context) (RbacDriftReport, error) {
	path, ok := defaultRealmExportPath()
	if !ok {
		return RbacDriftReport{
			Source:     "default-realm-export",
			Systems:    []RbacDriftSystem{},
			RealmRoles: []RbacDriftRealmRole{},
			Warnings:   []string{"No local infra/keycloak/realm-export.json file was found for drift comparison."},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return RbacDriftReport{}, err
	}
	report, err := s.DriftFromRealmExport(ctx, data)
	if err != nil {
		return RbacDriftReport{}, err
	}
	report.Source = path
	return report, nil
}

func (s *Service) DriftFromRealmExport(ctx context.Context, payload []byte) (RbacDriftReport, error) {
	discovered, err := parseRealmExport(payload, s.knownSystemIDs(ctx))
	if err != nil {
		return RbacDriftReport{}, err
	}
	return s.buildDriftReport(ctx, "realm-export", discovered)
}

func (s *Service) PreviewRealmExportSync(ctx context.Context, payload []byte) (SyncPreviewResponse, discoveredRBAC, error) {
	discovered, err := parseRealmExport(payload, s.knownSystemIDs(ctx))
	if err != nil {
		return SyncPreviewResponse{}, discoveredRBAC{}, err
	}
	report, err := s.buildDriftReport(ctx, "realm-export", discovered)
	if err != nil {
		return SyncPreviewResponse{}, discoveredRBAC{}, err
	}

	preview := SyncPreviewResponse{Report: report}
	for _, system := range report.Systems {
		if system.RBACStatus == "missing" {
			preview.SystemsToCreate++
		} else if system.KeycloakStatus == "present" && system.RBACStatus == "present" {
			preview.SystemsToUpdate++
		}
		preview.RolesToCreate += len(system.MissingRolesInRBAC)
		preview.AccessRolesToAdd += len(system.MissingAccessRoles)
	}
	for _, role := range report.RealmRoles {
		if role.RBACStatus == "missing" {
			preview.RealmRolesToTrack++
		}
	}
	return preview, discovered, nil
}

func (s *Service) ApplyRealmExportSync(ctx context.Context, payload []byte) (SyncApplyResponse, error) {
	preview, discovered, err := s.PreviewRealmExportSync(ctx, payload)
	if err != nil {
		return SyncApplyResponse{}, err
	}

	response := SyncApplyResponse{Preview: preview}
	for _, system := range discovered.Systems {
		enabled := system.Enabled
		if _, err := s.UpsertSystem(ctx, UpsertSystemInput{
			ClientID:               system.ClientID,
			DisplayName:            system.DisplayName,
			Description:            system.Description,
			Icon:                   system.Icon,
			LaunchURL:              system.LaunchURL,
			AuthenticatedLaunchURL: system.AuthenticatedLaunchURL,
			Category:               system.Category,
			Navigation:             system.Navigation,
			SystemType:             system.SystemType,
			DisplayInLauncher:      boolPointer(system.DisplayInLauncher),
			DisplayInSideNav:       boolPointer(system.DisplayInSideNav),
			LaunchMode:             system.LaunchMode,
			Enabled:                &enabled,
		}); err != nil {
			return SyncApplyResponse{}, err
		}
		response.SystemsSynced++

		for _, role := range system.Roles {
			systemRole, err := s.repository.UpsertSystemRole(ctx, system.ClientID, RoleInput{
				Name:        role.Name,
				Description: role.Description,
				Enabled:     &enabled,
			})
			if err != nil {
				return SyncApplyResponse{}, err
			}
			if err := s.recordAudit(ctx, "system_role.synced", "system-role", systemRole.ID, system.ClientID, systemRole.Name, "", nil); err != nil {
				return SyncApplyResponse{}, err
			}
			response.RolesSynced++
			if containsNormalized(system.AccessRoles, role.Name) {
				if err := s.repository.AddSystemAccessRole(ctx, system.ClientID, role.Name); err != nil {
					return SyncApplyResponse{}, err
				}
				if err := s.recordAudit(ctx, "system.access_role_synced", "system", system.ClientID, system.ClientID, role.Name, "", nil); err != nil {
					return SyncApplyResponse{}, err
				}
				response.AccessRoles++
			}
		}
	}

	groupsSynced, err := s.applyDiscoveredGroups(ctx, discovered.Groups, "realm-export")
	if err != nil {
		return SyncApplyResponse{}, err
	}
	response.GroupsSynced = groupsSynced

	if err := s.recordAudit(ctx, "rbac.sync_applied", "sync", "realm-export", "", "", "", map[string]any{"systemsSynced": response.SystemsSynced, "rolesSynced": response.RolesSynced, "accessRolesSynced": response.AccessRoles}); err != nil {
		return SyncApplyResponse{}, err
	}
	return response, nil
}

func (s *Service) applyDiscoveredGroups(ctx context.Context, groups []discoveredGroup, source string) (int, error) {
	if len(groups) == 0 {
		return 0, nil
	}

	synced := 0
	for _, discovered := range groups {
		enabled := true
		group, err := s.repository.UpsertGroup(ctx, GroupInput{
			Path:        discovered.Path,
			Name:        discovered.Name,
			DisplayName: firstNonEmpty(discovered.DisplayName, discovered.Name),
			Description: discovered.Description,
			Enabled:     &enabled,
		})
		if err != nil {
			return synced, err
		}

		if err := s.repository.ReplaceGroupMembers(ctx, group.ID, discovered.Members); err != nil {
			return synced, err
		}
		for _, roleName := range discovered.RealmRoles {
			if err := s.repository.AssignGroupRealmRole(ctx, group.ID, roleName); err != nil {
				return synced, err
			}
		}
		for clientID, roles := range discovered.ClientRoles {
			for _, roleName := range roles {
				if err := s.repository.AssignGroupSystemRole(ctx, group.ID, clientID, roleName); err != nil {
					return synced, err
				}
			}
		}
		for _, permissionKey := range discovered.Permissions {
			if err := s.repository.AssignGroupPermission(ctx, group.ID, permissionKey); err != nil {
				return synced, err
			}
		}
		synced++
	}

	if err := s.recordAudit(ctx, "rbac.groups_synced", "sync", source+"-groups", "", "", "", map[string]any{"groups_synced": synced}); err != nil {
		return synced, err
	}
	return synced, nil
}

func (s *Service) buildDriftReport(ctx context.Context, source string, discovered discoveredRBAC) (RbacDriftReport, error) {
	currentSystems, err := s.ListSystems(ctx)
	if err != nil {
		return RbacDriftReport{}, err
	}
	currentRealmRoles, err := s.ListRealmRolePermissions(ctx)
	if err != nil {
		return RbacDriftReport{}, err
	}

	currentByClient := make(map[string]System, len(currentSystems))
	for _, system := range currentSystems {
		currentByClient[system.ClientID] = system
	}
	discoveredByClient := make(map[string]KeycloakDiscoveredSystem, len(discovered.Systems))
	for _, system := range discovered.Systems {
		discoveredByClient[system.ClientID] = system
	}

	report := RbacDriftReport{
		Source:     source,
		Systems:    make([]RbacDriftSystem, 0, len(discovered.Systems)+len(currentSystems)),
		RealmRoles: make([]RbacDriftRealmRole, 0, len(discovered.RealmRoles)),
	}

	seenSystems := map[string]bool{}
	for _, discoveredSystem := range discovered.Systems {
		seenSystems[discoveredSystem.ClientID] = true
		current, exists := currentByClient[discoveredSystem.ClientID]
		drift := RbacDriftSystem{
			ClientID:       discoveredSystem.ClientID,
			DisplayName:    firstNonEmpty(discoveredSystem.DisplayName, discoveredSystem.ClientID),
			KeycloakStatus: "present",
			RBACStatus:     statusForExists(exists),
		}
		if exists {
			drift.DisplayName = firstNonEmpty(current.DisplayName, drift.DisplayName)
			detail, err := s.GetSystem(ctx, discoveredSystem.ClientID)
			if err != nil {
				return RbacDriftReport{}, err
			}
			drift.MissingRolesInRBAC = missingRoleNames(discoveredSystem.Roles, detail.Roles)
			drift.StaleRolesInRBAC = staleRoleNames(discoveredSystem.Roles, detail.Roles)
			drift.MissingAccessRoles = missingConfiguredAccessRoles(discoveredSystem.AccessRoles, detail.AccessRoles)
			drift.ConfigurationDifferences = systemConfigurationDifferences(discoveredSystem, current)
			drift.DiscoveredRoleDetails = roleDriftDetails(discoveredSystem.Roles, detail.Roles)
		} else {
			drift.MissingRolesInRBAC = discoveredRoleNames(discoveredSystem.Roles)
			drift.MissingAccessRoles = sortedStrings(discoveredSystem.AccessRoles)
			drift.DiscoveredRoleDetails = roleDriftDetails(discoveredSystem.Roles, nil)
		}
		updateSummary(&report.Summary, drift)
		report.Systems = append(report.Systems, drift)
	}

	for _, current := range currentSystems {
		if seenSystems[current.ClientID] {
			continue
		}
		drift := RbacDriftSystem{
			ClientID:       current.ClientID,
			DisplayName:    current.DisplayName,
			KeycloakStatus: "missing",
			RBACStatus:     "present",
		}
		updateSummary(&report.Summary, drift)
		report.Systems = append(report.Systems, drift)
	}

	currentRealmRoleSet := map[string]bool{}
	for _, group := range currentRealmRoles {
		currentRealmRoleSet[normalize(group.RealmRole)] = true
	}
	for _, role := range discovered.RealmRoles {
		normalized := normalize(role)
		exists := currentRealmRoleSet[normalized]
		if !exists {
			report.Summary.RealmRolesMissing++
		}
		report.RealmRoles = append(report.RealmRoles, RbacDriftRealmRole{
			Name:       normalized,
			Status:     "present",
			RBACStatus: statusForExists(exists),
		})
	}

	sort.Slice(report.Systems, func(i, j int) bool { return report.Systems[i].ClientID < report.Systems[j].ClientID })
	sort.Slice(report.RealmRoles, func(i, j int) bool { return report.RealmRoles[i].Name < report.RealmRoles[j].Name })
	return report, nil
}

func parseRealmExport(payload []byte, knownSystems ...map[string]bool) (discoveredRBAC, error) {
	var export realmExportFile
	if err := json.Unmarshal(payload, &export); err != nil {
		return discoveredRBAC{}, fmt.Errorf("%w: invalid realm export JSON", ErrInvalidInput)
	}

	discovered := discoveredRBAC{
		Systems:    make([]KeycloakDiscoveredSystem, 0, len(export.Clients)),
		RealmRoles: make([]string, 0, len(export.Roles.Realm)),
		Groups:     make([]discoveredGroup, 0),
	}

	for _, role := range export.Roles.Realm {
		name := normalize(role.Name)
		if shouldSkipRealmRole(name) {
			continue
		}
		discovered.RealmRoles = append(discovered.RealmRoles, name)
	}

	for _, client := range export.Clients {
		clientID := strings.TrimSpace(client.ClientID)
		if shouldSkipClientID(clientID) {
			continue
		}
		known := len(knownSystems) > 0 && knownSystems[0][clientID]
		if !attributeBool(client.Attributes, "portal.system", false) && !known {
			continue
		}
		roles := export.Roles.Client[clientID]
		if len(roles) == 0 && client.ServiceAccountsEnabled && !client.StandardFlowEnabled {
			continue
		}

		behavior := systemrbac.NormalizeSystemBehavior(systemrbac.SeedSystem{
			LaunchURL:              firstNonEmpty(client.Attributes["ui.launchUrl"], client.Attributes["ui.home"], client.BaseURL, client.RootURL),
			AuthenticatedLaunchURL: client.Attributes["ui.authenticatedLaunchUrl"],
			Navigation:             firstNonEmpty(client.Attributes["ui.navigation"], client.Attributes["ui.sidenav"]),
			SystemType:             client.Attributes["ui.systemType"],
			DisplayInLauncher:      boolAttributePointer(client.Attributes, "ui.displayInLauncher"),
			DisplayInSideNav:       boolAttributePointer(client.Attributes, "ui.displayInSideNav"),
			LaunchMode:             client.Attributes["ui.launchMode"],
		})
		system := KeycloakDiscoveredSystem{
			ClientID:               clientID,
			DisplayName:            firstNonEmpty(client.Name, client.Description, clientID),
			Description:            client.Description,
			Icon:                   client.Attributes["ui.icon"],
			LaunchURL:              behavior.LaunchURL,
			AuthenticatedLaunchURL: behavior.AuthenticatedLaunchURL,
			Category:               client.Attributes["ui.category"],
			Navigation:             behavior.Navigation,
			SystemType:             behavior.SystemType,
			DisplayInLauncher:      *behavior.DisplayInLauncher,
			DisplayInSideNav:       *behavior.DisplayInSideNav,
			LaunchMode:             behavior.LaunchMode,
			SortOrder:              int32Attribute(client.Attributes, "ui.order"),
			AccessRoles:            splitAttributeList(client.Attributes["portal.accessRoles"]),
			Enabled:                client.Enabled,
			Roles:                  make([]KeycloakDiscoveredRole, 0, len(roles)),
		}
		for _, role := range roles {
			roleName := normalize(role.Name)
			if roleName == "" {
				continue
			}
			system.Roles = append(system.Roles, KeycloakDiscoveredRole{Name: roleName, Description: role.Description})
		}
		discovered.Systems = append(discovered.Systems, system)
	}

	discovered.Groups = discoverRealmExportGroups(export.Groups, export.Users)

	return discovered, nil
}

func discoverRealmExportGroups(groups []realmExportGroup, users []realmExportUser) []discoveredGroup {
	membersByPath := map[string][]GroupMember{}
	for _, user := range users {
		member := GroupMember{
			UserID:   strings.TrimSpace(user.ID),
			Username: strings.TrimSpace(user.Username),
			Email:    strings.TrimSpace(user.Email),
		}
		for _, groupPath := range user.Groups {
			path := normalizeGroupPath(groupPath)
			if path == "" {
				continue
			}
			membersByPath[path] = append(membersByPath[path], member)
		}
	}

	discovered := make([]discoveredGroup, 0)
	var walk func(values []realmExportGroup, parentPath string)
	walk = func(values []realmExportGroup, parentPath string) {
		for _, group := range values {
			name := strings.TrimSpace(group.Name)
			path := normalizeGroupPath(group.Path)
			if path == "" {
				path = normalizeGroupPath(parentPath + "/" + name)
			}
			if path == "" {
				continue
			}

			realmRoles := make([]string, 0, len(group.RealmRoles))
			for _, role := range group.RealmRoles {
				if normalized := normalize(role); normalized != "" {
					realmRoles = append(realmRoles, normalized)
				}
			}
			sort.Strings(realmRoles)

			clientRoles := map[string][]string{}
			for clientID, roles := range group.ClientRoles {
				clientID = strings.TrimSpace(clientID)
				if clientID == "" {
					continue
				}
				for _, role := range roles {
					if normalized := normalize(role); normalized != "" {
						clientRoles[clientID] = append(clientRoles[clientID], normalized)
					}
				}
				sort.Strings(clientRoles[clientID])
			}

			discovered = append(discovered, discoveredGroup{
				Path:        path,
				Name:        name,
				DisplayName: firstAttributeValue(group.Attributes, "display_name", name),
				Description: firstAttributeValue(group.Attributes, "description", ""),
				RealmRoles:  realmRoles,
				ClientRoles: clientRoles,
				Members:     membersByPath[path],
			})
			walk(group.SubGroups, path)
		}
	}
	walk(groups, "")

	sort.Slice(discovered, func(i, j int) bool { return discovered[i].Path < discovered[j].Path })
	return discovered
}

func discoveredGroupsFromSeed(groups []systemrbac.SeedGroup, memberships []systemrbac.SeedGroupMembership) []discoveredGroup {
	membersByPath := map[string][]GroupMember{}
	for _, membership := range memberships {
		member := GroupMember{
			UserID:   firstNonEmpty(membership.UserID, membership.Username, membership.Email),
			Username: strings.TrimSpace(membership.Username),
			Email:    strings.TrimSpace(membership.Email),
		}
		if member.UserID == "" {
			continue
		}
		for _, groupPath := range membership.Groups {
			path := systemrbac.NormalizeGroupPath(groupPath)
			if path == "" {
				continue
			}
			membersByPath[path] = append(membersByPath[path], member)
		}
	}

	discovered := make([]discoveredGroup, 0)
	for _, group := range systemrbac.FlattenGroups(groups) {
		path := systemrbac.NormalizeGroupPath(group.Path)
		if path == "" {
			continue
		}
		realmRoles := make([]string, 0, len(group.RealmRoles))
		for _, roleName := range group.RealmRoles {
			if normalized := normalize(roleName); normalized != "" {
				realmRoles = append(realmRoles, normalized)
			}
		}
		sort.Strings(realmRoles)

		clientRoles := map[string][]string{}
		for clientID, roles := range group.SystemRoles {
			clientID = strings.TrimSpace(clientID)
			if clientID == "" {
				continue
			}
			for _, roleName := range roles {
				if normalized := normalize(roleName); normalized != "" {
					clientRoles[clientID] = append(clientRoles[clientID], normalized)
				}
			}
			sort.Strings(clientRoles[clientID])
		}

		permissions := make([]string, 0, len(group.Permissions))
		for _, permission := range group.Permissions {
			permission = strings.TrimSpace(permission)
			if permission != "" && permission != "*" {
				permissions = append(permissions, permission)
			}
		}
		sort.Strings(permissions)

		discovered = append(discovered, discoveredGroup{
			Path:        path,
			Name:        strings.TrimSpace(group.Name),
			DisplayName: firstNonEmpty(group.DisplayName, firstAttributeValue(group.Attributes, "display_name", "")),
			Description: firstNonEmpty(group.Description, firstAttributeValue(group.Attributes, "description", "")),
			RealmRoles:  realmRoles,
			ClientRoles: clientRoles,
			Permissions: permissions,
			Members:     membersByPath[path],
		})
	}
	sort.Slice(discovered, func(i, j int) bool { return discovered[i].Path < discovered[j].Path })
	return discovered
}

func normalizeGroupPath(value string) string {
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

func firstAttributeValue(attributes map[string][]string, key string, fallback string) string {
	if len(attributes) == 0 {
		return fallback
	}
	for _, value := range attributes[key] {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fallback
}

func defaultRealmExportPath() (string, bool) {
	for _, path := range []string{
		"../infra/keycloak/realm-export.json",
		"infra/keycloak/realm-export.json",
		"../../infra/keycloak/realm-export.json",
	} {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func shouldSkipClientID(clientID string) bool {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return true
	}
	for _, prefix := range []string{"account", "realm-management", "security-admin-console", "admin-cli"} {
		if strings.HasPrefix(clientID, prefix) {
			return true
		}
	}
	return false
}

func (s *Service) knownSystemIDs(ctx context.Context) map[string]bool {
	known := map[string]bool{}
	if s == nil || s.repository == nil {
		return known
	}
	systems, err := s.repository.ListSystems(ctx)
	if err != nil {
		return known
	}
	for _, system := range systems {
		known[system.ClientID] = true
	}
	return known
}

func attributeBool(attributes map[string]string, key string, fallback bool) bool {
	if attributes == nil {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(attributes[key]))
	if err != nil {
		return fallback
	}
	return value
}

func boolAttributePointer(attributes map[string]string, key string) *bool {
	if attributes == nil || strings.TrimSpace(attributes[key]) == "" {
		return nil
	}
	value := attributeBool(attributes, key, false)
	return &value
}

func int32Attribute(attributes map[string]string, key string) int32 {
	if attributes == nil {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(attributes[key]), 10, 32)
	if err != nil {
		return 0
	}
	return int32(value)
}

func splitAttributeList(value string) []string {
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = normalize(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func containsNormalized(values []string, expected string) bool {
	expected = normalize(expected)
	for _, value := range values {
		if normalize(value) == expected {
			return true
		}
	}
	return false
}

func shouldSkipRealmRole(roleName string) bool {
	roleName = strings.TrimSpace(roleName)
	return roleName == "" || strings.HasPrefix(roleName, "default-roles-") || strings.HasPrefix(roleName, "uma_")
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func statusForExists(exists bool) string {
	if exists {
		return "present"
	}
	return "missing"
}

func discoveredRoleNames(roles []KeycloakDiscoveredRole) []string {
	values := make([]string, 0, len(roles))
	for _, role := range roles {
		values = append(values, role.Name)
	}
	sort.Strings(values)
	return values
}

func missingRoleNames(discovered []KeycloakDiscoveredRole, current []SystemRole) []string {
	currentSet := map[string]bool{}
	for _, role := range current {
		currentSet[normalize(role.Name)] = true
	}
	missing := make([]string, 0)
	for _, role := range discovered {
		if !currentSet[normalize(role.Name)] {
			missing = append(missing, role.Name)
		}
	}
	sort.Strings(missing)
	return missing
}

func staleRoleNames(discovered []KeycloakDiscoveredRole, current []SystemRole) []string {
	discoveredSet := map[string]bool{}
	for _, role := range discovered {
		discoveredSet[normalize(role.Name)] = true
	}
	stale := make([]string, 0)
	for _, role := range current {
		if !discoveredSet[normalize(role.Name)] {
			stale = append(stale, role.Name)
		}
	}
	sort.Strings(stale)
	return stale
}

func missingAccessRoleNames(discovered []KeycloakDiscoveredRole, accessRoles []string) []string {
	accessSet := map[string]bool{}
	for _, role := range accessRoles {
		accessSet[normalize(role)] = true
	}
	missing := make([]string, 0)
	for _, role := range discovered {
		if !accessSet[normalize(role.Name)] {
			missing = append(missing, role.Name)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingConfiguredAccessRoles(discovered []string, accessRoles []string) []string {
	configured := map[string]bool{}
	for _, role := range accessRoles {
		configured[normalize(role)] = true
	}
	missing := make([]string, 0)
	for _, role := range discovered {
		role = normalize(role)
		if role != "" && !configured[role] {
			missing = append(missing, role)
		}
	}
	sort.Strings(missing)
	return missing
}

func systemConfigurationDifferences(discovered KeycloakDiscoveredSystem, current System) []string {
	differences := make([]string, 0)
	if discovered.SystemType != current.SystemType {
		differences = append(differences, "systemType")
	}
	if discovered.DisplayInLauncher != current.DisplayInLauncher {
		differences = append(differences, "displayInLauncher")
	}
	if discovered.DisplayInSideNav != current.DisplayInSideNav {
		differences = append(differences, "displayInSideNav")
	}
	if discovered.LaunchMode != current.LaunchMode {
		differences = append(differences, "launchMode")
	}
	if strings.TrimSpace(discovered.LaunchURL) != strings.TrimSpace(current.LaunchURL) {
		differences = append(differences, "launchUrl")
	}
	if strings.TrimSpace(discovered.AuthenticatedLaunchURL) != strings.TrimSpace(current.AuthenticatedLaunchURL) {
		differences = append(differences, "authenticatedLaunchUrl")
	}
	if strings.TrimSpace(discovered.Navigation) != strings.TrimSpace(current.Navigation) {
		differences = append(differences, "navigation")
	}
	return differences
}

func roleDriftDetails(discovered []KeycloakDiscoveredRole, current []SystemRole) []RbacDriftRole {
	currentSet := map[string]bool{}
	for _, role := range current {
		currentSet[normalize(role.Name)] = true
	}
	values := make([]RbacDriftRole, 0, len(discovered))
	for _, role := range discovered {
		values = append(values, RbacDriftRole{Name: role.Name, Status: statusForExists(currentSet[normalize(role.Name)])})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return values
}

func updateSummary(summary *RbacDriftSummary, drift RbacDriftSystem) {
	if drift.KeycloakStatus == "present" && drift.RBACStatus == "present" &&
		len(drift.MissingRolesInRBAC) == 0 && len(drift.StaleRolesInRBAC) == 0 && len(drift.MissingAccessRoles) == 0 && len(drift.ConfigurationDifferences) == 0 {
		summary.SystemsInSync++
	}
	if drift.RBACStatus == "missing" {
		summary.SystemsMissingInRBAC++
	}
	if drift.KeycloakStatus == "missing" {
		summary.SystemsStaleInRBAC++
	}
	summary.RolesMissingInRBAC += len(drift.MissingRolesInRBAC)
	summary.RolesStaleInRBAC += len(drift.StaleRolesInRBAC)
}
