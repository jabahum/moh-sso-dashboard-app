import { Content } from "@carbon/react";
import { Outlet } from "react-router-dom";

import { HeaderPanelProvider, PublicFooter } from "@moh-sso/ui";

import { useVersionInfo } from "@/app/version/useVersionInfo";
import { ConnectedPublicHeader } from "@/app/ui-containers";
import { PanelRouteReset } from "../PanelRouteReset";
import "./public-layout.scss";

export default function PublicLayout() {
  const versionInfo = useVersionInfo();

  return (
    <div className="public-layout app-side-panel-layout">
      <HeaderPanelProvider>
        <PanelRouteReset />
        <ConnectedPublicHeader />

        <main className="public-layout__main app-side-panel-content">
          <Content id="main-content" className="public-layout__content">
            <Outlet />
          </Content>
        </main>

        <PublicFooter
          frontendVersion={versionInfo.frontend.version}
          backendVersion={versionInfo.backend?.version}
          backendUnavailable={!versionInfo.isLoading && !versionInfo.backend}
        />
      </HeaderPanelProvider>
    </div>
  );
}
