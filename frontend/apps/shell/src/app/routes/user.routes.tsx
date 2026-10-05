import type { ReactElement } from "react";
import { Navigate, Route } from "react-router-dom";

import { ProtectedRoute } from "./guards/ProtectedRoute";
import { PermissionRoute } from "./guards/PermissionRoute";
import { UserRoute } from "./guards/UserRoute";
import UserLayout from "../layouts/user/user-layout.component";
import { ComingSoon } from "@moh-sso/ui";
import { PERMISSIONS, type Permission, type System } from "@moh-sso/auth";
import type { MicrofrontendRoute } from "@moh-sso/microfrontend";
import {
  dataValidationRoute,
  dataVisualizerRoute,
  documentsRoute,
  eServicesRoute,
  issueTrackerRoute,
  reportBrowserRoute,
  reportSchedulerRoute,
  surveillanceRoute,
  utilitiesRoute,
} from "@/app/microfrontends/registry";

import NewsFeedPage from "@/app/newsfeed/pages/news_feed.component";
import { SettingsLayout } from "@/app/settings/settings-layout";
import MyProfilePage from "@/app/settings/pages/Profile/profile.component";
import SecurityPage from "@/app/settings/pages/security/security.component";
import ActiveSessionsPage from "@/app/settings/pages/sessions/active-sesssions.component";
import SettingsHomePage from "@/app/settings/pages/SettingsHomePage";
import NotificationSettingsPage from "@/app/settings/pages/NotificationSettingsPage";
import PreferenceSettingsPage from "@/app/settings/pages/PreferenceSettingsPage";
import AppSettingsPage from "@/app/settings/pages/AppSettingsPage";
import AboutSettingsPage from "@/app/settings/pages/AboutSettingsPage";

import { SingleSpaApp } from "@/app/microfrontends/SingleSpaApp";

import {
  dataVisualizerLifecycles,
  dataValidationLifecycles,
  documentsLifecycles,
  issueTrackerLifecycles,
  reportBrowserLifecycles,
  reportSchedulerLifecycles,
  surveillanceLifecycles,
  utilitiesLifecycles,
  eServicesLifecycles,
} from "@/app/microfrontends/lifecycles";

type RouteAccess = {
  permission?: Permission;
  allOf?: Permission[];
  anyOf?: Permission[];
  systems?: Array<System | string>;
  systemRoles?: Array<{ system: System | string; role: string }>;
};

const accessFromRoute = (route: MicrofrontendRoute): RouteAccess => ({
  allOf: route.requiredPermissions as Permission[] | undefined,
  anyOf: route.requiredAnyPermissions as Permission[] | undefined,
  systems: route.requiredSystems as System[] | undefined,
  systemRoles: route.requiredSystemRoles,
});

const SYSTEMS = {
  dataStatistics: "data-statistics",
  eServices: "eservices",
  researchStudies: "research-studies",
  caseRegisters: "case-registers",
  outbreakManagement: "outbreak-management",
  referenceRegisters: "reference-registers",
  utilities: "utilities",
  settings: "settings",
} as const;

const withSystemAccess = (
  system: System | string,
  access: Permission | RouteAccess = { allOf: [PERMISSIONS.systemsLaunch] },
): RouteAccess => {
  const routeAccess: RouteAccess = typeof access === "string" ? { permission: access } : access;

  return {
    ...routeAccess,
    allOf: [...(routeAccess.allOf ?? []), PERMISSIONS.systemsLaunch],
    systems: [...(routeAccess.systems ?? []), system],
  };
};

const userPage = (element: ReactElement, access?: Permission | RouteAccess) => {
  const routeAccess: RouteAccess | undefined =
    typeof access === "string" ? { permission: access } : access;

  const guarded = routeAccess ? (
    <PermissionRoute
      permission={routeAccess.permission}
      allOf={routeAccess.allOf}
      anyOf={routeAccess.anyOf}
      systems={routeAccess.systems}
      systemRoles={routeAccess.systemRoles}
    >
      {element}
    </PermissionRoute>
  ) : (
    element
  );

  return <UserRoute>{guarded}</UserRoute>;
};

