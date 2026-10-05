import {
  resolveRuntimeBasename,
  type MicrofrontendLifecycle,
  type MicrofrontendRoute,
} from "@moh-sso/microfrontend";

import {
  announcementsLifecycles,
  auditLifecycles,
  clientsLifecycles,
  dataValidationLifecycles,
  dataVisualizerLifecycles,
  documentsLifecycles,
  eServicesLifecycles,
  emailLifecycles,
  issueTrackerLifecycles,
  reportBrowserLifecycles,
 reportSchedulerLifecycles,
  rbacLifecycles,
  surveillanceLifecycles,
  usersLifecycles,
  utilitiesLifecycles,
} from "./lifecycles";
import { getMicrofrontendContainer } from "./containers";
import { registeredMicrofrontends } from "./registerApps";

type MicrofrontendMountMode = "hybrid" | "orchestrated";

type RuntimeMicrofrontendConfig = {
  singleSpaOrchestration?: boolean;
  microfrontendMode?: "local" | "remote";
  microfrontendMountMode?: MicrofrontendMountMode;
};

type SingleSpaModule = {
  registerApplication: (config: {
    name: string;
    app: () => Promise<MicrofrontendLifecycle>;
    activeWhen: Array<(location: Location) => boolean>;
    customProps: Record<string, unknown>;
  }) => void;
  start: () => void;
};

export const MICROFRONTEND_ORCHESTRATION_EVENT = "moh-sso:microfrontend-orchestration";

let orchestrationStarted = false;
let orchestrationUnavailable = false;
let orchestrationStartPromise: Promise<boolean> | null = null;

const lifecycleLoaders: Record<string, () => Promise<MicrofrontendLifecycle>> = {
  "@moh-sso/announcements": announcementsLifecycles,
  "@moh-sso/audit": auditLifecycles,
  "@moh-sso/clients": clientsLifecycles,
  "@moh-sso/data-validation": dataValidationLifecycles,
  "@moh-sso/data-visualizer": dataVisualizerLifecycles,
  "@moh-sso/documents": documentsLifecycles,
  "@moh-sso/e-services": eServicesLifecycles,
  "@moh-sso/email": emailLifecycles,
  "@moh-sso/issue-tracker": issueTrackerLifecycles,
  "@moh-sso/report-browser": reportBrowserLifecycles,
 "@moh-sso/report-scheduler": reportSchedulerLifecycles,
  "@moh-sso/rbac": rbacLifecycles,
  "@moh-sso/surveillance": surveillanceLifecycles,
  "@moh-sso/users": usersLifecycles,
  "@moh-sso/utilities": utilitiesLifecycles,
};

function notifyOrchestrationState() {
  window.dispatchEvent(
    new CustomEvent(MICROFRONTEND_ORCHESTRATION_EVENT, {
      detail: {
        started: orchestrationStarted,
        unavailable: orchestrationUnavailable,
      },
    }),
  );
}

function getRuntimeConfig() {
  const runtimeConfig = (
    window as Window & {
      __APP_CONFIG__?: RuntimeMicrofrontendConfig;
    }
  ).__APP_CONFIG__;

  return runtimeConfig;
}

export function shouldUseSingleSpaOrchestration() {
  const runtimeConfig = getRuntimeConfig();
  const orchestrationEnabled =
    runtimeConfig?.singleSpaOrchestration ?? import.meta.env.VITE_SINGLE_SPA_ORCHESTRATION === "true";

  return (
    orchestrationEnabled &&
    getMicrofrontendMode() === "remote" &&
    getMicrofrontendMountMode() === "orchestrated"
  );
}

export function isSingleSpaOrchestrationStarted() {
  return orchestrationStarted;
}

export function isSingleSpaOrchestrationUnavailable() {
  return orchestrationUnavailable;
}

function getMicrofrontendMode() {
  const runtimeConfig = getRuntimeConfig();
  return runtimeConfig?.microfrontendMode ?? import.meta.env.VITE_MICROFRONTEND_MODE ?? "local";
}

function getMicrofrontendMountMode(): MicrofrontendMountMode {
  const runtimeConfig = getRuntimeConfig();
  return runtimeConfig?.microfrontendMountMode ?? import.meta.env.VITE_MICROFRONTEND_MOUNT_MODE ?? "hybrid";
}

function pathMatches(pathname: string, basePath: string) {
  const normalizedPathname = pathname.replace(/\/+$/, "") || "/";
  const normalizedBasePath = basePath.replace(/\/+$/, "") || "/";

  return (
    normalizedPathname === normalizedBasePath ||
    normalizedPathname.startsWith(`${normalizedBasePath}/`)
  );
}

async function loadSingleSpa(): Promise<SingleSpaModule | null> {
  try {
    return await runtimeImport("single-spa");
  } catch {
    return null;
  }
}

function registerRoute(singleSpa: SingleSpaModule, route: MicrofrontendRoute) {
  const mode = getMicrofrontendMode();
  const routeBasename = resolveRuntimeBasename(route.path, import.meta.env.BASE_URL);
  const excludedPaths = (route.excludedPaths ?? []).map((path) =>
    resolveRuntimeBasename(path, import.meta.env.BASE_URL),
  );
  const activeWhen = (route.paths ?? [route.path])
    .map((path) => resolveRuntimeBasename(path, import.meta.env.BASE_URL))
    .map((activePath) => (location: Location) => {
      if (!pathMatches(location.pathname, activePath)) {
        return false;
      }

      return !excludedPaths.some((excludedPath) => pathMatches(location.pathname, excludedPath));
    });
  const loader =
    mode === "remote"
      ? () => runtimeImport<MicrofrontendLifecycle>(route.appName)
      : lifecycleLoaders[route.appName];

  if (!loader) {
    return;
  }

  singleSpa.registerApplication({
    name: route.appName,
    app: async () => {
      const lifecycle = await loader();
      const propsWithContainer = (props: Record<string, unknown>) => ({
        ...props,
        basename: routeBasename,
        domElement: getMicrofrontendContainer(route.appName),
      });

      return {
        bootstrap: (props) => lifecycle.bootstrap(propsWithContainer(props)),
        mount: (props) => lifecycle.mount(propsWithContainer(props)),
        unmount: (props) => lifecycle.unmount(propsWithContainer(props)),
      };
    },
    activeWhen,
    customProps: {
      basename: routeBasename,
    },
  });
}

function runtimeImport<T>(specifier: string): Promise<T> {
  const load = new Function("specifier", "return import(specifier)") as (
    specifier: string,
  ) => Promise<T>;
  return load(specifier);
}

async function startMicrofrontendOrchestrationOnce() {
  if (!shouldUseSingleSpaOrchestration()) {
    orchestrationStarted = false;
    orchestrationUnavailable = false;
    notifyOrchestrationState();
    return false;
  }

  const singleSpa = await loadSingleSpa();

  if (!singleSpa) {
    orchestrationStarted = false;
    orchestrationUnavailable = true;
    notifyOrchestrationState();
    return false;
  }

  registeredMicrofrontends.forEach((route) => registerRoute(singleSpa, route));
  singleSpa.start();
  orchestrationStarted = true;
  orchestrationUnavailable = false;
  notifyOrchestrationState();
  return true;
}

export function startMicrofrontendOrchestration() {
  if (orchestrationStarted) {
    return Promise.resolve(true);
  }

  if (orchestrationStartPromise) {
    return orchestrationStartPromise;
  }

  orchestrationStartPromise = startMicrofrontendOrchestrationOnce().finally(() => {
    if (!orchestrationStarted) {
      orchestrationStartPromise = null;
    }
  });

  return orchestrationStartPromise;
}
