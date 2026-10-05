package authz

import "strings"

const (
	RoleAdmin                = "admin"
	RoleUser                 = "user"
	RoleManager              = "manager"
	RoleDocumentsAdmin       = "documents_admin"
	RoleSurveillanceAdmin    = "surveillance_admin"
	RoleSurveillanceImporter = "surveillance_importer"
	RoleEmailAdmin           = "email_admin"
	RoleAuditReader          = "audit_reader"
	RoleMetricsReader        = "metrics_reader"
)

var rolePermissions = map[string][]Permission{
	RoleAdmin: AllPermissions,
	RoleUser: {
		PermissionPortalAccess,
		PermissionSystemsRead,
		PermissionSystemsLaunch,
	},
	RoleManager: {
		PermissionPortalAccess,
		PermissionSystemsRead,
		PermissionSystemsLaunch,
		PermissionUsersRead,
		PermissionClientsRead,
		PermissionAnnouncementsRead,
		PermissionDocumentsRead,
		PermissionDocumentsWrite,
		PermissionDocumentTemplatesRead,
		PermissionSurveillanceRead,
		PermissionStorageLocationsRead,
		PermissionDataQualityRead,
		PermissionDataQualityWrite,
		PermissionNotificationsRead,
	},
	RoleDocumentsAdmin: {
		PermissionDocumentsRead,
		PermissionDocumentsWrite,
		PermissionDocumentsProcess,
		PermissionDocumentTemplatesRead,
		PermissionDocumentTemplatesWrite,
		PermissionDocumentTemplatesPublish,
		PermissionStorageLocationsRead,
		PermissionStorageLocationsWrite,
	},
	RoleSurveillanceAdmin: {
		PermissionSurveillanceRead,
		PermissionSurveillanceImport,
		PermissionSurveillanceManageLocations,
		PermissionSurveillanceManageAlerts,
	},
	RoleSurveillanceImporter: {
		PermissionSurveillanceRead,
		PermissionSurveillanceImport,
	},
	RoleEmailAdmin: {
		PermissionEmailRead,
		PermissionEmailSend,
		PermissionEmailManage,
	},
	RoleAuditReader: {
		PermissionAuditRead,
	},
	RoleMetricsReader: {
		PermissionMetricsRead,
	},
}

