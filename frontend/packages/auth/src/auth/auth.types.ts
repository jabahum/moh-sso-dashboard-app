export interface AuthUser {
  id: string;
  username: string;
  email?: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
  isAdmin: boolean;
  isUser: boolean;
  realmRoles: string[];
  clientRoles: Record<string, string[]>;
  permissions: Permission[];
  systems: System[];
  accessibleSystems: SystemAccess[];
  enabled: boolean;
  emailVerified: boolean;
  requirePwdChange: boolean;
  lastLoginAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface SystemAccess {
  clientId: string;
  displayName: string;
  launchUrl?: string;
  authenticatedLaunchUrl?: string;
  icon?: string;
  category?: string;
  navigation?: string;
  systemType?: "platform" | "external";
  displayInLauncher?: boolean;
  displayInSideNav?: boolean;
  launchMode?: "internal" | "new_tab" | "same_tab";
  sortOrder?: number;
  roles: string[];
}

export type Role = string;

export type System = string;

export type Permission =
  | "portal:access"
  | "systems:read"
  | "systems:launch"
  | "users:read"
  | "users:write"
  | "users:roles:write"
  | "clients:read"
  | "clients:write"
  | "clients:roles:write"
  | "announcements:read"
  | "announcements:write"
  | "announcements:publish"
  | "documents:read"
  | "documents:write"
  | "documents:process"
  | "document_templates:read"
  | "document_templates:write"
  | "document_templates:publish"
  | "surveillance:read"
  | "surveillance:import"
  | "surveillance:manage_locations"
  | "surveillance:manage_alerts"
  | "email:read"
  | "email:send"
  | "email:manage"
  | "storage_locations:read"
  | "storage_locations:write"
  | "data_quality:read"
  | "data_quality:write"
  | "data_quality:resolve"
  | "issue_tracker:resolve"
  | "report_browser:read"
 | "report_scheduler:read"
  | "outbreak:access"
  | "outbreak:manage"
  | "metrics:read"
  | "audit:read"
  | "notifications:read"
  | "notifications:write"
  | "rbac:read"
  | "rbac:write"
  | "rbac:roles:write"
  | "rbac:permissions:write"
  | "issue_tracker:read"
  | "issue_tracker:write"
  | "issue_tracker:manage"
  | "issue_tracker:assign"
  | "issue_tracker:close"
  | "issue_tracker:reopen"
  | "issue_tracker:comment";
