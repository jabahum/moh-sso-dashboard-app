import type { MicrofrontendLifecycle } from "@moh-sso/microfrontend";

export const announcementsLifecycles = () =>
  import("@moh-sso/announcements/single-spa") as Promise<MicrofrontendLifecycle>;

export const auditLifecycles = () =>
  import("@moh-sso/audit/single-spa") as Promise<MicrofrontendLifecycle>;

export const clientsLifecycles = () =>
  import("@moh-sso/clients/single-spa") as Promise<MicrofrontendLifecycle>;

export const dataVisualizerLifecycles = () =>
  import("@moh-sso/data-visualizer/single-spa") as Promise<MicrofrontendLifecycle>;

export const dataValidationLifecycles = () =>
  import("@moh-sso/data-validation/single-spa") as Promise<MicrofrontendLifecycle>;

export const documentsLifecycles = () =>
  import("@moh-sso/documents/single-spa") as Promise<MicrofrontendLifecycle>;

export const eServicesLifecycles = () =>
  import("@moh-sso/e-services/single-spa") as Promise<MicrofrontendLifecycle>;

export const emailLifecycles = () =>
  import("@moh-sso/email/single-spa") as Promise<MicrofrontendLifecycle>;

export const issueTrackerLifecycles = () =>
  import("@moh-sso/issue-tracker/single-spa") as Promise<MicrofrontendLifecycle>;

export const reportBrowserLifecycles = () =>
  import("@moh-sso/report-browser/single-spa") as Promise<MicrofrontendLifecycle>;

export const reportSchedulerLifecycles = () =>
  import("@moh-sso/report-scheduler/single-spa") as Promise<MicrofrontendLifecycle>;

export const rbacLifecycles = () =>
  import("@moh-sso/rbac/single-spa") as Promise<MicrofrontendLifecycle>;

export const surveillanceLifecycles = () =>
  import("@moh-sso/surveillance/single-spa") as Promise<MicrofrontendLifecycle>;

export const usersLifecycles = () =>
  import("@moh-sso/users/single-spa") as Promise<MicrofrontendLifecycle>;

export const utilitiesLifecycles = () =>
  import("@moh-sso/utilities/single-spa") as Promise<MicrofrontendLifecycle>;
