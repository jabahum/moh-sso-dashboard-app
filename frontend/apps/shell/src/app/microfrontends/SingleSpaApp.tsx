import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSelector } from "react-redux";
import { useLocation } from "react-router-dom";

import {
  resolveRuntimeBasename,
  type MicrofrontendLifecycle,
  type MicrofrontendRuntimeProps,
} from "@moh-sso/microfrontend";
import { selectAuthenticated, selectUser, useHealthContext } from "@moh-sso/auth";
import { MicrofrontendErrorBoundary } from "@moh-sso/ui";
import { attachMicrofrontendContainer } from "./containers";
import {
  isSingleSpaOrchestrationStarted,
  isSingleSpaOrchestrationUnavailable,
  MICROFRONTEND_ORCHESTRATION_EVENT,
  shouldUseSingleSpaOrchestration,
  startMicrofrontendOrchestration,
} from "./orchestrator";
import "./microfrontends.scss";

type SingleSpaAppProps = MicrofrontendRuntimeProps & {
  orchestrate?: boolean;
  appName: string;
  lifecycles: MicrofrontendLifecycle | (() => Promise<MicrofrontendLifecycle>);
};

export function SingleSpaApp({
  appName,
  lifecycles,
  orchestrate = true,
  ...runtimeProps
}: SingleSpaAppProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const location = useLocation();
  const shellUser = useSelector(selectUser);
  const shellAuthenticated = useSelector(selectAuthenticated);
  const shellHealthContext = useHealthContext();
  const { apiBaseUrl, eventBus } = runtimeProps;
  const auth = useMemo(
    () =>
      runtimeProps.auth ?? {
        isAuthenticated: shellAuthenticated,
        user: shellUser ?? undefined,
      },
    [runtimeProps.auth, shellAuthenticated, shellUser],
  );
  const healthContext = useMemo(
    () =>
      runtimeProps.healthContext ?? {
        activeContext: shellHealthContext.activeContext,
        contexts: shellHealthContext.contexts,
        selectContext: shellHealthContext.selectContext,
      },
    [
      runtimeProps.healthContext,
      shellHealthContext.activeContext,
      shellHealthContext.contexts,
      shellHealthContext.selectContext,
    ],
  );
  const orchestrationRequested = orchestrate && shouldUseSingleSpaOrchestration();
  const [orchestrationState, setOrchestrationState] = useState(() => ({
    started: isSingleSpaOrchestrationStarted(),
    unavailable: isSingleSpaOrchestrationUnavailable(),
  }));
  const [mountError, setMountError] = useState<Error | null>(null);
  const shouldMountLocally = !orchestrationRequested || orchestrationState.unavailable;
  const attachHost = useCallback(
    (host: HTMLDivElement | null) => {
      containerRef.current = host;
      if (host && orchestrationRequested) {
        attachMicrofrontendContainer(appName, host);
      }
    },
    [appName, orchestrationRequested],
  );

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
    window.__MOH_SSO_HEALTH_CONTEXT__ = healthContext;

    return () => {
      if (window.__MOH_SSO_AUTH__ === auth) {
        delete window.__MOH_SSO_AUTH__;
      }
      if (window.__MOH_SSO_HEALTH_CONTEXT__ === healthContext) {
        delete window.__MOH_SSO_HEALTH_CONTEXT__;
      }
    };
  }, [auth, healthContext]);

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
    const props = { apiBaseUrl, auth, basename, eventBus, healthContext, domElement };
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
  }, [
    apiBaseUrl,
    appName,
    auth,
    basename,
    eventBus,
    healthContext,
    lifecycles,
    shouldMountLocally,
  ]);

  return (
    <MicrofrontendErrorBoundary appName={appName}>
      {mountError ? (
        <div role="alert" className="microfrontend-mount-error">
          Unable to load {appName}. {mountError.message}
        </div>
      ) : null}
      <div ref={attachHost} data-microfrontend={appName} />
    </MicrofrontendErrorBoundary>
  );
}
