import { useHeaderPanel } from "@moh-sso/ui";
import { useEffect } from "react";
import { useLocation } from "react-router-dom";

export function PanelRouteReset() {
  const { pathname } = useLocation();
  const { closePanel } = useHeaderPanel();

  useEffect(() => {
    closePanel();
  }, [pathname, closePanel]);

  return null;
}
