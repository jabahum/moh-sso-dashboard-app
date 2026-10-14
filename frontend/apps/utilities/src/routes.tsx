import type { MicrofrontendRoute } from "@moh-sso/microfrontend";

export const utilitiesRoute: MicrofrontendRoute = {
  appName: "@moh-sso/utilities",
  path: "/apps/utilities/self-service",
  paths: ["/apps/utilities/self-service"],
  requiredSystems: ["utilities"],
};
