package authz

const (
	SystemDashboardWeb       = "dashboard-web"
	SystemOutbreakManagement = "outbreak-management"
	SystemDataStatistics     = "data-statistics"
	SystemUtilities          = "utilities"

	SystemSettings        = "settings"
	SystemReportScheduler = "report-scheduler"
)

const (
	DashboardWebAccess  = "dashboard-web_access"
	DashboardWebAdmin   = "portal_admin"
	DashboardWebManager = "portal_manager"
	DashboardWebUser    = "portal_user"

	OutbreakManagementAccess              = "outbreak-management_access"
	OutbreakManagementSuperAdmin          = "super_admin"
	OutbreakManagementAdmin               = "admin"
	OutbreakManagementManager             = "manager"
	OutbreakManagementViewer              = "viewer"
	OutbreakManagementDataEntry           = "data_entry"
	OutbreakManagementLabTechnician       = "lab_technician"
	OutbreakManagementSurveillanceOfficer = "surveillance_officer"

	ReportBrowserAdmin   = "report_admin"
	ReportBrowserManager = "report_manager"
	ReportBrowserAnalyst = "report_analyst"
	ReportBrowserViewer  = "report_viewer"

	DataStatisticsAccess = "data-statistics_access"

	SurveillanceViewer    = "surveillance_viewer"
	SurveillanceOfficer   = "surveillance_officer"
	SurveillanceDataEntry = "surveillance_data_entry"
	SurveillanceManager   = "surveillance_manager"

	IssueTrackerViewer      = "issue_tracker_viewer"
	IssueTrackerContributor = "issue_tracker_contributor"
	IssueTrackerEditor      = "issue_tracker_editor"
	IssueTrackerManager     = "issue_tracker_manager"

	DocumentViewer    = "document_viewer"
	DocumentEditor    = "document_editor"
	DocumentProcessor = "document_processor"
	DocumentManager   = "document_manager"

	DocumentTemplateViewer    = "document_template_viewer"
	DocumentTemplateEditor    = "document_template_editor"
	DocumentTemplatePublisher = "document_template_publisher"

	UtilitiesAccess       = "utilities_access"
	SettingsAccess        = "settings_access"
	ReportSchedulerAccess = "report-scheduler_access"
)
