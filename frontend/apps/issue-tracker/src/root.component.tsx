import { resolveRuntimeBasename, type MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { BrowserRouter, Route, Routes, Navigate, NavLink, useLocation } from "react-router-dom";

import IssueTracker from "./pages/issue-tracker.component";
import IssueDashboard from "./pages/issue-dashboard.component";

import { PageShell } from "@moh-sso/ui";
import "./issue-tracker-shell.scss";

const DEFAULT_ISSUE_TRACKER_BASE = "/apps/dwh/issue-tracker";

function normalizePath(value?: string) {
  if (!value) return "";
  return value.startsWith("/") ? value : `/${value}`;
}

function resolveIssueTrackerBasename(props: MicrofrontendRuntimeProps) {
  const propBasename = normalizePath(props.basename);
  if (propBasename) {
    return resolveRuntimeBasename(propBasename);
  }
  return resolveRuntimeBasename(DEFAULT_ISSUE_TRACKER_BASE);
}

function IssueTrackerIndexRedirect() {
  const location = useLocation();
  return <Navigate to={{ pathname: "issues", search: location.search }} replace />;
}

export function IssueTrackerRoot(props: MicrofrontendRuntimeProps) {
  const basename = resolveIssueTrackerBasename(props);

  return (
    <div className="moh-microfrontend-root issue-tracker-module">
      <BrowserRouter basename={basename}>
        <PageShell
          title="Issue Tracker"
          subtitle="Monitor data quality, review summaries, and manage reported issues."
        >
          <nav className="issue-tracker-navigation" aria-label="Issue Tracker pages">
            <NavLink to="/dashboard">Issue Summary</NavLink>
            <NavLink to="/issues">Registered Issues</NavLink>
          </nav>
          <Routes>
            <Route path="dashboard/*" element={<IssueDashboard />} />
            <Route path="dashboard" element={<IssueDashboard />} />
            <Route path="issues/*" element={<IssueTracker />} />
            <Route path="issues" element={<IssueTracker />} />
            <Route index element={<IssueTrackerIndexRedirect />} />
            <Route path="*" element={<IssueTracker />} />
          </Routes>
        </PageShell>
      </BrowserRouter>
    </div>
  );
}
