// Copy this template to frontend/apps/platform-pilot before building.
import { defineMicrofrontendConfig } from "../../build/vite.microfrontend.config";
export default defineMicrofrontendConfig({ appUrl: import.meta.url, name: "MohSsoPlatformPilot" });