var systemRolePermissions = map[string]map[string][]Permission{
	SystemDashboardWeb: {
		DashboardWebAccess: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
		},
		DashboardWebAdmin: AllPermissions,
		DashboardWebManager: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
			PermissionUsersRead,
			PermissionClientsRead,
			PermissionAnnouncementsRead,
			PermissionDocumentsRead,
			PermissionDocumentTemplatesRead,
			PermissionSurveillanceRead,
			PermissionReportBrowserRead,
			PermissionNotificationsRead,
		},
		DashboardWebUser: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
		},
	},
	SystemOutbreakManagement: {
		OutbreakManagementAccess: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
		},
		OutbreakManagementSuperAdmin: {
			PermissionOutbreakAccess,
			PermissionOutbreakManage,
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionSurveillanceManageLocations,
			PermissionSurveillanceManageAlerts,
			PermissionDataQualityRead,
			PermissionDataQualityWrite,
			PermissionDataQualityResolve,
			PermissionDocumentsRead,
			PermissionDocumentsWrite,
			PermissionDocumentsProcess,
		},
		OutbreakManagementAdmin: {
			PermissionOutbreakAccess,
			PermissionOutbreakManage,
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionSurveillanceManageLocations,
			PermissionSurveillanceManageAlerts,
			PermissionDataQualityRead,
			PermissionDataQualityWrite,
			PermissionDataQualityResolve,
		},
		OutbreakManagementManager: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
			PermissionDataQualityRead,
			PermissionDataQualityWrite,
			PermissionDocumentsRead,
		},
		OutbreakManagementViewer: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
			PermissionDataQualityRead,
			PermissionDocumentsRead,
		},
		OutbreakManagementDataEntry: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionDataQualityRead,
			PermissionDataQualityWrite,
		},
		OutbreakManagementLabTechnician: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
			PermissionDataQualityRead,
			PermissionDataQualityWrite,
		},
		OutbreakManagementSurveillanceOfficer: {
			PermissionOutbreakAccess,
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionSurveillanceManageAlerts,
		},
	},
	SystemDataStatistics: {
		ReportSchedulerAccess: {PermissionPortalAccess, PermissionSystemsRead, PermissionSystemsLaunch, PermissionReportSchedulerRead},
		DataStatisticsAccess: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
			PermissionDataQualityRead,
			PermissionDocumentsRead,
			PermissionSurveillanceRead,
			PermissionReportBrowserRead,
		},
		ReportBrowserAdmin: {
			PermissionReportBrowserRead,
			PermissionMetricsRead,
			PermissionAuditRead,
		},
		ReportBrowserManager: {
			PermissionReportBrowserRead,
			PermissionMetricsRead,
		},
		ReportBrowserAnalyst: {
			PermissionReportBrowserRead,
			PermissionDataQualityRead,
		},
		ReportBrowserViewer: {
			PermissionReportBrowserRead,
		},
		SurveillanceViewer: {
			PermissionSurveillanceRead,
		},
		SurveillanceOfficer: {
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionSurveillanceManageAlerts,
		},
		SurveillanceDataEntry: {
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
		},
		SurveillanceManager: {
			PermissionSurveillanceRead,
			PermissionSurveillanceImport,
			PermissionSurveillanceManageLocations,
			PermissionSurveillanceManageAlerts,
		},
		IssueTrackerViewer: {
			PermissionIssueTrackerRead,
		},
		IssueTrackerContributor: {
			PermissionIssueTrackerRead,
			PermissionIssueTrackerComment,
		},
		IssueTrackerEditor: {
			PermissionIssueTrackerRead,
			PermissionIssueTrackerWrite,
			PermissionIssueTrackerComment,
		},
		IssueTrackerManager: {
			PermissionIssueTrackerRead,
			PermissionIssueTrackerWrite,
			PermissionIssueTrackerManage,
			PermissionIssueTrackerAssign,
			PermissionIssueTrackerClose,
			PermissionIssueTrackerReopen,
			PermissionIssueTrackerComment,
		},
		DocumentViewer: {
			PermissionDocumentsRead,
			PermissionDocumentTemplatesRead,
			PermissionStorageLocationsRead,
		},
		DocumentEditor: {
			PermissionDocumentsRead,
			PermissionDocumentsWrite,
			PermissionDocumentTemplatesRead,
			PermissionStorageLocationsRead,
		},
		DocumentProcessor: {
			PermissionDocumentsRead,
			PermissionDocumentsProcess,
			PermissionDocumentTemplatesRead,
			PermissionStorageLocationsRead,
		},
		DocumentManager: {
			PermissionDocumentsRead,
			PermissionDocumentsWrite,
			PermissionDocumentsProcess,
			PermissionDocumentTemplatesRead,
			PermissionDocumentTemplatesWrite,
			PermissionDocumentTemplatesPublish,
			PermissionStorageLocationsRead,
		},
		DocumentTemplateViewer: {
			PermissionDocumentsRead,
			PermissionDocumentTemplatesRead,
			PermissionStorageLocationsRead,
		},
		DocumentTemplateEditor: {
			PermissionDocumentsRead,
			PermissionDocumentTemplatesRead,
			PermissionDocumentTemplatesWrite,
			PermissionStorageLocationsRead,
		},
		DocumentTemplatePublisher: {
			PermissionDocumentsRead,
			PermissionDocumentTemplatesRead,
			PermissionDocumentTemplatesWrite,
			PermissionDocumentTemplatesPublish,
			PermissionStorageLocationsRead,
		},
	},

	SystemUtilities: {
		UtilitiesAccess: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
		},
	},

	SystemSettings: {
		SettingsAccess: {
			PermissionPortalAccess,
			PermissionSystemsRead,
			PermissionSystemsLaunch,
		},
	},
}

var defaultSystemRolesForRealmRole = map[string]map[string][]string{
	RoleUser: {
		SystemDataStatistics: {DataStatisticsAccess, DocumentViewer},
		SystemUtilities:      {UtilitiesAccess},
		SystemSettings:       {SettingsAccess},
	},
	RoleManager: {
		SystemDataStatistics: {DataStatisticsAccess, SurveillanceManager, ReportBrowserManager, IssueTrackerManager, DocumentManager},
		SystemUtilities:      {UtilitiesAccess},
		SystemSettings:       {SettingsAccess},
	},
}

