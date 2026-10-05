import type { MicrofrontendRoute } from "@moh-sso/microfrontend";

export const announcementsRoute: MicrofrontendRoute = {
  appName: "@moh-sso/announcements",
  path: "/admin/announcements",
  requiredPermissions: ["announcements:read"],
};

export const auditRoute: MicrofrontendRoute = {
  appName: "@moh-sso/audit",
  path: "/admin/audit-logs",
  requiredPermissions: ["audit:read"],
};

export const clientsRoute: MicrofrontendRoute = {
  appName: "@moh-sso/clients",
  path: "/admin/clients",
  requiredPermissions: ["clients:read"],
};

export const dataValidationRoute: MicrofrontendRoute = {
  appName: "@moh-sso/data-validation",
  path: "/apps/dwh/data-validation",
  requiredPermissions: ["data_quality:read"],
  requiredSystems: ["data-statistics"],
};

export const dataVisualizerRoute: MicrofrontendRoute = {
  appName: "@moh-sso/data-visualizer",
  path: "/apps/dwh/data-visualizer",
  requiredAnyPermissions: ["documents:read", "data_quality:read"],
  requiredSystems: ["data-statistics"],
};

export const documentsRoute: MicrofrontendRoute = {
  appName: "@moh-sso/documents",
  path: "/apps/dwh/documents",
  requiredSystems: ["data-statistics"],
  requiredAnyPermissions: ["documents:read"],
};

export const eServicesRoute: MicrofrontendRoute = {
  appName: "@moh-sso/e-services",
  path: "/apps/eservices",
  requiredPermissions: ["systems:launch"],
};

export const emailRoute: MicrofrontendRoute = {
  appName: "@moh-sso/email",
  path: "/admin/emails",
  requiredPermissions: ["email:read"],
};

export const issueTrackerRoute: MicrofrontendRoute = {
  appName: "@moh-sso/issue-tracker",
  path: "/apps/dwh/issue-tracker",
  requiredPermissions: ["issue_tracker:read"],
  requiredSystems: ["data-statistics"],
};

export const reportBrowserRoute: MicrofrontendRoute = {
  appName: "@moh-sso/report-browser",
  path: "/apps/dwh/dashboards",
  paths: ["/apps/dwh/dashboards"],
  requiredPermissions: ["report_browser:read"],
  requiredSystems: ["data-statistics"],
};

export const rbacRoute: MicrofrontendRoute = {
  appName: "@moh-sso/rbac",
  path: "/admin/rbac",
  requiredPermissions: ["rbac:read"],
};

export const surveillanceRoute: MicrofrontendRoute = {
  appName: "@moh-sso/surveillance",
  path: "/apps/dwh/surveillance",
  requiredAnyPermissions: ["surveillance:read"],
  requiredSystems: ["data-statistics"],
};

export const usersRoute: MicrofrontendRoute = {
  appName: "@moh-sso/users",
  path: "/admin/users",
  requiredPermissions: ["users:read"],
};

export const utilitiesRoute: MicrofrontendRoute = {
  appName: "@moh-sso/utilities",
  path: "/apps/utilities/self-service",
  paths: ["/apps/utilities/self-service"],
};

export const reportSchedulerRoute: MicrofrontendRoute = {
  appName: "@moh-sso/report-scheduler",
  path: "/apps/dwh/report-scheduler",
  requiredPermissions: ["report_scheduler:read"],
  requiredSystems: ["data-statistics"],
};

export const registeredMicrofrontends: MicrofrontendRoute[] = [
  announcementsRoute,
  auditRoute,
  clientsRoute,
  dataValidationRoute,
  dataVisualizerRoute,
  documentsRoute,
  eServicesRoute,
  emailRoute,
  issueTrackerRoute,
  reportBrowserRoute,
  reportSchedulerRoute,
  rbacRoute,
  surveillanceRoute,
  usersRoute,
  utilitiesRoute,
];
