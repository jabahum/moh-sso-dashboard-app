import { useMemo, useRef } from "react";
import { SideNav, SideNavItems, SideNavLink, SideNavMenu } from "@carbon/react";
import { ChevronLeft, ChevronRight } from "@carbon/react/icons";

import type { Client, SystemNavigationItem } from "@moh-sso/types";
import { useFocusTrap } from "../../accessibility";

type SideNavItem = SystemNavigationItem;

/* --------------------------------
 * Helpers
 * -------------------------------- */

function normalizePath(path?: string): string {
  if (!path) {
    return "";
  }

  const trimmed = path.trim();

  if (!trimmed) {
    return "";
  }

  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function isActivePath(currentPath: string, itemPath?: string): boolean {
  const normalizedItemPath = normalizePath(itemPath);

  if (!normalizedItemPath) {
    return false;
  }

  if (currentPath === normalizedItemPath) {
    return true;
  }

  const currentWithoutQuery = currentPath.split("?")[0];
  const itemWithoutQuery = normalizedItemPath.split("?")[0];

  if (normalizedItemPath.includes("?")) {
    return currentPath === normalizedItemPath;
  }

  return currentWithoutQuery === itemWithoutQuery || currentWithoutQuery.startsWith(`${itemWithoutQuery}/`);
}

function sortItems(items: SideNavItem[]): SideNavItem[] {
  return [...items].sort((a, b) => {
    const aHasOrder = typeof a.order === "number";
    const bHasOrder = typeof b.order === "number";
    const aOrder = aHasOrder ? a.order! : Number.MAX_SAFE_INTEGER;
    const bOrder = bHasOrder ? b.order! : Number.MAX_SAFE_INTEGER;

    if (aOrder !== bOrder) {
      return aOrder - bOrder;
    }

    if (!aHasOrder && !bHasOrder) {
      return 0;
    }

    return a.label.localeCompare(b.label);
  });
}

function getClientOrder(client: Client): number {
  const order = Number(client.attributes?.["ui.order"]);

  return Number.isFinite(order) ? order : Number.MAX_SAFE_INTEGER;
}

function isValidSideNavItem(value: unknown): value is SideNavItem {
  if (!value || typeof value !== "object") {
    return false;
  }

  const item = value as Partial<SideNavItem>;

  return typeof item.id === "string" && typeof item.label === "string";
}

function normalizeLegacySideNavItems(items: SideNavItem[]): SideNavItem[] {
  const normalizedItems = items
    .filter(
      (item) => item.id !== "data-exports" && normalizePath(item.path) !== "/apps/dwh/exports",
    )
    .map((item) => {
      const children = item.children ? normalizeLegacySideNavItems(item.children) : undefined;
      const itemPath = normalizePath(item.path);

      if (
        itemPath === "/apps/dwh" ||
        itemPath === "/apps/dwh/reports" ||
        itemPath === "/apps/dwh/dashboards"
      ) {
        return {
          ...item,
          id: "dashboards",
          label: "Dashboards",
          path: "/apps/dwh/dashboards",
          children,
        };
      }

      if (itemPath === "/apps/dwh/filesvr") {
        return {
          ...item,
          id: item.id === "file-svr" ? "documents" : item.id,
          label: item.label === "File Upload" ? "Documents" : item.label,
          path: "/apps/dwh/documents",
          children,
        };
      }

      if (
        itemPath === "/apps/dwh/issue-tracker?view=dashboard" ||
        item.id === "issue-dashboard" ||
        item.id === "issue-summary"
      ) {
        return {
          ...item,
          id: "issue-summary",
          label: "Issue Summary",
          path: "/apps/dwh/issue-tracker/dashboard",
        };
      }

      if (
        itemPath === "/apps/dwh/issue-tracker?view=issues" ||
        item.id === "registered-issues"
      ) {
        return {
          ...item,
          id: "registered-issues",
          label: item.label || "Registered Issues",
          path: "/apps/dwh/issue-tracker/issues",
        };
      }

      if (itemPath === "/apps/dwh/issue-tracker" || item.id === "issue-tracker" || item.id === "issue-tracking") {
        return {
          ...item,
          id: "issue-tracking",
          label: "Issue Tracking",
          path: undefined,
          children: children && children.length > 0 ? children : [
            {
              id: "issue-summary",
              label: "Issue Summary",
              path: "/apps/dwh/issue-tracker/dashboard",
              permission: item.permission ?? "issue_tracker:read",
            },
            {
              id: "registered-issues",
              label: "Registered Issues",
              path: "/apps/dwh/issue-tracker/issues",
              permission: item.permission ?? "issue_tracker:read",
            },
          ],
        };
      }

      return {
        ...item,
        children,
      };
    });

  const seen = new Set<string>();

  return normalizedItems.filter((item) => {
    const key = normalizePath(item.path) || item.id;

    if (seen.has(key)) {
      return false;
    }

    seen.add(key);
    return true;
  });
}

function parseSideNav(client: Client): SideNavItem[] {
  const rawSideNav = client.attributes?.["ui.sidenav"];

  if (!rawSideNav) {
    return [];
  }

  try {
    const parsed = JSON.parse(rawSideNav);

    if (!Array.isArray(parsed)) {
      return [];
    }

    return normalizeLegacySideNavItems(parsed.filter(isValidSideNavItem));
  } catch {
    return [];
  }
}

function isPlatformSideNavClient(client: Client): boolean {
  const systemType = client.attributes?.["ui.systemType"];
  const displayInSideNav = client.attributes?.["ui.displayInSideNav"];
  const navigation = parseSideNav(client);

  if (systemType === "external") {
    return false;
  }

  if (displayInSideNav === "true") {
    return navigation.length > 0;
  }

  if (displayInSideNav === "false") {
    return false;
  }

  return navigation.length > 0;
}

function hasActiveChild(item: SideNavItem, currentPath: string): boolean {
  if (isActivePath(currentPath, item.path)) {
    return true;
  }

  return item.children?.some((child) => hasActiveChild(child, currentPath)) ?? false;
}

function filterByPermission(
  items: SideNavItem[],
  hasPermission?: (permission: string) => boolean,
): SideNavItem[] {
  return items
    .filter((item) => {
      if (item.displayInSideNav === false) {
        return false;
      }

      if (!item.permission) {
        return true;
      }

      return hasPermission?.(item.permission) ?? true;
    })
    .filter((item) => {
      if (!hasPermission) {
        return true;
      }

      if (item.requiredPermissions?.some((permission) => !hasPermission(permission))) {
        return false;
      }

      if (
        item.requiredAnyPermissions?.length &&
        !item.requiredAnyPermissions.some((permission) => hasPermission(permission))
      ) {
        return false;
      }

      return true;
    })
    .map((item) => ({
      ...item,
      children: item.children ? filterByPermission(item.children, hasPermission) : undefined,
    }))
    .filter((item) => {
      const hasChildren = item.children && item.children.length > 0;
      const hasPath = Boolean(normalizePath(item.path));

      return hasPath || hasChildren;
    });
}

function getClientLabel(client: Client): string {
  return client.name || client.clientId || "Application";
}

/* --------------------------------
 * Recursive item
 * -------------------------------- */

type RenderSideNavItemProps = {
  item: SideNavItem;
  currentPath: string;
  onNavigate: (path: string) => void;
};

function RenderSideNavItem({ item, currentPath, onNavigate }: RenderSideNavItemProps) {
  const children = sortItems(item.children ?? []);
  const hasChildren = children.length > 0;
  const itemPath = normalizePath(item.path);
  const active = isActivePath(currentPath, itemPath);
  const childActive = hasActiveChild(item, currentPath);

  if (hasChildren) {
    return (
      <SideNavMenu
        key={item.id}
        title={item.label}
        defaultExpanded={childActive}
        isActive={childActive}
      >
        {children.map((child) => (
          <RenderSideNavItem
            key={child.id}
            item={child}
            currentPath={currentPath}
            onNavigate={onNavigate}
          />
        ))}
      </SideNavMenu>
    );
  }

  if (!itemPath) {
    return null;
  }

  return (
    <SideNavLink
      key={item.id}
      isActive={active}
      aria-current={active ? "page" : undefined}
      onClick={() => onNavigate(itemPath)}
    >
      {item.label}
    </SideNavLink>
  );
}

/* --------------------------------
 * Component
 * -------------------------------- */

type ClientSideNavProps = {
  clients?: Client[];
  activeClient?: Client | null;
  currentPath: string;
  onNavigate: (path: string) => void;
  hasPermission?: (permission: string) => boolean;
  visible?: boolean;
  onToggleVisibility?: () => void;
};

export function ClientSideNav({
  clients = [],
  activeClient,
  currentPath,
  onNavigate,
  hasPermission,
  visible = true,
  onToggleVisibility,
}: ClientSideNavProps) {
  const navRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const shouldTrapFocus =
    visible && typeof window !== "undefined" && window.matchMedia("(max-width: 1056px)").matches;

  const navClients = useMemo(() => {
    return clients
      .filter(isPlatformSideNavClient)
      .map((client) => {
        const items = filterByPermission(parseSideNav(client), hasPermission);

        return {
          client,
          items: sortItems(items),
        };
      })
      .filter(({ items }) => items.length > 0)
      .sort((a, b) => {
        const orderDiff = getClientOrder(a.client) - getClientOrder(b.client);

        if (orderDiff !== 0) {
          return orderDiff;
        }

        return getClientLabel(a.client).localeCompare(getClientLabel(b.client));
      });
  }, [clients, hasPermission]);

  useFocusTrap({
    active: shouldTrapFocus,
    containerRef: navRef,
    initialFocusRef: closeButtonRef,
  });

  if (navClients.length === 0) {
    return null;
  }

  const handleNavigate = (path: string) => {
    if (currentPath === path) {
      return;
    }

    onNavigate(path);
  };

  return (
    <>
      {!visible && onToggleVisibility ? (
        <button
          type="button"
          className="moh-client-sidenav__toggle moh-client-sidenav__toggle--rail"
          aria-label="Expand navigation"
          title="Expand navigation"
          aria-controls="user-sidenav"
          aria-expanded={false}
          onClick={onToggleVisibility}
        >
          <ChevronRight size={20} aria-hidden="true" />
        </button>
      ) : null}

      <SideNav
        ref={navRef}
        id="user-sidenav"
        isFixedNav
        expanded
        aria-hidden={!visible}
        aria-label="Application navigation"
        className="moh-client-sidenav"
      >
        <SideNavItems>
          <div className="moh-client-sidenav__header">
            <div className="moh-client-sidenav__heading">
              <span className="moh-client-sidenav__kicker">Applications</span>
              <span className="moh-client-sidenav__title">Workspace</span>
            </div>

            {onToggleVisibility ? (
              <button
                ref={closeButtonRef}
                type="button"
                className="moh-client-sidenav__toggle"
                aria-label="Collapse navigation"
                title="Collapse navigation"
                aria-controls="user-sidenav"
                aria-expanded={visible}
                onClick={onToggleVisibility}
              >
                <ChevronLeft size={20} aria-hidden="true" />
              </button>
            ) : null}
          </div>

          {navClients.map(({ client, items }) => {
            const clientActive =
              client.clientId === activeClient?.clientId ||
              items.some((item) => hasActiveChild(item, currentPath));

            return (
              <SideNavMenu
                key={client.clientId}
                title={getClientLabel(client)}
                defaultExpanded={clientActive}
                isActive={clientActive}
              >
                {items.map((item) => (
                  <RenderSideNavItem
                    key={item.id}
                    item={item}
                    currentPath={currentPath}
                    onNavigate={handleNavigate}
                  />
                ))}
              </SideNavMenu>
            );
          })}
        </SideNavItems>
      </SideNav>
    </>
  );
}

export function hasVisibleClientSideNav(
  clients: Client[],
  hasPermission?: (permission: string) => boolean,
): boolean {
  return clients.some((client) => {
    if (!isPlatformSideNavClient(client)) {
      return false;
    }
    return filterByPermission(parseSideNav(client), hasPermission).length > 0;
  });
}
