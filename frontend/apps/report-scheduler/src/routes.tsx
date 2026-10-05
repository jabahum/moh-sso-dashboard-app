import type { MicrofrontendRoute } from "@moh-sso/microfrontend";

export const reportSchedulerRoute: MicrofrontendRoute = {
  appName: "@moh-sso/report-scheduler",
  path: "/apps/dwh/report-scheduler",
  requiredPermissions: ["report_scheduler:read"],
  requiredSystems: ["data-statistics"],
};
