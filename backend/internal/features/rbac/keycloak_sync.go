package rbac

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
)

type KeycloakSyncSource interface {
	ListClients() ([]keycloak.ClientInfo, error)
	ListRealmRoles(ctx context.Context) ([]keycloak.RoleRep, error)
	ListClientRoles(ctx context.Context, clientID string) ([]keycloak.ClientRoleRep, error)
	CreateClient(opts keycloak.CreateClientParams) (string, error)
	CreateRealmRole(ctx context.Context, roleName string, description string) error
	CreateClientRole(ctx context.Context, clientID string, req *model.CreateClientRoleRequest) error
	EnsureRealmRoleClientRoleComposite(ctx context.Context, realmRole string, clientID string, roleName string) error
}

type KeycloakGroupSyncSource interface {
	ListGroups(ctx context.Context) ([]keycloak.GroupRep, error)
	ListGroupMembers(ctx context.Context, groupID string) ([]keycloak.KeycloakUser, error)
	ListGroupRealmRoles(ctx context.Context, groupID string) ([]keycloak.RoleRep, error)
	ListGroupClientRoles(ctx context.Context, groupID string, clientID string) ([]keycloak.ClientRoleRep, error)
}

type KeycloakPushResult struct {
	ClientsCreated     int      `json:"clientsCreated"`
	RealmRolesCreated  int      `json:"realmRolesCreated"`
	ClientRolesCreated int      `json:"clientRolesCreated"`
	CompositesSynced   int      `json:"compositesSynced"`
	Warnings           []string `json:"warnings,omitempty"`
}

type keycloakPortalAttributeUpdater interface {
	UpdateClientPortalAttributes(ctx context.Context, clientID string, attributes map[string]string) error
}

func (s *Service) ApplyRealmExportFileSync(ctx context.Context, path string) (SyncApplyResponse, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		discoveredPath, ok := defaultRealmExportPath()
		if !ok {
			return SyncApplyResponse{
				Preview: SyncPreviewResponse{
					Report: RbacDriftReport{
						Source:   "default-realm-export",
						Warnings: []string{"No local infra/keycloak/realm-export.json file was found for startup sync."},
					},
				},
			}, nil
		}
		path = discoveredPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return SyncApplyResponse{}, fmt.Errorf("read realm export %q: %w", path, err)
	}

	response, err := s.ApplyRealmExportSync(ctx, data)
	if err != nil {
		return SyncApplyResponse{}, err
	}
	response.Preview.Report.Source = path
	return response, nil
}

func (s *Service) ApplyLiveKeycloakSync(ctx context.Context, source KeycloakSyncSource) (SyncApplyResponse, error) {
	if source == nil {
		return SyncApplyResponse{}, fmt.Errorf("%w: keycloak source is required", ErrInvalidInput)
	}

	discovered, warnings, err := discoverLiveKeycloakRBAC(ctx, source, s.knownSystemIDs(ctx))
	if err != nil {
		return SyncApplyResponse{}, err
	}

	preview, err := s.previewDiscoveredSync(ctx, "live-keycloak", discovered)
	if err != nil {
		return SyncApplyResponse{}, err
	}
	preview.Report.Warnings = append(preview.Report.Warnings, warnings...)

	response, err := s.applyDiscoveredSync(ctx, preview, discovered, "live-keycloak")
	if err != nil {
		return SyncApplyResponse{}, err
	}
	if groupSource, ok := source.(KeycloakGroupSyncSource); ok {
		groupsSynced, warnings := s.syncLiveKeycloakGroups(ctx, groupSource, discovered.Systems)
		response.Preview.Report.Warnings = append(response.Preview.Report.Warnings, warnings...)
		response.Preview.Report.Warnings = append(response.Preview.Report.Warnings, fmt.Sprintf("Groups synced from Keycloak: %d", groupsSynced))
	}
	return response, nil
}

