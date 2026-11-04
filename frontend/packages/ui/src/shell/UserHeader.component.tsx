import { UserAvatarFilled, Logout } from "@carbon/react/icons";
import {
  Header,
  HeaderName,
  HeaderGlobalBar,
  HeaderGlobalAction,
  SkipToContent,
} from "@carbon/react";
import type React from "react";
import type { ReactNode } from "react";

import AppMenuAction from "../navigation/appmenu/AppMenu.component";
import "./public-header.scss";

import imagePath from "../assets/logo.png";

export type UserHeaderProps = {
  appName?: string;
  username?: string | null;
  appMenu?: ReactNode;
  onNavigateHome?: () => void;
  onLogout?: () => void;
  onProfile?: () => void;
};

const UserHeader: React.FC<UserHeaderProps> = ({
  appName = "Integrated Health Portal",
  username,
  appMenu,
  onNavigateHome,
  onLogout,
  onProfile,
}) => {
  return (
    <Header aria-label={`MOH ${appName}`}>
      <SkipToContent />

      <img src={imagePath} className="moh-image-style" alt="Uganda coat of arms" />
      <HeaderName prefix="MOH" onClick={onNavigateHome} className="moh-header-name" title={appName}>
        {appName}
      </HeaderName>

      <HeaderGlobalBar>
        {appMenu ?? <AppMenuAction />}
        <HeaderGlobalAction
          aria-label={`Signed in as ${username ?? "user"}`}
          tooltipAlignment="end"
          onClick={onProfile}
        >
          <UserAvatarFilled size={20} />
        </HeaderGlobalAction>
        <HeaderGlobalAction aria-label="Logout" tooltipAlignment="end" onClick={onLogout}>
          <Logout size={20} />
        </HeaderGlobalAction>
      </HeaderGlobalBar>
    </Header>
  );
};

export default UserHeader;
