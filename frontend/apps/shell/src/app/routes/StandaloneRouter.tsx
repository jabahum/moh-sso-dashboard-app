import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import type { Permission } from "@moh-sso/auth";
import type { MicrofrontendLifecycle } from "@moh-sso/microfrontend";

import { APP_CONFIG } from "../../config";
import { registeredMicrofrontends } from "../microfrontends/registry";
import { SingleSpaApp } from "../microfrontends/SingleSpaApp";
import { ProtectedRoute } from "./guards/ProtectedRoute";
import { PermissionRoute } from "./guards/PermissionRoute";

// Keep a stable loader reference so navigation/auth updates don't create new loaders.
const route = registeredMicrofrontends.find(
  (entry) => entry.appName === APP_CONFIG.standaloneModule,
);
const loadModule = () => {
  const load = new Function("specifier", "return import(specifier)") as (
    specifier: string,
  ) => Promise<MicrofrontendLifecycle>;
  return load(APP_CONFIG.standaloneModule!);
};

export function StandaloneRouter() {
  if (!route) return <main role="alert">Unknown standalone module.</main>;

  return (
    <BrowserRouter basename="/portal">
      <Routes>
        <Route path="/" element={<Navigate to={route.path} replace />} />
        <Route
          path="/forbidden"
          element={<main role="alert">You do not have permission to open this module.</main>}
        />
        {(route.paths ?? [route.path]).map((path) => (
          <Route
            key={path}
            path={`${path}/*`}
            element={
              <ProtectedRoute>
                <PermissionRoute
                  allOf={route.requiredPermissions as Permission[] | undefined}
                  anyOf={route.requiredAnyPermissions as Permission[] | undefined}
                  systems={route.requiredSystems}
                  systemRoles={route.requiredSystemRoles}
                >
                  <SingleSpaApp appName={route.appName} lifecycles={loadModule} basename={path} />
                </PermissionRoute>
              </ProtectedRoute>
            }
          />
        ))}
        <Route path="*" element={<main>Page not available in this standalone application.</main>} />
      </Routes>
    </BrowserRouter>
  );
}