func (s *Service) syncLiveKeycloakGroups(ctx context.Context, source KeycloakGroupSyncSource, systems []KeycloakDiscoveredSystem) (int, []string) {
	groups, err := source.ListGroups(ctx)
	if err != nil {
		return 0, []string{fmt.Sprintf("Keycloak group sync skipped: %v", err)}
	}

	systemIDs := make([]string, 0, len(systems))
	for _, system := range systems {
		if strings.TrimSpace(system.ClientID) != "" {
			systemIDs = append(systemIDs, system.ClientID)
		}
	}
	sort.Strings(systemIDs)

	warnings := make([]string, 0)
	synced := 0
	for _, kcGroup := range groups {
		path := strings.TrimSpace(kcGroup.Path)
		if path == "" {
			path = "/" + strings.Trim(strings.TrimSpace(kcGroup.Name), "/")
		}
		group, err := s.repository.UpsertGroup(ctx, GroupInput{
			KeycloakGroupID: kcGroup.ID,
			Path:            path,
			Name:            kcGroup.Name,
			DisplayName:     kcGroup.Name,
			Enabled:         enabledBoolPtr(true),
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("group %q metadata sync failed: %v", path, err))
			continue
		}

		members, err := source.ListGroupMembers(ctx, kcGroup.ID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("group %q member sync failed: %v", path, err))
		} else {
			groupMembers := make([]GroupMember, 0, len(members))
			for _, member := range members {
				groupMembers = append(groupMembers, GroupMember{
					UserID:   member.ID,
					Username: member.Username,
					Email:    member.Email,
				})
			}
			if err := s.repository.ReplaceGroupMembers(ctx, group.ID, groupMembers); err != nil {
				warnings = append(warnings, fmt.Sprintf("group %q member persistence failed: %v", path, err))
			}
		}

		realmRoles, err := source.ListGroupRealmRoles(ctx, kcGroup.ID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("group %q realm role sync failed: %v", path, err))
		} else {
			for _, role := range realmRoles {
				roleName := normalize(role.Name)
				if roleName == "" {
					continue
				}
				if err := s.repository.AssignGroupRealmRole(ctx, group.ID, roleName); err != nil {
					warnings = append(warnings, fmt.Sprintf("group %q realm role %q persistence failed: %v", path, roleName, err))
				}
			}
		}

		for _, clientID := range systemIDs {
			clientRoles, err := source.ListGroupClientRoles(ctx, kcGroup.ID, clientID)
			if err != nil {
				continue
			}
			for _, role := range clientRoles {
				roleName := normalize(role.Name)
				if roleName == "" {
					continue
				}
				if err := s.repository.AssignGroupSystemRole(ctx, group.ID, clientID, roleName); err != nil {
					warnings = append(warnings, fmt.Sprintf("group %q client role %q/%q persistence failed: %v", path, clientID, roleName, err))
				}
			}
		}
		synced++
	}

	if synced > 0 {
		if err := s.recordAudit(ctx, "rbac.groups_synced", "sync", "live-keycloak-groups", "", "", "", map[string]any{"groups_synced": synced}); err != nil {
			warnings = append(warnings, fmt.Sprintf("group sync audit failed: %v", err))
		}
	}
	sort.Strings(warnings)
	return synced, warnings
}

func enabledBoolPtr(value bool) *bool {
	return &value
}

