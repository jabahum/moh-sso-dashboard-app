import { useSelector } from "react-redux";
import { useLocation, useNavigate } from "react-router-dom";

import { useAuthorization } from "@moh-sso/auth";
import { selectActiveClient } from "@moh-sso/state";
import { ClientSideNav } from "@moh-sso/ui";
import { buildAccessibleSideNavClients } from "@/app/access/accessClients";

type ConnectedClientSideNavProps = {
  hasPermission?: (permission: string) => boolean;
  visible?: boolean;
  onToggleVisibility?: () => void;
};

export function ConnectedClientSideNav({
  hasPermission,
  visible,
  onToggleVisibility,
}: ConnectedClientSideNavProps) {
  const activeClient = useSelector(selectActiveClient);
  const location = useLocation();
  const navigate = useNavigate();
  const { accessibleSystems, can } = useAuthorization();
  const visibleClients = buildAccessibleSideNavClients({ accessibleSystems });
  const checkPermission = hasPermission ?? ((permission: string) => can(permission as never));

  const handleNavigate = (path: string) => {
    navigate(path);
    window.dispatchEvent(new PopStateEvent("popstate"));
  };

  return (
    <ClientSideNav
      clients={visibleClients}
      activeClient={activeClient}
      currentPath={location.pathname + location.search}
      hasPermission={checkPermission}
      visible={visible}
      onToggleVisibility={onToggleVisibility}
      onNavigate={handleNavigate}
    />
  );
}
