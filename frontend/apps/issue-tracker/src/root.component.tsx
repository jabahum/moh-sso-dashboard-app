import { resolveRuntimeBasename, type MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import { BrowserRouter, Route, Routes, Navigate, useLocation } from "react-router-dom";

import IssueTracker from "./pages/issue-tracker.component";
import IssueDashboard from "./pages/issue-dashboard.component";

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
    <div className="moh-microfrontend-root">
      <BrowserRouter basename={basename}>
        <Routes>
          <Route path="dashboard/*" element={<IssueDashboard />} />
          <Route path="dashboard" element={<IssueDashboard />} />
          <Route path="issues/*" element={<IssueTracker />} />
          <Route path="issues" element={<IssueTracker />} />
          <Route index element={<IssueTrackerIndexRedirect />} />
          <Route path="*" element={<IssueTracker />} />
        </Routes>
      </BrowserRouter>
    </div>
  );
}