func (s *Service) PushMissingRBACRolesToKeycloak(ctx context.Context, source KeycloakSyncSource) (KeycloakPushResult, error) {
	if source == nil {
		return KeycloakPushResult{}, fmt.Errorf("%w: keycloak source is required", ErrInvalidInput)
	}

	discovered, warnings, err := discoverLiveKeycloakRBAC(ctx, source, s.knownSystemIDs(ctx))
	if err != nil {
		return KeycloakPushResult{}, err
	}

	result := KeycloakPushResult{Warnings: warnings}
	discoveredSystems := map[string]KeycloakDiscoveredSystem{}
	for _, system := range discovered.Systems {
		discoveredSystems[system.ClientID] = system
	}

	systems, err := s.repository.ListSystems(ctx)
	if err != nil {
		return KeycloakPushResult{}, err
	}

	for _, system := range systems {
		clientID := strings.TrimSpace(system.ClientID)
		if clientID == "" {
			continue
		}
		detail, err := s.repository.GetSystem(ctx, clientID)
		if err != nil {
			return KeycloakPushResult{}, err
		}

		attributes := map[string]string{
			"ui.icon":                   system.Icon,
			"ui.home":                   system.LaunchURL,
			"ui.launchUrl":              system.LaunchURL,
			"ui.authenticatedLaunchUrl": system.AuthenticatedLaunchURL,
			"ui.category":               system.Category,
			"ui.navigation":             system.Navigation,
			"ui.sidenav":                system.Navigation,
			"ui.systemType":             system.SystemType,
			"ui.displayInLauncher":      fmt.Sprintf("%t", system.DisplayInLauncher),
			"ui.displayInSideNav":       fmt.Sprintf("%t", system.DisplayInSideNav),
			"ui.launchMode":             system.LaunchMode,
			"ui.order":                  fmt.Sprintf("%d", system.SortOrder),
			"portal.system":             "true",
			"portal.accessRoles":        strings.Join(detail.AccessRoles, ","),
		}
		discoveredSystem, ok := discoveredSystems[clientID]
		if !ok {
			clientURL := s.keycloakClientURL(system.LaunchURL)
			if _, err := source.CreateClient(keycloak.CreateClientParams{
				ClientID:    clientID,
				Name:        system.DisplayName,
				Description: system.Description,
				BaseURL:     clientURL,
				RootURL:     clientURL,
				Enabled:     system.Enabled,
				Attributes:  attributes,
			}); err != nil {
				return KeycloakPushResult{}, fmt.Errorf("create keycloak client %q: %w", clientID, err)
			}
			result.ClientsCreated++
			discoveredSystem = KeycloakDiscoveredSystem{ClientID: clientID}
		}
		if updater, ok := source.(keycloakPortalAttributeUpdater); ok {
			if err := updater.UpdateClientPortalAttributes(ctx, clientID, attributes); err != nil {
				return KeycloakPushResult{}, fmt.Errorf("update keycloak client %q portal attributes: %w", clientID, err)
			}
		}

		keycloakRoles := map[string]bool{}
		for _, role := range discoveredSystem.Roles {
			keycloakRoles[normalize(role.Name)] = true
		}

		for _, role := range detail.Roles {
			roleName := normalize(role.Name)
			if roleName == "" || keycloakRoles[roleName] {
				continue
			}

			if err := source.CreateClientRole(ctx, clientID, &model.CreateClientRoleRequest{
				Role:        roleName,
				Description: role.Description,
			}); err != nil {
				return KeycloakPushResult{}, fmt.Errorf("create keycloak client role %q for %q: %w", roleName, clientID, err)
			}
			result.ClientRolesCreated++
		}
	}

	discoveredRealmRoles := map[string]bool{}
	for _, role := range discovered.RealmRoles {
		discoveredRealmRoles[normalize(role)] = true
	}

	realmGroups, err := s.repository.ListRealmRolePermissions(ctx)
	if err != nil {
		return KeycloakPushResult{}, err
	}
	for _, group := range realmGroups {
		roleName := normalize(group.RealmRole)
		if roleName == "" || discoveredRealmRoles[roleName] {
			continue
		}
		if err := source.CreateRealmRole(ctx, roleName, "Synced from MOH Integrated Health Portal"); err != nil {
			return KeycloakPushResult{}, fmt.Errorf("create keycloak realm role %q: %w", roleName, err)
		}
		result.RealmRolesCreated++
	}

	realmSystemRoles, err := s.repository.ListRealmRoleSystemRoles(ctx)
	if err != nil {
		return KeycloakPushResult{}, err
	}
	for _, mapping := range realmSystemRoles {
		if err := source.EnsureRealmRoleClientRoleComposite(
			ctx,
			mapping.RealmRole,
			mapping.ClientID,
			mapping.RoleName,
		); err != nil {
			return KeycloakPushResult{}, fmt.Errorf(
				"sync realm role %q composite %q/%q: %w",
				mapping.RealmRole,
				mapping.ClientID,
				mapping.RoleName,
				err,
			)
		}
		result.CompositesSynced++
	}

	sort.Strings(result.Warnings)
	if err := s.recordAudit(ctx, "rbac.keycloak_push_applied", "sync", "live-keycloak", "", "", "", map[string]any{
		"clients_created":      result.ClientsCreated,
		"realm_roles_created":  result.RealmRolesCreated,
		"client_roles_created": result.ClientRolesCreated,
		"composites_synced":    result.CompositesSynced,
		"warnings":             result.Warnings,
	}); err != nil {
		return KeycloakPushResult{}, err
	}

	return result, nil
}

