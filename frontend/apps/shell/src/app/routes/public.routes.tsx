import { Navigate, Route, useLocation } from "react-router-dom";

import PublicLayout from "../layouts/public/public-layout.component";
import { ForbiddenPage } from "./ForbiddenPage";
import { PortalLandingRoute } from "./PortalLandingRoute";

function IssueRedirect() {
  const location = useLocation();
  return <Navigate to={{ pathname: "/apps/dwh/issue-tracker/issues", search: location.search }} replace />;
}

export const publicRoutes = (
  <Route element={<PublicLayout />}>
    <Route index element={<PortalLandingRoute />} />
    <Route path="/" element={<PortalLandingRoute />} />
    <Route path="/forbidden" element={<ForbiddenPage />} />
    <Route path="/issue/*" element={<IssueRedirect />} />
    <Route path="/issues/*" element={<IssueRedirect />} />
  </Route>
);
