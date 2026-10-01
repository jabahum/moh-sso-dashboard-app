import { BreadcrumbBar } from "@moh-sso/ui";
import { useLocation, useNavigate } from "react-router-dom";

import { buildBreadcrumbs } from "./breadcrumbs";

export function RouteBreadcrumbBar() {
  const location = useLocation();
  const navigate = useNavigate();

  const items = buildBreadcrumbs(location.pathname);

  if (items.length <= 1) {
    return null;
  }

  return (
    <BreadcrumbBar
      items={items}
      onNavigate={(href) => navigate(href)}
      showBackButton={false}
    />
  );
}