func PermissionsForContext(realmRoles []string, clientRoles map[string][]string) []Permission {
	seen := map[Permission]bool{}
	permissions := make([]Permission, 0)

	add := func(values []Permission) {
		for _, permission := range values {
			if seen[permission] {
				continue
			}
			seen[permission] = true
			permissions = append(permissions, permission)
		}
	}

	for _, role := range NormalizeRoles(realmRoles) {
		add(rolePermissions[role])
	}

	for system, roles := range clientRolesWithRealmDefaults(realmRoles, clientRoles) {
		roleMap := systemRolePermissions[system]
		for _, role := range roles {
			add(roleMap[role])
		}
	}

	return permissions
}

func AccessibleSystemsForContext(clientRoles map[string][]string) []string {
	return systemIDs(AccessibleSystemDetailsForContext(clientRoles))
}

func AccessibleSystemDetailsForContext(clientRoles map[string][]string) []SystemAccess {
	seen := map[string]bool{}
	systems := make([]SystemAccess, 0)

	add := func(system SystemAccess) {
		clientID := strings.TrimSpace(system.ClientID)
		if clientID == "" || seen[clientID] {
			return
		}
		seen[clientID] = true
		systems = append(systems, system)
	}

	systemAccess := func(system string, roles []string) SystemAccess {
		system = strings.TrimSpace(system)
		metadata := staticSystemMetadata[system]
		return SystemAccess{
			ClientID:               system,
			DisplayName:            metadata.DisplayName,
			LaunchURL:              metadata.LaunchURL,
			AuthenticatedLaunchURL: metadata.AuthenticatedLaunchURL,
			Icon:                   metadata.Icon,
			Category:               metadata.Category,
			SystemType:             "platform",
			DisplayInLauncher:      true,
			DisplayInSideNav:       false,
			LaunchMode:             "internal",
			Roles:                  roles,
		}
	}

	for system, roles := range normalizeClientRoles(clientRoles) {
		if len(roles) == 0 {
			continue
		}

		switch system {
		case SystemDashboardWeb:
			add(systemAccess(system, roles))
		case SystemOutbreakManagement, SystemDataStatistics, SystemUtilities, SystemSettings:
			add(systemAccess(system, roles))
		}
	}

	return systems
}

type systemMetadata struct {
	DisplayName            string
	LaunchURL              string
	AuthenticatedLaunchURL string
	Icon                   string
	Category               string
}

var staticSystemMetadata = map[string]systemMetadata{
	SystemDashboardWeb: {
		DisplayName:            "Integrated Health Portal",
		LaunchURL:              "/portal",
		AuthenticatedLaunchURL: "/api/v1/auth/launch?returnTo=%2Fportal%2Fapps%2Fnews",
		Icon:                   "dashboard",
		Category:               "platform",
	},
	SystemOutbreakManagement: {
		DisplayName: "Outbreak Management",
		LaunchURL:   "/portal/apps/dwh/surveillance",
		Icon:        "outbreak",
		Category:    "surveillance",
	},
	SystemDataStatistics: {
		DisplayName: "Data & Statistics",
		LaunchURL:   "/portal/apps/dwh",
		Icon:        "home",
		Category:    "platform",
	},
	SystemUtilities: {
		DisplayName: "Utilities",
		LaunchURL:   "/portal/apps/utilities",
		Icon:        "tools",
		Category:    "utilities",
	},
	SystemSettings: {
		DisplayName: "Settings",
		LaunchURL:   "/portal/apps/settings",
		Icon:        "settings",
		Category:    "platform",
	},
}

func clientRolesWithRealmDefaults(
	realmRoles []string,
	clientRoles map[string][]string,
) map[string][]string {
	merged := normalizeClientRoles(clientRoles)
	for _, realmRole := range NormalizeRoles(realmRoles) {
		for clientID, roles := range defaultSystemRolesForRealmRole[realmRole] {
			merged[clientID] = NormalizeRoles(append(merged[clientID], roles...))
		}
	}
	return merged
}

func PermissionsForRoles(roles []string) []Permission {
	seen := map[Permission]bool{}
	permissions := make([]Permission, 0)

	for _, role := range NormalizeRoles(roles) {
		for _, permission := range rolePermissions[role] {
			if seen[permission] {
				continue
			}
			seen[permission] = true
			permissions = append(permissions, permission)
		}
	}

	return permissions
}

func NormalizeRoles(roles []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(roles))

	for _, role := range roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		normalized = append(normalized, role)
	}

	return normalized
}

func hasRole(roles []string, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return false
	}

	for _, current := range roles {
		if strings.EqualFold(strings.TrimSpace(current), role) {
			return true
		}
	}

	return false
}
