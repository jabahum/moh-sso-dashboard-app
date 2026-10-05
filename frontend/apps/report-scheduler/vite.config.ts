import { defineMicrofrontendConfig } from "../../build/vite.microfrontend.config";

export default defineMicrofrontendConfig({ appUrl: import.meta.url, name: "MohSsoReportScheduler" });
