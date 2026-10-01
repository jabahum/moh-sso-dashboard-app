import { useEffect, useMemo, useRef, useState } from "react";
import type { ComponentType } from "react";
import {
  Content,
  Header,
  HeaderGlobalAction,
  HeaderGlobalBar,
  HeaderName,
  SideNav,
  SideNavItems,
  SideNavLink,
} from "@carbon/react";
import {
  Activity,
  Api,
  Application,
  Bullhorn,
  Dashboard,
  Email,
  Logout,
  ChevronLeft,
  ChevronRight,
  Notification,
  UserAvatarFilled,
  UserMultiple,
  UserRole,
} from "@carbon/react/icons";
import { useSelector } from "react-redux";
import { Outlet, useLocation, useNavigate } from "react-router-dom";

import {
  HeaderPanelProvider,
  NotificationsPanel,
  PublicFooter,
  useHeaderPanel,
  useFocusTrap,
} from "@moh-sso/ui";

import { API } from "@moh-sso/config";
import {
  useDeleteNotificationMutation,
  useGetNotificationsQuery,
  useGetUnreadNotificationsCountQuery,
  useMarkNotificationAsReadMutation,
} from "../../api";
import { PERMISSIONS, selectUser, useAuthorization } from "@moh-sso/auth";
import type { Permission } from "@moh-sso/auth";

import { RouteBreadcrumbBar } from "@/app/navigation/RouteBreadcrumbBar";
import { useVersionInfo } from "@/app/version/useVersionInfo";
import { NotificationDetailPanel } from "./NotificationDetailPanel";

import "./admin-layout.scss";

import imagePath from "../../../assets/logo.png";

type CarbonIconComponent = ComponentType<{
  size?: number | string;
  className?: string;
  "aria-label"?: string;
}>;

type AdminNavItem = {
  id: string;
  label: string;
  path: string;
  icon: CarbonIconComponent;
  group: "main" | "identity" | "communications" | "governance";
  exact?: boolean;
  requiredPermission?: Permission;
  requiredAnyPermissions?: Permission[];
};

const ADMIN_NAV_ITEMS: AdminNavItem[] = [
  {
    id: "home",
    label: "Home",
    path: "/admin",
    icon: Dashboard,
    group: "main",
    exact: true,
    requiredPermission: PERMISSIONS.portalAccess,
  },
  {
    id: "users",
    label: "Users",
    path: "/admin/users",
    icon: UserMultiple,
    group: "identity",
    requiredPermission: PERMISSIONS.usersRead,
  },
  {
    id: "clients",
    label: "Clients",
    path: "/admin/clients",
    icon: Api,
    group: "identity",
    requiredPermission: PERMISSIONS.clientsRead,
  },
  {
    id: "systems",
    label: "Systems",
    path: "/admin/systems",
    icon: Application,
    group: "identity",
    requiredAnyPermissions: [PERMISSIONS.systemsRead, PERMISSIONS.rbacRead],
  },
  {
    id: "announcements",
    label: "Announcements",
    path: "/admin/announcements",
    icon: Bullhorn,
    group: "communications",
    requiredPermission: PERMISSIONS.announcementsRead,
  },
  {
    id: "emails",
    label: "Emails",
    path: "/admin/emails",
    icon: Email,
    group: "communications",
    requiredAnyPermissions: [PERMISSIONS.emailRead, PERMISSIONS.emailManage],
  },
  {
    id: "notification-deliveries",
    label: "Deliveries",
    path: "/admin/notifications/deliveries",
    icon: Notification,
    group: "communications",
    requiredPermission: PERMISSIONS.notificationsRead,
  },
  {
    id: "audit-logs",
    label: "Audits",
    path: "/admin/audit-logs",
    icon: Activity,
    group: "governance",
    requiredPermission: PERMISSIONS.auditRead,
  },
  {
    id: "rbac",
    label: "RBAC",
    path: "/admin/rbac",
    icon: UserRole,
    group: "governance",
    requiredAnyPermissions: [PERMISSIONS.rbacRead, PERMISSIONS.rbacRolesWrite],
  },
];

const ADMIN_NAV_GROUPS: Array<{
  id: AdminNavItem["group"];
  label: string;
}> = [
  { id: "main", label: "Overview" },
  { id: "identity", label: "Access" },
  { id: "communications", label: "Messaging" },
  { id: "governance", label: "Governance" },
];

const ADMIN_SIDENAV_STORAGE_KEY = "moh.adminLayout.sideNavVisible";

