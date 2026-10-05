package system_rbac

import "github.com/moh-sso-dashboard/internal/authz"

func defaultReportSchedulerSystem(enabled bool) SeedSystem {
	system := defaultPortalSystem(authz.SystemReportScheduler, "Report Scheduler", "Schedule report generation and delivery.", "reporting", "/portal/apps/report-scheduler", "reporting", `[{"id":"schedules","label":"Report Scheduler","path":"/apps/report-scheduler","permission":"report_scheduler:read"}]`, []string{string(authz.PermissionReportSchedulerRead)}, enabled)
	launcher, sideNav := true, true
	system.SystemType = "platform"
	system.DisplayInLauncher = &launcher
	system.DisplayInSideNav = &sideNav
	system.LaunchMode = "internal"
	system.SortOrder = 30
	return system
}
