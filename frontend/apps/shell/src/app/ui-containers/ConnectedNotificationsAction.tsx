import { HeaderGlobalAction } from "@carbon/react";
import { Notification } from "@carbon/react/icons";

import { PERMISSIONS, useAuthorization } from "@moh-sso/auth";

import { useGetUnreadNotificationsCountQuery } from "../api";
import { useOpenNotifications } from "./ConnectedNotificationsPanel";

import "./notifications-action.scss";

/**
 * Header bell shared by the admin and user layouts. The notifications API
 * requires `notifications:read`, so the action is only rendered (and the
 * unread count only fetched) for users who hold that permission.
 */
export function ConnectedNotificationsAction() {
  const { can } = useAuthorization();

  if (!can(PERMISSIONS.notificationsRead)) {
    return null;
  }

  return <NotificationsAction />;
}

function NotificationsAction() {
  const { data: unreadCount = 0 } = useGetUnreadNotificationsCountQuery();
  const openNotifications = useOpenNotifications();
  const safeUnreadCount = Number(unreadCount) || 0;

  return (
    <HeaderGlobalAction
      aria-label={
        safeUnreadCount > 0 ? `Notifications, ${safeUnreadCount} unread` : "Notifications"
      }
      tooltipAlignment="end"
      onClick={() => openNotifications()}
    >
      <Notification size={20} />

      {safeUnreadCount > 0 && (
        <span className="moh-header-notification-badge">
          {safeUnreadCount > 99 ? "99+" : safeUnreadCount}
        </span>
      )}
    </HeaderGlobalAction>
  );
}