func (s *Service) keycloakClientURL(launchURL string) string {
	launchURL = strings.TrimSpace(launchURL)
	if launchURL == "" {
		return ""
	}
	if parsed, err := url.Parse(launchURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return launchURL
	}

	base := strings.TrimSpace(s.frontendBaseURL)
	if base == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return ""
	}

	relative, err := url.Parse(launchURL)
	if err != nil {
		return ""
	}
	if relative.IsAbs() {
		return relative.String()
	}

	basePath := strings.TrimRight(baseURL.Path, "/")
	path := relative.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if basePath != "" && path != basePath && !strings.HasPrefix(path, basePath+"/") {
		path = basePath + path
	}

	resolved := *baseURL
	resolved.Path = path
	resolved.RawQuery = relative.RawQuery
	resolved.Fragment = relative.Fragment
	return resolved.String()
}

func (s *Service) previewDiscoveredSync(ctx context.Context, source string, discovered discoveredRBAC) (SyncPreviewResponse, error) {
	report, err := s.buildDriftReport(ctx, source, discovered)
	if err != nil {
		return SyncPreviewResponse{}, err
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
	return preview, nil
}

func (s *Service) applyDiscoveredSync(ctx context.Context, preview SyncPreviewResponse, discovered discoveredRBAC, source string) (SyncApplyResponse, error) {
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
			SortOrder:              system.SortOrder,
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
			if err := s.recordAudit(ctx, "system_role.synced", "system-role", systemRole.ID, system.ClientID, systemRole.Name, "", map[string]any{"source": source}); err != nil {
				return SyncApplyResponse{}, err
			}
			response.RolesSynced++
			if containsNormalized(system.AccessRoles, role.Name) {
				if err := s.repository.AddSystemAccessRole(ctx, system.ClientID, role.Name); err != nil {
					return SyncApplyResponse{}, err
				}
				if err := s.recordAudit(ctx, "system.access_role_synced", "system", system.ClientID, system.ClientID, role.Name, "", map[string]any{"source": source}); err != nil {
					return SyncApplyResponse{}, err
				}
				response.AccessRoles++
			}
		}
	}

	if err := s.recordAudit(ctx, "rbac.sync_applied", "sync", source, "", "", "", map[string]any{"systemsSynced": response.SystemsSynced, "rolesSynced": response.RolesSynced, "accessRolesSynced": response.AccessRoles}); err != nil {
		return SyncApplyResponse{}, err
	}
	return response, nil
}

func discoverLiveKeycloakRBAC(ctx context.Context, source KeycloakSyncSource, knownSystemSets ...map[string]bool) (discoveredRBAC, []string, error) {
	knownSystems := map[string]bool{}
	if len(knownSystemSets) > 0 {
		knownSystems = knownSystemSets[0]
	}
	clients, err := source.ListClients()
	if err != nil {
		return discoveredRBAC{}, nil, fmt.Errorf("list keycloak clients: %w", err)
	}

	realmRoles, err := source.ListRealmRoles(ctx)
	if err != nil {
		return discoveredRBAC{}, nil, fmt.Errorf("list keycloak realm roles: %w", err)
	}

	discovered := discoveredRBAC{
		Systems:    make([]KeycloakDiscoveredSystem, 0, len(clients)),
		RealmRoles: make([]string, 0, len(realmRoles)),
	}
	warnings := make([]string, 0)

	for _, role := range realmRoles {
		name := normalize(role.Name)
		if shouldSkipRealmRole(name) {
			continue
		}
		discovered.RealmRoles = append(discovered.RealmRoles, name)
	}

	for _, client := range clients {
		clientID := strings.TrimSpace(client.ClientID)
		if shouldSkipClientID(clientID) {
			continue
		}
		if !attributeBool(client.Attributes, "portal.system", false) && !knownSystems[clientID] {
			continue
		}

		roles, err := source.ListClientRoles(ctx, clientID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to discover roles for client %q: %v", clientID, err))
			continue
		}

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

	sort.Strings(discovered.RealmRoles)
	sort.Slice(discovered.Systems, func(i, j int) bool {
		if discovered.Systems[i].SortOrder != discovered.Systems[j].SortOrder {
			return discovered.Systems[i].SortOrder < discovered.Systems[j].SortOrder
		}
		return discovered.Systems[i].ClientID < discovered.Systems[j].ClientID
	})
	for i := range discovered.Systems {
		sort.Slice(discovered.Systems[i].Roles, func(a, b int) bool {
			return discovered.Systems[i].Roles[a].Name < discovered.Systems[i].Roles[b].Name
		})
	}
	sort.Strings(warnings)
	return discovered, warnings, nil
}