function readStoredSideNavVisible(): boolean {
  if (typeof window === "undefined") {
    return true;
  }

  if (window.matchMedia("(max-width: 1056px)").matches) {
    return window.localStorage.getItem(ADMIN_SIDENAV_STORAGE_KEY) === "true";
  }

  return window.localStorage.getItem(ADMIN_SIDENAV_STORAGE_KEY) !== "false";
}

function isActiveRoute(pathname: string, item: AdminNavItem): boolean {
  if (item.exact) {
    return pathname === item.path || pathname === "portal/admin/home";
  }

  return pathname === item.path || pathname.startsWith(`${item.path}/`);
}

function HeaderActions() {
  const user = useSelector(selectUser);
  const { openPanel } = useHeaderPanel();
  const { can } = useAuthorization();

  const {
    data: notifications = [],
    isLoading: isLoadingNotifications,
    refetch: refetchUnreadNotifications,
  } = useGetNotificationsQuery({
    unread: true,
    limit: 10,
    offset: 0,
  });
  const {
    data: recentNotifications = [],
    isLoading: isLoadingRecentNotifications,
    refetch: refetchRecentNotifications,
  } = useGetNotificationsQuery({
    limit: 20,
    offset: 0,
  });

  const { data: unreadCount = 0 } = useGetUnreadNotificationsCountQuery();
  const [markNotificationAsRead] = useMarkNotificationAsReadMutation();
  const [deleteNotification] = useDeleteNotificationMutation();

  const safeUnreadCount = Number(unreadCount) || 0;

  const handleLogout = () => {
    window.location.replace(API.auth.logout());
  };

  const handleOpenNotifications = () => {
    openPanel({
      title: "Notifications",
      content: (
        <div className="notifications-header-panel">
          <NotificationsPanel
            notifications={notifications}
            recentNotifications={recentNotifications}
            isLoading={isLoadingNotifications}
            isLoadingRecent={isLoadingRecentNotifications}
            onMarkRead={(id) => {
              void markNotificationAsRead(id).then(() => {
                void refetchUnreadNotifications();
                void refetchRecentNotifications();
              });
            }}
            onDelete={(id) => {
              void deleteNotification(id).then(() => {
                void refetchUnreadNotifications();
                void refetchRecentNotifications();
              });
            }}
            onView={(notification) => {
              openPanel({
                title: "Notification details",
                size: "md",
                content: (
                  <NotificationDetailPanel
                    notification={notification}
                    canRetryDelivery={can(PERMISSIONS.notificationsWrite)}
                  />
                ),
              });
            }}
          />
        </div>
      ),
    });
  };

  return (
    <>
      <HeaderGlobalAction
        aria-label={
          safeUnreadCount > 0 ? `Notifications, ${safeUnreadCount} unread` : "Notifications"
        }
        tooltipAlignment="end"
        onClick={handleOpenNotifications}
      >
        <Notification size={20} />

        {safeUnreadCount > 0 && (
          <span className="admin-notification-badge">
            {safeUnreadCount > 99 ? "99+" : safeUnreadCount}
          </span>
        )}
      </HeaderGlobalAction>

      <HeaderGlobalAction
        aria-label={`Signed in as ${user?.username ?? "user"}`}
        tooltipAlignment="end"
      >
        <UserAvatarFilled size={20} />
      </HeaderGlobalAction>

      <HeaderGlobalAction aria-label="Logout" tooltipAlignment="end" onClick={handleLogout}>
        <Logout size={20} />
      </HeaderGlobalAction>
    </>
  );
}

type AdminSideNavProps = {
  visible: boolean;
  onToggleVisibility: () => void;
  onNavigate?: () => void;
};

