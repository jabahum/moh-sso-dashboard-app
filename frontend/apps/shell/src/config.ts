type ShellRuntimeConfig = {
  hostRuntime?: boolean;
  standaloneModule?: string;
  disablePwa?: boolean;
  singleSpaOrchestration?: boolean;
  microfrontendMode?: "local" | "remote";
  microfrontendMountMode?: "hybrid" | "orchestrated";
};

const DEFAULT_DEV_CONFIG: ShellRuntimeConfig = {
  singleSpaOrchestration: false,
  microfrontendMode: "local",
  microfrontendMountMode: "hybrid",
};

const runtimeConfig = (
  window as Window & {
    __APP_CONFIG__?: ShellRuntimeConfig;
  }
).__APP_CONFIG__;

const isLocalDevelopmentHost =
  window.location.hostname === "localhost" ||
  window.location.hostname === "127.0.0.1" ||
  window.location.hostname === "::1";

export const APP_CONFIG: ShellRuntimeConfig =
  isLocalDevelopmentHost && !runtimeConfig?.hostRuntime
    ? {
        ...DEFAULT_DEV_CONFIG,
        ...runtimeConfig,
        singleSpaOrchestration: false,
        microfrontendMode: "local",
        microfrontendMountMode: "hybrid",
      }
    : {
        ...DEFAULT_DEV_CONFIG,
        ...runtimeConfig,
      };
