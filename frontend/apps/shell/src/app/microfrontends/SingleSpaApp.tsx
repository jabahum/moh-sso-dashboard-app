import { useEffect, useMemo, useRef, useState } from "react";
import { useSelector } from "react-redux";
import { useLocation } from "react-router-dom";

import {
  resolveRuntimeBasename,
  type MicrofrontendLifecycle,
  type MicrofrontendRuntimeProps,
} from "@moh-sso/microfrontend";
import { selectAuthenticated, selectUser } from "@moh-sso/auth";
import { MicrofrontendErrorBoundary } from "@moh-sso/ui";
import { microfrontendContainerId } from "./containers";
import {
  isSingleSpaOrchestrationStarted,
  isSingleSpaOrchestrationUnavailable,
  MICROFRONTEND_ORCHESTRATION_EVENT,
  shouldUseSingleSpaOrchestration,
  startMicrofrontendOrchestration,
} from "./orchestrator";
import "./microfrontends.scss";

type SingleSpaAppProps = MicrofrontendRuntimeProps & {
  appName: string;
  lifecycles: MicrofrontendLifecycle | (() => Promise<MicrofrontendLifecycle>);
};

export function SingleSpaApp({ appName, lifecycles, ...runtimeProps }: SingleSpaAppProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const location = useLocation();
  const shellUser = useSelector(selectUser);
  const shellAuthenticated = useSelector(selectAuthenticated);
  const { apiBaseUrl, eventBus } = runtimeProps;
  const auth = useMemo(
    () =>
      runtimeProps.auth ?? {
        isAuthenticated: shellAuthenticated,
        user: shellUser ?? undefined,
      },
    [runtimeProps.auth, shellAuthenticated, shellUser],
  );
  const orchestrationRequested = shouldUseSingleSpaOrchestration();
  const [orchestrationState, setOrchestrationState] = useState(() => ({
    started: isSingleSpaOrchestrationStarted(),
    unavailable: isSingleSpaOrchestrationUnavailable(),
  }));
  const [mountError, setMountError] = useState<Error | null>(null);
  const orchestrated =
    orchestrationRequested && orchestrationState.started && !orchestrationState.unavailable;
  const shouldMountLocally = !orchestrationRequested || orchestrationState.unavailable;

  const basename = resolveRuntimeBasename(
    runtimeProps.basename || location.pathname.replace(/\/$/, ""),
    import.meta.env.BASE_URL,
  );

  useEffect(() => {
    const handleOrchestrationChange = () => {
      setOrchestrationState({
        started: isSingleSpaOrchestrationStarted(),
        unavailable: isSingleSpaOrchestrationUnavailable(),
      });
    };

    window.addEventListener(MICROFRONTEND_ORCHESTRATION_EVENT, handleOrchestrationChange);
    handleOrchestrationChange();

    return () => {
      window.removeEventListener(MICROFRONTEND_ORCHESTRATION_EVENT, handleOrchestrationChange);
    };
  }, []);

  useEffect(() => {
    window.__MOH_SSO_AUTH__ = auth;

    return () => {
      if (window.__MOH_SSO_AUTH__ === auth) {
        delete window.__MOH_SSO_AUTH__;
      }
    };
  }, [auth]);

  useEffect(() => {
    // Notify embedded microfrontends that listen to popstate/URL changes
    window.dispatchEvent(new PopStateEvent("popstate"));
  }, [location.pathname, location.search]);

  useEffect(() => {
    if (orchestrationRequested && !orchestrationState.started && !orchestrationState.unavailable) {
      void startMicrofrontendOrchestration();
    }
  }, [orchestrationRequested, orchestrationState.started, orchestrationState.unavailable]);

  useEffect(() => {
    if (!shouldMountLocally) {
      return;
    }

    const domElement = containerRef.current;
    if (!domElement) {
      return;
    }

    let disposed = false;
    const props = { apiBaseUrl, auth, basename, eventBus, domElement };
    let mountedLifecycles: MicrofrontendLifecycle | null = null;

    const loadAndMount = async () => {
      try {
        setMountError(null);
        const resolvedLifecycles =
          typeof lifecycles === "function" ? await lifecycles() : lifecycles;
        mountedLifecycles = resolvedLifecycles;

        if (disposed) return;

        await resolvedLifecycles.bootstrap(props);

        if (disposed) return;

        // Catch internal framework/rendering crashes (like Context/Hook mismatches)
        await resolvedLifecycles.mount(props);
      } catch (error: unknown) {
        console.error(`Failed to load or mount microfrontend ${appName}`, error);
        if (!disposed) {
          setMountError(error instanceof Error ? error : new Error(String(error)));
        }
      }
    };

    void loadAndMount();

    return () => {
      disposed = true;
      if (mountedLifecycles) {
        // Safely unmount and trap errors to prevent unmount bubbles from crashing the host
        Promise.resolve(mountedLifecycles.unmount(props)).catch((err) => {
          console.error(`Clean unmount failed for microfrontend: ${appName}`, err);
        });
      }
      // Hard wipe the DOM element to prevent unmounted/corrupted nodes from leaking
      if (domElement) {
        domElement.innerHTML = "";
      }
    };
  }, [apiBaseUrl, appName, auth, basename, eventBus, lifecycles, shouldMountLocally]);

  return (
    <MicrofrontendErrorBoundary appName={appName}>
      {mountError ? (
        <div role="alert" className="microfrontend-mount-error">
          Unable to load {appName}. {mountError.message}
        </div>
      ) : null}
      <div
        id={orchestrated || orchestrationRequested ? microfrontendContainerId(appName) : undefined}
        ref={containerRef}
        data-microfrontend={appName}
      />
    </MicrofrontendErrorBoundary>
  );
}
