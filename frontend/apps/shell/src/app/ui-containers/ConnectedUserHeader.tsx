import { useSelector } from "react-redux";
import { useNavigate } from "react-router-dom";

import { selectUser } from "@moh-sso/auth";
import { API } from "@moh-sso/config";
import { UserHeader, useHeaderPanel } from "@moh-sso/ui";

import { ConnectedAccountPanel } from "./ConnectedAccountPanel";
import { ConnectedAppMenu } from "./ConnectedAppMenu";
import { ConnectedNotificationsAction } from "./ConnectedNotificationsAction";
import { HealthContextSelector } from "./HealthContextSelector";

type Props = {
  navigationExpanded?: boolean;
  onToggleNavigation?: () => void;
  onGlobalAction?: () => void;
};

export function ConnectedUserHeader({
  navigationExpanded,
  onToggleNavigation,
  onGlobalAction,
}: Props) {
  const navigate = useNavigate();
  const user = useSelector(selectUser);
  const { openPanel } = useHeaderPanel();

  const handleLogout = () => {
    window.location.replace(API.auth.logout());
  };

  return (
    <UserHeader
      username={user?.username}
      // Production builds the shell against the published @moh-sso/ui, so the
      // bell rides in the existing contextSelector slot rather than a new prop.
      contextSelector={
        <>
          <HealthContextSelector />
          <ConnectedNotificationsAction />
        </>
      }
      navigationExpanded={navigationExpanded}
      onToggleNavigation={onToggleNavigation}
      onGlobalAction={onGlobalAction}
      onProfile={() =>
        openPanel({
          title: "Your account",
          size: "sm",
          maximizable: false,
          content: <ConnectedAccountPanel />,
        })
      }
      appMenu={<ConnectedAppMenu />}
      onNavigateHome={() => navigate("/apps")}
      onLogout={handleLogout}
    />
  );
}
