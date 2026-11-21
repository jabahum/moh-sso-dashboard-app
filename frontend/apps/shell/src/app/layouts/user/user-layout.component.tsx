import { Content } from "@carbon/react";
import { Outlet } from "react-router-dom";
import { useMemo } from "react";

import { useAuthorization } from "@moh-sso/auth";
import { HeaderPanelProvider, PublicFooter, hasVisibleClientSideNav } from "@moh-sso/ui";
import { buildAccessibleSideNavClients } from "@/app/access/accessClients";

import { RouteBreadcrumbBar } from "@/app/navigation/RouteBreadcrumbBar";
import { ConnectedClientSideNav, ConnectedUserHeader } from "@/app/ui-containers";
import { useVersionInfo } from "@/app/version/useVersionInfo";

import { useShellNavigation } from "../useShellNavigation";
import { PanelRouteReset } from "../PanelRouteReset";
import "./user-layout.scss";

const USER_SIDENAV_STORAGE_KEY = "moh.userLayout.sideNavVisible";

export default function UserLayout() {
  const { accessibleSystems, can } = useAuthorization();
  const versionInfo = useVersionInfo();
  const clients = useMemo(
    () => buildAccessibleSideNavClients({ accessibleSystems }),
    [accessibleSystems],
  );
  const hasSideNav = hasVisibleClientSideNav(clients, (permission) => can(permission as never));
  const navigation = useShellNavigation(USER_SIDENAV_STORAGE_KEY, hasSideNav);
  const isSideNavVisible = navigation.visible;
  const layoutClassName = [
    "user-layout app-side-panel-layout",
    !hasSideNav ? "user-layout--without-sidenav" : "",
    hasSideNav && !isSideNavVisible ? "user-layout--sidenav-hidden" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={layoutClassName}>
      <HeaderPanelProvider>
        <PanelRouteReset />
        <ConnectedUserHeader
          navigationExpanded={isSideNavVisible}
          onGlobalAction={navigation.compact ? () => navigation.setVisible(false) : undefined}
          onToggleNavigation={hasSideNav ? navigation.toggle : undefined}
        />

        <div className="user-layout__body app-side-panel-content">
          {hasSideNav && (
            <ConnectedClientSideNav
              visible={isSideNavVisible}
              onToggleVisibility={navigation.toggle}
            />
          )}

          {hasSideNav && isSideNavVisible ? (
            <button
              type="button"
              className="user-layout__sidenav-backdrop"
              aria-label="Close navigation"
              onClick={() => navigation.setVisible(false)}
            />
          ) : null}

          <div className="user-layout__content-shell">
            <RouteBreadcrumbBar />

            <Content id="main-content" className="user-layout__content">
              <Outlet />
            </Content>
          </div>
        </div>

        <PublicFooter
          frontendVersion={versionInfo.frontend.version}
          backendVersion={versionInfo.backend?.version}
          backendUnavailable={!versionInfo.isLoading && !versionInfo.backend}
        />
      </HeaderPanelProvider>
    </div>
  );
}