export const userRoutes = (
  <Route
    path="/apps"
    element={
      <ProtectedRoute>
        <UserLayout />
      </ProtectedRoute>
    }
  >
    <Route index element={<Navigate to="news" replace />} />

    <Route path="news" element={userPage(<NewsFeedPage />)} />

    <Route
      path="report-scheduler/*"
      element={<Navigate to="/apps/dwh/report-scheduler" replace />}
    />

    {/* =========================
        DWH
       ========================= */}
    <Route path="dwh">
      <Route index element={<Navigate to="dashboards" replace />} />

      <Route
        path="report-scheduler/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/report-scheduler"
            lifecycles={reportSchedulerLifecycles}
            basename="/apps/dwh/report-scheduler"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(reportSchedulerRoute)),
        )}
      />

      <Route
        path="data-visualizer/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/data-visualizer"
            lifecycles={dataVisualizerLifecycles}
            basename="/apps/dwh/data-visualizer"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(dataVisualizerRoute)),
        )}
      />

      <Route
        path="data-validation/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/data-validation"
            lifecycles={dataValidationLifecycles}
            basename="/apps/dwh/data-validation"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(dataValidationRoute)),
        )}
      />

      <Route
        path="dashboards/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/report-browser"
            lifecycles={reportBrowserLifecycles}
            basename="/apps/dwh/dashboards"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(reportBrowserRoute)),
        )}
      />

      <Route
        path="documents/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/documents"
            lifecycles={documentsLifecycles}
            basename="/apps/dwh/documents"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(documentsRoute)),
        )}
      />

      <Route
        path="surveillance/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/surveillance"
            lifecycles={surveillanceLifecycles}
            basename="/apps/dwh/surveillance"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(surveillanceRoute)),
        )}
      />

      <Route
        path="issue-tracker/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/issue-tracker"
            lifecycles={issueTrackerLifecycles}
            basename="/apps/dwh/issue-tracker"
          />,
          withSystemAccess(SYSTEMS.dataStatistics, accessFromRoute(issueTrackerRoute)),
        )}
      />
    </Route>

    {/* =========================
        E-SERVICES MICROFRONTEND
       ========================= */}
    <Route
      path="eservices/*"
      element={userPage(
        <SingleSpaApp
          appName="@moh-sso/e-services"
          lifecycles={eServicesLifecycles}
          basename="/apps/eservices"
        />,
        withSystemAccess(SYSTEMS.eServices, accessFromRoute(eServicesRoute)),
      )}
    />

    {/* =========================
        RESEARCH STUDIES
       ========================= */}
    <Route path="research-studies">
      <Route index element={<Navigate to="studies" replace />} />
      <Route
        path="studies"
        element={userPage(
          <ComingSoon title="Studies" />,
          withSystemAccess(SYSTEMS.researchStudies),
        )}
      />
      <Route
        path="datasets"
        element={userPage(
          <ComingSoon title="Datasets" />,
          withSystemAccess(SYSTEMS.researchStudies),
        )}
      />
      <Route
        path="ethics"
        element={userPage(
          <ComingSoon title="Ethics & Approvals" />,
          withSystemAccess(SYSTEMS.researchStudies),
        )}
      />
      <Route
        path="publications"
        element={userPage(
          <ComingSoon title="Publications" />,
          withSystemAccess(SYSTEMS.researchStudies),
        )}
      />
    </Route>

    {/* =========================
        CASE REGISTERS
       ========================= */}
    <Route path="case-registers">
      <Route index element={<Navigate to="external-referrals" replace />} />

      <Route
        path="external-referrals"
        element={userPage(
          <ComingSoon title="External Referrals" />,
          withSystemAccess(SYSTEMS.caseRegisters, PERMISSIONS.documentsRead),
        )}
      />

      <Route
        path="disease-registers"
        element={userPage(
          <ComingSoon title="Disease Registers" />,
          withSystemAccess(SYSTEMS.caseRegisters, PERMISSIONS.documentsRead),
        )}
      />
    </Route>

    {/* =========================
        OUTBREAK MANAGEMENT
       ========================= */}
    <Route path="outbreak-management">
      <Route index element={<Navigate to="signals-alerts" replace />} />

      <Route
        path="signals-alerts"
        element={userPage(
          <ComingSoon title="Signals & Alerts" />,
          withSystemAccess(SYSTEMS.outbreakManagement, {
            anyOf: [PERMISSIONS.outbreakAccess, PERMISSIONS.surveillanceRead],
          }),
        )}
      />

      <Route
        path="poe-management"
        element={userPage(
          <ComingSoon title="PoE Management" />,
          withSystemAccess(SYSTEMS.outbreakManagement, {
            anyOf: [PERMISSIONS.outbreakAccess, PERMISSIONS.surveillanceRead],
          }),
        )}
      />

      <Route
        path="case-management"
        element={userPage(
          <ComingSoon title="Case Management" />,
          withSystemAccess(SYSTEMS.outbreakManagement, {
            anyOf: [PERMISSIONS.outbreakAccess, PERMISSIONS.surveillanceRead],
          }),
        )}
      />
    </Route>

    {/* =========================
        REFERENCE REGISTERS
       ========================= */}
    <Route path="reference-registers">
      <Route index element={<Navigate to="facility-register" replace />} />

      <Route
        path="facility-register"
        element={userPage(
          <ComingSoon title="Facility Register" />,
          withSystemAccess(SYSTEMS.referenceRegisters, PERMISSIONS.systemsRead),
        )}
      />

      <Route path="terminology">
        <Route index element={<Navigate to="test-menu" replace />} />

        <Route
          path="test-menu"
          element={userPage(
            <ComingSoon title="Test Menu" />,
            withSystemAccess(SYSTEMS.referenceRegisters, PERMISSIONS.systemsRead),
          )}
        />

        <Route
          path="pharmaceuticals"
          element={userPage(
            <ComingSoon title="Pharmaceuticals" />,
            withSystemAccess(SYSTEMS.referenceRegisters, PERMISSIONS.systemsRead),
          )}
        />

        <Route
          path="procedures"
          element={userPage(
            <ComingSoon title="Procedures" />,
            withSystemAccess(SYSTEMS.referenceRegisters, PERMISSIONS.systemsRead),
          )}
        />

        <Route
          path="equipment"
          element={userPage(
            <ComingSoon title="Equipment" />,
            withSystemAccess(SYSTEMS.referenceRegisters, PERMISSIONS.systemsRead),
          )}
        />
      </Route>
    </Route>

    {/* =========================
    UTILITIES MICROFRONTEND
   ========================= */}
    <Route path="utilities">
      <Route index element={<Navigate to="self-service" replace />} />

      {/* E-Service routes should stay in the host because they mount other apps */}
      <Route path="self-service/eservice">
        <Route index element={<Navigate to="document-upload" replace />} />

        <Route
          path="document-upload/*"
          element={userPage(
            <SingleSpaApp
              appName="@moh-sso/documents"
              lifecycles={documentsLifecycles}
              basename="/apps/utilities/self-service/eservice/document-upload"
            />,
            {
              anyOf: [PERMISSIONS.documentsWrite],
              systems: [SYSTEMS.utilities],
              allOf: [PERMISSIONS.systemsLaunch],
            },
          )}
        />
      </Route>

      {/* Mount utilities once here */}
      <Route
        path="self-service/*"
        element={userPage(
          <SingleSpaApp
            appName="@moh-sso/utilities"
            lifecycles={utilitiesLifecycles}
            basename="/apps/utilities/self-service"
          />,
          withSystemAccess(SYSTEMS.utilities, accessFromRoute(utilitiesRoute)),
        )}
      />
    </Route>

    {/* =========================
        SETTINGS
       ========================= */}
    <Route
      path="settings"
      element={userPage(<SettingsLayout />, withSystemAccess(SYSTEMS.settings))}
    >
      <Route index element={<SettingsHomePage />} />
      <Route path="profile" element={<MyProfilePage />} />
      <Route path="sessions" element={<ActiveSessionsPage />} />
      <Route path="security" element={<SecurityPage />} />
      <Route path="notifications" element={<NotificationSettingsPage />} />
      <Route path="preferences" element={<PreferenceSettingsPage />} />
      <Route path="apps" element={<AppSettingsPage />} />
      <Route path="about" element={<AboutSettingsPage />} />
    </Route>

    {/* =========================
        FALLBACK
       ========================= */}
    <Route path="*" element={<Navigate to="news" replace />} />
  </Route>
);
