package authz

type Permission string

const (
	PermissionPortalAccess  Permission = "portal:access"
	PermissionSystemsRead   Permission = "systems:read"
	PermissionSystemsLaunch Permission = "systems:launch"

	PermissionUsersRead       Permission = "users:read"
	PermissionUsersWrite      Permission = "users:write"
	PermissionUsersRolesWrite Permission = "users:roles:write"

	PermissionClientsRead       Permission = "clients:read"
	PermissionClientsWrite      Permission = "clients:write"
	PermissionClientsRolesWrite Permission = "clients:roles:write"

	PermissionAnnouncementsRead    Permission = "announcements:read"
	PermissionAnnouncementsWrite   Permission = "announcements:write"
	PermissionAnnouncementsPublish Permission = "announcements:publish"

	PermissionDocumentsRead    Permission = "documents:read"
	PermissionDocumentsWrite   Permission = "documents:write"
	PermissionDocumentsProcess Permission = "documents:process"

	PermissionDocumentTemplatesRead    Permission = "document_templates:read"
	PermissionDocumentTemplatesWrite   Permission = "document_templates:write"
	PermissionDocumentTemplatesPublish Permission = "document_templates:publish"

	PermissionSurveillanceRead            Permission = "surveillance:read"
	PermissionSurveillanceImport          Permission = "surveillance:import"
	PermissionSurveillanceManageLocations Permission = "surveillance:manage_locations"
	PermissionSurveillanceManageAlerts    Permission = "surveillance:manage_alerts"

	PermissionEmailRead   Permission = "email:read"
	PermissionEmailSend   Permission = "email:send"
	PermissionEmailManage Permission = "email:manage"

	PermissionStorageLocationsRead  Permission = "storage_locations:read"
	PermissionStorageLocationsWrite Permission = "storage_locations:write"

	PermissionDataQualityRead    Permission = "data_quality:read"
	PermissionDataQualityWrite   Permission = "data_quality:write"
	PermissionDataQualityResolve Permission = "data_quality:resolve"

	PermissionIssueTrackerRead    Permission = "issue_tracker:read"
	PermissionIssueTrackerWrite   Permission = "issue_tracker:write"
	PermissionIssueTrackerManage  Permission = "issue_tracker:manage"
	PermissionIssueTrackerAssign  Permission = "issue_tracker:assign"
	PermissionIssueTrackerClose   Permission = "issue_tracker:close"
	PermissionIssueTrackerReopen  Permission = "issue_tracker:reopen"
	PermissionIssueTrackerComment Permission = "issue_tracker:comment"

	PermissionReportBrowserRead   Permission = "report_browser:read"
	PermissionReportSchedulerRead    Permission = "report_scheduler:read"
	PermissionReportSchedulerCreate  Permission = "report_scheduler:create"
	PermissionReportSchedulerUpdate  Permission = "report_scheduler:update"
	PermissionReportSchedulerDelete  Permission = "report_scheduler:delete"
	PermissionReportSchedulerExecute Permission = "report_scheduler:execute"
	PermissionReportSchedulerHistory Permission = "report_scheduler:history"
	PermissionReportSchedulerManage  Permission = "report_scheduler:manage"

	PermissionOutbreakAccess Permission = "outbreak:access"
	PermissionOutbreakManage Permission = "outbreak:manage"

	PermissionMetricsRead Permission = "metrics:read"
	PermissionAuditRead   Permission = "audit:read"

	PermissionNotificationsRead  Permission = "notifications:read"
	PermissionNotificationsWrite Permission = "notifications:write"

	PermissionRBACRead             Permission = "rbac:read"
	PermissionRBACWrite            Permission = "rbac:write"
	PermissionRBACRolesWrite       Permission = "rbac:roles:write"
	PermissionRBACPermissionsWrite Permission = "rbac:permissions:write"
)

var AllPermissions = []Permission{
	PermissionPortalAccess,
	PermissionSystemsRead,
	PermissionSystemsLaunch,

	PermissionUsersRead,
	PermissionUsersWrite,
	PermissionUsersRolesWrite,

	PermissionClientsRead,
	PermissionClientsWrite,
	PermissionClientsRolesWrite,

	PermissionAnnouncementsRead,
	PermissionAnnouncementsWrite,
	PermissionAnnouncementsPublish,

	PermissionDocumentsRead,
	PermissionDocumentsWrite,
	PermissionDocumentsProcess,

	PermissionDocumentTemplatesRead,
	PermissionDocumentTemplatesWrite,
	PermissionDocumentTemplatesPublish,

	PermissionSurveillanceRead,
	PermissionSurveillanceImport,
	PermissionSurveillanceManageLocations,
	PermissionSurveillanceManageAlerts,

	PermissionEmailRead,
	PermissionEmailSend,
	PermissionEmailManage,

	PermissionStorageLocationsRead,
	PermissionStorageLocationsWrite,

	PermissionDataQualityRead,
	PermissionDataQualityWrite,
	PermissionDataQualityResolve,

	PermissionIssueTrackerRead,
	PermissionIssueTrackerWrite,
	PermissionIssueTrackerManage,
	PermissionIssueTrackerAssign,
	PermissionIssueTrackerClose,
	PermissionIssueTrackerReopen,
	PermissionIssueTrackerComment,

		PermissionReportBrowserRead,
		PermissionReportSchedulerRead,
		PermissionReportSchedulerCreate,
		PermissionReportSchedulerUpdate,
		PermissionReportSchedulerDelete,
		PermissionReportSchedulerExecute,
		PermissionReportSchedulerHistory,
		PermissionReportSchedulerManage,

	PermissionOutbreakAccess,
	PermissionOutbreakManage,

	PermissionMetricsRead,
	PermissionAuditRead,

	PermissionNotificationsRead,
	PermissionNotificationsWrite,

	PermissionRBACRead,
	PermissionRBACWrite,
	PermissionRBACRolesWrite,
	PermissionRBACPermissionsWrite,
}
