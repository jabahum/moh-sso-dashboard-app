import type { SystemNavigationItem } from "@moh-sso/types";

import type { ApplicationLifecycleStatus, RbacPermission, RbacSystem } from "../../types";

/** Reads the backend's error envelope, falling back to a readable message. */
export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (
    typeof error === "object" &&
    error !== null &&
    "data" in error &&
    typeof (error as { data?: unknown }).data === "object" &&
    (error as { data?: unknown }).data !== null
  ) {
    const data = (error as { data: { error?: { message?: string }; message?: string } }).data;
    return data.error?.message || data.message || fallback;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return fallback;
}

const acronyms: Record<string, string> = { rbac: "RBAC", api: "API", sso: "SSO", id: "ID" };

function words(value: string) {
  return value
    .replace(/[_-]+/g, " ")
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => acronyms[word.toLowerCase()] ?? word.toLowerCase());
}

function sentence(value: string) {
  const text = words(value).join(" ");
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** "data_quality" → "Data quality". */
export function humanizeCategory(category: string) {
  return sentence(category) || "Other";
}

const actionVerbs: Record<string, string> = {
  read: "View",
  write: "Edit",
  comment: "Comment on",
  execute: "Run",
};

/**
 * A readable permission name: the catalog's display name when it has one,
 * otherwise derived from the key ("announcements:publish" → "Publish announcements").
 */
export function permissionLabel(permission: RbacPermission) {
  if (permission.displayName?.trim()) return permission.displayName.trim();
  const segments = permission.key.split(":").filter(Boolean);
  if (segments.length < 2) return sentence(permission.key) || permission.key;
  const action = segments[segments.length - 1];
  const subject = segments
    .slice(0, -1)
    .flatMap((segment) => words(segment))
    .join(" ");
  // Multi-word actions ("manage_alerts") already name their object.
  if (/[_-]/.test(action)) return sentence(action);
  if (action === "history") return `View ${subject} history`;
  const verb = actionVerbs[action] ?? sentence(action);
  return `${verb} ${subject}`;
}

export function permissionCategory(permission: RbacPermission) {
  return permission.category || permission.key.split(":")[0] || "other";
}

export type PermissionCategory = {
  id: string;
  label: string;
  permissions: RbacPermission[];
};

/** Groups the catalog by category, sorted by readable name. */
export function groupPermissions(permissions: RbacPermission[]): PermissionCategory[] {
  const groups = new Map<string, RbacPermission[]>();
  for (const permission of permissions) {
    const category = permissionCategory(permission);
    groups.set(category, [...(groups.get(category) ?? []), permission]);
  }
  return [...groups.entries()]
    .map(([id, items]) => ({
      id,
      label: humanizeCategory(id),
      permissions: [...items].sort((a, b) => a.key.localeCompare(b.key)),
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
}

export const lifecycleLabels: Record<ApplicationLifecycleStatus, string> = {
  registered: "Registered",
  configured: "Configured",
  validated: "Validated",
  approved: "Approved",
  active: "Active",
  suspended: "Suspended",
  retired: "Retired",
};

export const lifecycleTagType: Record<
  ApplicationLifecycleStatus,
  "gray" | "blue" | "teal" | "green" | "red" | "warm-gray" | "purple"
> = {
  registered: "gray",
  configured: "blue",
  validated: "teal",
  approved: "purple",
  active: "green",
  suspended: "red",
  retired: "warm-gray",
};

export type SystemDraft = {
  integrationType: "external" | "hosted_module";
  contractVersion: string;
  displayName: string;
  description: string;
  icon: string;
  launchUrl: string;
  authenticatedLaunchUrl: string;
  category: string;
  ownerTeam: string;
  ownerName: string;
  ownerEmail: string;
  supportUrl: string;
  documentationUrl: string;
  environment: string;
  criticality: string;
  navigation: string;
  systemType: "platform" | "external";
  displayInLauncher: boolean;
  displayInSideNav: boolean;
  launchMode: "internal" | "new_tab" | "same_tab";
  enabled: boolean;
  sortOrder: number;
};

export function toDraft(system?: RbacSystem): SystemDraft {
  return {
    integrationType: system?.integrationType ?? "hosted_module",
    contractVersion: system?.contractVersion ?? "1",
    displayName: system?.displayName ?? "",
    description: system?.description ?? "",
    icon: system?.icon ?? "",
    launchUrl: system?.launchUrl ?? "",
    authenticatedLaunchUrl: system?.authenticatedLaunchUrl ?? "",
    category: system?.category ?? "",
    ownerTeam: system?.ownerTeam ?? "",
    ownerName: system?.ownerName ?? "",
    ownerEmail: system?.ownerEmail ?? "",
    supportUrl: system?.supportUrl ?? "",
    documentationUrl: system?.documentationUrl ?? "",
    environment: system?.environment ?? "",
    criticality: system?.criticality ?? "",
    navigation: system?.navigation ?? "",
    systemType: system?.systemType ?? "platform",
    displayInLauncher: system?.displayInLauncher ?? true,
    displayInSideNav: system?.displayInSideNav ?? true,
    launchMode: system?.launchMode ?? "internal",
    enabled: system?.enabled ?? true,
    sortOrder: system?.sortOrder ?? 0,
  };
}

export type NavigationPreview = {
  error?: string;
  launcherItems: SystemNavigationItem[];
  sideNavItems: SystemNavigationItem[];
};

export function parseNavigationPreview(rawNavigation: string): NavigationPreview {
  if (!rawNavigation.trim()) {
    return { launcherItems: [], sideNavItems: [] };
  }

  try {
    const parsed: unknown = JSON.parse(rawNavigation);

    if (!Array.isArray(parsed)) {
      return { error: "Navigation must be a JSON array.", launcherItems: [], sideNavItems: [] };
    }

    const launcherItems: SystemNavigationItem[] = [];
    const sideNavItems: SystemNavigationItem[] = [];
    const ids = new Set<string>();

    const visit = (items: unknown[]) => {
      for (const value of items) {
        if (!value || typeof value !== "object") {
          throw new Error("Each navigation item must be an object.");
        }

        const item = value as SystemNavigationItem;
        const id = item.id?.trim();
        const label = item.label?.trim();

        if (!id || !label) {
          throw new Error("Every navigation item requires a non-empty id and label.");
        }
        if (ids.has(id)) {
          throw new Error(`Navigation item id "${id}" is duplicated.`);
        }
        ids.add(id);

        if (item.displayInLauncher === true && !item.path?.trim()) {
          throw new Error(`Launcher item "${label}" requires a path.`);
        }

        if (item.displayInLauncher === true) launcherItems.push(item);
        if (item.displayInSideNav !== false) sideNavItems.push(item);
        if (item.children?.length) visit(item.children);
      }
    };

    visit(parsed);
    return { launcherItems, sideNavItems };
  } catch (navigationError) {
    return {
      error:
        navigationError instanceof Error ? navigationError.message : "Navigation JSON is invalid.",
      launcherItems: [],
      sideNavItems: [],
    };
  }
}

export function pluralize(count: number, singular: string, plural = `${singular}s`) {
  return `${count} ${count === 1 ? singular : plural}`;
}