function AdminSideNav({ visible, onToggleVisibility, onNavigate }: AdminSideNavProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { can, canAny } = useAuthorization();
  const navRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const shouldTrapFocus =
    visible && typeof window !== "undefined" && window.matchMedia("(max-width: 1056px)").matches;

  const navItems = useMemo(
    () =>
      ADMIN_NAV_ITEMS.filter(
        (item) =>
          (!item.requiredPermission || can(item.requiredPermission)) &&
          (!item.requiredAnyPermissions || canAny(item.requiredAnyPermissions)),
      ),
    [can, canAny],
  );

  useFocusTrap({
    active: shouldTrapFocus,
    containerRef: navRef,
    initialFocusRef: closeButtonRef,
  });

  return (
    <>
      {!visible ? (
        <button
          type="button"
          className="admin-layout__sidenav-toggle admin-layout__sidenav-toggle--rail"
          aria-label="Expand navigation"
          title="Expand navigation"
          aria-controls="admin-sidenav"
          aria-expanded={false}
          onClick={onToggleVisibility}
        >
          <ChevronRight size={20} aria-hidden="true" />
        </button>
      ) : null}

      <SideNav
        ref={navRef}
        id="admin-sidenav"
        isFixedNav
        expanded
        aria-hidden={!visible}
        aria-label="Admin navigation"
        className={`admin-layout__sidenav${visible ? "" : " admin-layout__sidenav--hidden"}`}
      >
        <SideNavItems>
          <div className="admin-layout__sidenav-header">
            <div className="admin-layout__sidenav-heading">
              <span className="admin-layout__sidenav-kicker">Administration</span>
              <span className="admin-layout__sidenav-title">Portal Console</span>
            </div>

            <button
              ref={closeButtonRef}
              type="button"
              className="admin-layout__sidenav-toggle"
              aria-label="Collapse navigation"
              title="Collapse navigation"
              aria-controls="admin-sidenav"
              aria-expanded={visible}
              onClick={onToggleVisibility}
            >
              <ChevronLeft size={20} aria-hidden="true" />
            </button>
          </div>

          {ADMIN_NAV_GROUPS.map((group) => {
            const groupItems = navItems.filter((item) => item.group === group.id);

            if (groupItems.length === 0) {
              return null;
            }

            return (
              <div key={group.id} className="admin-layout__sidenav-group">
                <div className="admin-layout__sidenav-group-label">{group.label}</div>

                {groupItems.map((item) => {
                  const active = isActiveRoute(location.pathname, item);

                  return (
                    <SideNavLink
                      key={item.id}
                      isActive={active}
                      aria-current={active ? "page" : undefined}
                      renderIcon={item.icon}
                      onClick={() => {
                        if (location.pathname !== item.path) {
                          navigate(item.path);
                        }
                        onNavigate?.();
                      }}
                    >
                      {item.label}
                    </SideNavLink>
                  );
                })}
              </div>
            );
          })}
        </SideNavItems>
      </SideNav>
    </>
  );
}

export default function AdminLayout() {
  const navigate = useNavigate();
  const [isSideNavVisible, setIsSideNavVisible] = useState(readStoredSideNavVisible);
  const versionInfo = useVersionInfo();

  useEffect(() => {
    if (!isSideNavVisible) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && window.matchMedia("(max-width: 1056px)").matches) {
        setIsSideNavVisible(false);
      }
    };

    const originalOverflow = document.body.style.overflow;

    if (window.matchMedia("(max-width: 1056px)").matches) {
      document.body.style.overflow = "hidden";
    }

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = originalOverflow;
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isSideNavVisible]);

  useEffect(() => {
    window.localStorage.setItem(ADMIN_SIDENAV_STORAGE_KEY, String(isSideNavVisible));
  }, [isSideNavVisible]);

  return (
    <HeaderPanelProvider>
      <div className={`admin-layout${isSideNavVisible ? "" : " admin-layout--sidenav-hidden"}`}>
        <Header aria-label="MOH Integrated Health Portal" className="admin-layout__header">
          <div className="admin-layout__brand">
            <button
              type="button"
              className="admin-layout__brand-logo-button"
              onClick={() => navigate("/admin")}
              aria-label="Go to admin home"
            >
              <img src={imagePath} className="admin-layout__brand-logo" alt="" aria-hidden="true" />
            </button>

            <HeaderName
              prefix="MOH"
              onClick={() => navigate("/admin")}
              className="admin-layout__brand-name"
            >
              Integrated Health Portal
            </HeaderName>
          </div>

          <HeaderGlobalBar>
            <HeaderActions />
          </HeaderGlobalBar>
        </Header>

        <AdminSideNav
          visible={isSideNavVisible}
          onToggleVisibility={() => setIsSideNavVisible((visible) => !visible)}
          onNavigate={() => {
            if (window.matchMedia("(max-width: 1056px)").matches) {
              setIsSideNavVisible(false);
            }
          }}
        />

        {isSideNavVisible ? (
          <button
            type="button"
            className="admin-layout__sidenav-backdrop"
            aria-label="Close navigation"
            onClick={() => setIsSideNavVisible(false)}
          />
        ) : null}

        <div className="admin-layout__content-shell">
          <RouteBreadcrumbBar />

          <Content id="main-content" className="admin-layout__content">
            <Outlet />
          </Content>
        </div>
        <PublicFooter
          frontendVersion={versionInfo.frontend.version}
          backendVersion={versionInfo.backend?.version}
          backendUnavailable={!versionInfo.isLoading && !versionInfo.backend}
        />
      </div>
    </HeaderPanelProvider>
  );
}
