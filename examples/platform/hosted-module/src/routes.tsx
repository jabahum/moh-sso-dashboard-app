import type { MicrofrontendRoute } from "@moh-sso/microfrontend";
export const platformPilotRoute: MicrofrontendRoute = {
  appName: "@moh-sso/platform-pilot",
  path: "/apps/pilot-hosted",
  paths: ["/apps/pilot-hosted"],
  requiredSystems: ["pilot-hosted"],
};
