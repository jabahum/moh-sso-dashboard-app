import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  Button,
  Checkbox,
  ComboBox,
  DataTable,
  Dropdown,
  InlineLoading,
  Search,
  InlineNotification,
  Tag,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  TextArea,
  TextInput,
  Toggle,
} from "@carbon/react";
import { Add, Document, Folder, FolderOpen, Renew, Save, TrashCan } from "@carbon/react/icons";

import {
  useAddRbacGroupMemberMutation,
  useAddSystemAccessRoleMutation,
  useAssignGroupPermissionMutation,
  useAssignGroupRealmRoleMutation,
  useAssignGroupSystemRoleMutation,
  useAssignRealmRolePermissionMutation,
  useAssignSystemRolePermissionMutation,
  useCreateSystemRoleMutation,
  useDeleteSystemRoleMutation,
  useGetRbacSystemQuery,
  useListRbacGroupMembersQuery,
  useListRbacGroupsQuery,
  useListRbacPermissionsQuery,
  useListRbacSystemsQuery,
  useListRealmRolePermissionsQuery,
  usePreviewRbacChangeMutation,
  useRemoveGroupPermissionMutation,
  useRemoveRbacGroupMemberMutation,
  useRemoveGroupRealmRoleMutation,
  useRemoveGroupSystemRoleMutation,
  useRemoveRealmRolePermissionMutation,
  useRemoveSystemAccessRoleMutation,
  useRemoveSystemRolePermissionMutation,
  useSyncRbacGroupMembersMutation,
  useUpdateRbacSystemMutation,
  useRegisterRbacSystemMutation,
  useTransitionRbacSystemMutation,
  useUpsertRbacGroupMutation,
} from "../api";
import { PERMISSIONS, PermissionGuard, useAuthorization } from "@moh-sso/auth";
import { useListUsersQuery, type User } from "@moh-sso/users";
import { useModal, useToast } from "@moh-sso/ui";
import type { SystemNavigationItem } from "@moh-sso/types";
import type {
  ApplicationLifecycleStatus,
  RbacGroup,
  RbacGroupMember,
  RbacPermission,
  RbacSystem,
  RbacSystemRole,
} from "../types";

import { EffectiveAccessPanel } from "../components/EffectiveAccessPanel";
import { GovernanceToolsPanel } from "../components/GovernanceToolsPanel";
import { SyncDriftPanel } from "../components/SyncDriftPanel";
import "./rbac-management.page.scss";

const workspaceSections = [
  {
    id: "applications",
    label: "Application settings",
    description: "Configure application identity, ownership, integration, and launch settings.",
  },
  {
    id: "lifecycle",
    label: "Application lifecycle",
    description: "Validate, approve, activate, or suspend the selected application.",
  },
  {
    id: "access-roles",
    label: "Access roles",
    description: "Manage the roles that grant access to the selected application.",
  },
  {
    id: "system-roles",
    label: "System roles",
    description: "Map application roles to permissions.",
  },
  {
    id: "realm-roles",
    label: "Realm permissions",
    description: "Manage permissions granted by realm roles.",
  },
  { id: "groups", label: "Groups", description: "Manage group membership and inherited access." },
  {
    id: "effective-access",
    label: "Effective access",
    description: "Explain a user's permissions and how they were granted.",
  },
  {
    id: "sync",
    label: "Sync & drift",
    description: "Review differences between Keycloak and portal access mappings.",
  },
  {
    id: "governance",
    label: "Governance tools",
    description: "Review requests, audit changes, and manage authorization policy.",
  },
] as const;
type WorkspaceSection = (typeof workspaceSections)[number]["id"];
const workspaceFolders: { id: string; label: string; sections: WorkspaceSection[] }[] = [
  { id: "applications-folder", label: "Applications", sections: ["applications", "lifecycle"] },
  {
    id: "roles-folder",
    label: "Roles & permissions",
    sections: ["access-roles", "system-roles", "realm-roles"],
  },
  { id: "access-folder", label: "Groups & access", sections: ["groups", "effective-access"] },
  { id: "operations-folder", label: "Operations", sections: ["sync", "governance"] },
];

const applicationSections = [
  { id: "basics", label: "Basics" },
  { id: "ownership", label: "Ownership & support" },
  { id: "integration", label: "Integration & environment" },
  { id: "launch", label: "Launch & visibility" },
  { id: "navigation", label: "Navigation" },
] as const;
type ApplicationSection = (typeof applicationSections)[number]["id"];

const lifecycleActions: Record<ApplicationLifecycleStatus, ApplicationLifecycleStatus[]> = {
  registered: ["configured", "retired"],
  configured: ["validated", "retired"],
  validated: ["approved", "configured", "retired"],
  approved: ["active", "configured", "retired"],
  active: ["suspended", "retired"],
  suspended: ["configured", "retired"],
  retired: [],
};

type SystemDraft = {
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

type NavigationPreview = {
  error?: string;
  launcherItems: SystemNavigationItem[];
  sideNavItems: SystemNavigationItem[];
};

function parseNavigationPreview(rawNavigation: string): NavigationPreview {
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

function permissionLabel(permission: RbacPermission) {
  return permission.displayName || permission.key;
}

function permissionCategory(permission: RbacPermission) {
  return permission.category || permission.key.split(":")[0] || "other";
}

function getApiErrorMessage(error: unknown, fallback: string): string {
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

function toDraft(system?: RbacSystem): SystemDraft {
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

export default function RbacManagementPage() {
  const workspaceRef = useRef<HTMLElement>(null);
  const [activeSection, setActiveSection] = useState<WorkspaceSection>("applications");
  const [expandedFolders, setExpandedFolders] = useState<string[]>(["applications-folder"]);
  const [applicationSection, setApplicationSection] = useState<ApplicationSection>("basics");
  const currentSection = workspaceSections.find((section) => section.id === activeSection)!;
  useEffect(() => {
    const folder = workspaceFolders.find((item) => item.sections.includes(activeSection));
    if (folder)
      setExpandedFolders((current) =>
        current.includes(folder.id) ? current : [...current, folder.id],
      );
  }, [activeSection]);
  useEffect(() => {
    if (workspaceRef.current) workspaceRef.current.scrollTop = 0;
  }, [activeSection, applicationSection]);
  const { can } = useAuthorization();
  const { openModal, closeModal } = useModal();
  const toast = useToast();
  const { data: systems = [], isLoading: systemsLoading } = useListRbacSystemsQuery();
  const { data: permissions = [], isLoading: permissionsLoading } = useListRbacPermissionsQuery();
  const { data: realmRoles = [] } = useListRealmRolePermissionsQuery();
  const { data: groups = [] } = useListRbacGroupsQuery();
  const { data: users = [] } = useListUsersQuery();
  const [selectedClientId, setSelectedClientId] = useState<string>("");
  const [registeringApplication, setRegisteringApplication] = useState(false);
  const activeClientId = registeringApplication
    ? ""
    : selectedClientId || systems[0]?.clientId || "";
  const { currentData: systemDetail, isFetching: systemLoading } = useGetRbacSystemQuery(
    activeClientId,
    {
      skip: !activeClientId,
    },
  );

  const [draft, setDraft] = useState<SystemDraft>(() => toDraft());
  const [draftClientId, setDraftClientId] = useState<string>("");
  const systemReady =
    registeringApplication || Boolean(systemDetail && draftClientId === activeClientId);
  const [newRoleName, setNewRoleName] = useState("");
  const [newRoleDisplayName, setNewRoleDisplayName] = useState("");
  const [newAccessRole, setNewAccessRole] = useState("");
  const [realmRoleName, setRealmRoleName] = useState("");
  const [selectedRoleId, setSelectedRoleId] = useState<string>("");
  const [selectedRealmRole, setSelectedRealmRole] = useState<string>("");
  const [groupSection, setGroupSection] = useState("Members");
  const [groupSearch, setGroupSearch] = useState("");
  const [selectedGroupId, setSelectedGroupId] = useState<string>("");
  const [newGroupPath, setNewGroupPath] = useState("");
  const [newGroupDisplayName, setNewGroupDisplayName] = useState("");
  const [groupRealmRoleName, setGroupRealmRoleName] = useState("");
  const [groupSystemClientId, setGroupSystemClientId] = useState("");
  const [groupSystemRoleName, setGroupSystemRoleName] = useState("");
  const [selectedGroupMemberUserId, setSelectedGroupMemberUserId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const [registerSystem, { isLoading: registeringSystem }] = useRegisterRbacSystemMutation();
  const [transitionSystem, { isLoading: transitioningSystem }] = useTransitionRbacSystemMutation();
  const [lifecycleReason, setLifecycleReason] = useState("");
  const [updateSystem, { isLoading: savingSystem }] = useUpdateRbacSystemMutation();
  const [createRole, { isLoading: creatingRole }] = useCreateSystemRoleMutation();
  const [deleteRole] = useDeleteSystemRoleMutation();
  const [assignRolePermission] = useAssignSystemRolePermissionMutation();
  const [removeRolePermission] = useRemoveSystemRolePermissionMutation();
  const [assignRealmPermission] = useAssignRealmRolePermissionMutation();
  const [removeRealmPermission] = useRemoveRealmRolePermissionMutation();
  const [addAccessRole] = useAddSystemAccessRoleMutation();
  const [removeAccessRole] = useRemoveSystemAccessRoleMutation();
  const [previewChange] = usePreviewRbacChangeMutation();
  const [upsertGroup, { isLoading: creatingGroup }] = useUpsertRbacGroupMutation();
  const [assignGroupPermission] = useAssignGroupPermissionMutation();
  const [removeGroupPermission] = useRemoveGroupPermissionMutation();
  const [assignGroupRealmRole] = useAssignGroupRealmRoleMutation();
  const [removeGroupRealmRole] = useRemoveGroupRealmRoleMutation();
  const [assignGroupSystemRole] = useAssignGroupSystemRoleMutation();
  const [removeGroupSystemRole] = useRemoveGroupSystemRoleMutation();
  const [addGroupMember, { isLoading: addingGroupMember }] = useAddRbacGroupMemberMutation();
  const [removeGroupMember, { isLoading: removingGroupMember }] =
    useRemoveRbacGroupMemberMutation();
  const [syncGroupMembers, { isLoading: syncingGroupMembers }] = useSyncRbacGroupMembersMutation();

  const canWriteSystems = can(PERMISSIONS.rbacWrite);
  const canWriteRoles = can(PERMISSIONS.rbacRolesWrite);
  const canWritePermissions = can(PERMISSIONS.rbacPermissionsWrite);
  const navigationPreview = useMemo(
    () => parseNavigationPreview(draft.navigation),
    [draft.navigation],
  );

  useEffect(() => {
    if (registeringApplication || !systemDetail || draftClientId === systemDetail.clientId) {
      return;
    }

    setDraftClientId(systemDetail.clientId);
    setDraft(toDraft(systemDetail));
  }, [draftClientId, systemDetail, registeringApplication]);

  const selectedRole = useMemo(
    () => systemDetail?.roles.find((role) => role.id === selectedRoleId) ?? systemDetail?.roles[0],
    [selectedRoleId, systemDetail?.roles],
  );

  const selectedRealmRoleGroup = useMemo(
    () =>
      realmRoles.find((group) => group.realmRole === selectedRealmRole) ??
      realmRoles.find((group) => group.realmRole === realmRoleName),
    [realmRoleName, realmRoles, selectedRealmRole],
  );

  const selectedGroup = useMemo(
    () => groups.find((group) => group.id === selectedGroupId) ?? groups[0],
    [groups, selectedGroupId],
  );
  const selectedGroupIdForMembers = selectedGroup?.id ?? "";
  const {
    data: groupMembers = [],
    isLoading: groupMembersLoading,
    isFetching: groupMembersFetching,
  } = useListRbacGroupMembersQuery(selectedGroupIdForMembers, {
    skip: !selectedGroupIdForMembers,
  });

  const groupedPermissions = useMemo(() => {
    return permissions.reduce<Record<string, RbacPermission[]>>((groups, permission) => {
      const category = permissionCategory(permission);
      groups[category] = [...(groups[category] ?? []), permission];
      return groups;
    }, {});
  }, [permissions]);

  const rolePermissionKeys = new Set(
    selectedRole?.permissions.map((permission) => permission.key) ?? [],
  );
  const realmPermissionKeys = new Set(
    selectedRealmRoleGroup?.permissions.map((permission) => permission.key) ?? [],
  );
  const groupPermissionKeys = new Set(
    selectedGroup?.permissions.map((permission) => permission.key) ?? [],
  );
  const groupMemberUserIds = useMemo(
    () => new Set(groupMembers.map((member) => member.userId)),
    [groupMembers],
  );
  const assignableGroupUsers = useMemo(
    () => users.filter((user) => !groupMemberUserIds.has(user.id)),
    [groupMemberUserIds, users],
  );
  const groupMemberRows = useMemo(
    () =>
      groupMembers.map((member) => ({
        id: member.userId,
        username: member.username || "—",
        email: member.email || "—",
        userId: member.userId,
      })),
    [groupMembers],
  );

  const handleSelectSystem = (clientId: string) => {
    const system = systems.find((item) => item.clientId === clientId);
    setRegisteringApplication(false);
    setSelectedClientId(clientId);
    setLifecycleReason("");
    setSelectedRoleId("");
    setDraftClientId(clientId);
    setDraft(toDraft(system));
  };

  const handleSaveSystem = async () => {
    const clientId = draftClientId || activeClientId;
    if (!clientId) return;
    if (navigationPreview.error) {
      setActiveSection("applications");
      setApplicationSection("navigation");
      setError(navigationPreview.error);
      return;
    }
    try {
      setError(null);
      const result = registeringApplication
        ? await registerSystem({ clientId, ...draft }).unwrap()
        : await updateSystem({ clientId, data: draft }).unwrap();
      setRegisteringApplication(false);
      setSelectedClientId(result.clientId);
      setDraftClientId(result.clientId);
      setDraft(toDraft(result));
    } catch {
      setError("Unable to save system metadata.");
    }
  };

  const handleTransitionSystem = async (status: ApplicationLifecycleStatus) => {
    if (!activeClientId || !lifecycleReason.trim()) return;
    try {
      setError(null);
      const result = await transitionSystem({
        clientId: activeClientId,
        status,
        reason: lifecycleReason.trim(),
      }).unwrap();
      setDraft(toDraft(result));
      setLifecycleReason("");
    } catch (failure) {
      const message = (failure as { data?: { error?: { message?: string } } })?.data?.error
        ?.message;
      setError(
        message ||
          "Unable to change application status. Check saved metadata, access roles, and permissions.",
      );
    }
  };

  const handleCreateRole = async () => {
    if (!activeClientId || !newRoleName.trim()) return;
    try {
      setError(null);
      await createRole({
        clientId: activeClientId,
        data: { name: newRoleName, displayName: newRoleDisplayName, enabled: true },
      }).unwrap();
      setNewRoleName("");
      setNewRoleDisplayName("");
    } catch {
      setError("Unable to create system role.");
    }
  };

  const handleDeleteRole = async (role: RbacSystemRole) => {
    const preview = await previewChange({
      action: "delete-role",
      resourceType: "system-role",
      resourceId: role.id,
      systemClientId: role.clientId,
      roleName: role.name,
    }).unwrap();
    const warning = preview.warnings.length ? `\n\n${preview.warnings.join("\n")}` : "";
    if (!confirm(`Delete role "${role.name}"? Risk: ${preview.riskLevel}.${warning}`)) return;
    try {
      setError(null);
      await deleteRole({ roleId: role.id, clientId: role.clientId }).unwrap();
    } catch {
      setError("Unable to delete system role.");
    }
  };

  const toggleRolePermission = async (permission: RbacPermission, checked: boolean) => {
    if (!selectedRole) return;
    try {
      setError(null);
      if (checked) {
        await assignRolePermission({
          roleId: selectedRole.id,
          clientId: selectedRole.clientId,
          data: { permissionKey: permission.key },
        }).unwrap();
      } else {
        await removeRolePermission({
          roleId: selectedRole.id,
          clientId: selectedRole.clientId,
          permissionKey: permission.key,
        }).unwrap();
      }
    } catch {
      setError("Unable to update role permissions.");
    }
  };

  const toggleRealmPermission = async (permission: RbacPermission, checked: boolean) => {
    const realmRole = selectedRealmRole || realmRoleName.trim();
    if (!realmRole) return;
    try {
      setError(null);
      if (checked) {
        await assignRealmPermission({
          realmRole,
          data: { permissionKey: permission.key },
        }).unwrap();
      } else {
        await removeRealmPermission({ realmRole, permissionKey: permission.key }).unwrap();
      }
      setSelectedRealmRole(realmRole);
    } catch {
      setError("Unable to update realm role permissions.");
    }
  };

  const handleAddAccessRole = async () => {
    if (!activeClientId || !newAccessRole.trim()) return;
    try {
      setError(null);
      await addAccessRole({ clientId: activeClientId, data: { roleName: newAccessRole } }).unwrap();
      setNewAccessRole("");
    } catch {
      setError("Unable to add access role.");
    }
  };

  const handleRemoveAccessRole = async (roleName: string) => {
    if (!activeClientId) return;
    try {
      setError(null);
      await removeAccessRole({ clientId: activeClientId, roleName }).unwrap();
    } catch {
      setError("Unable to remove access role. Use backend force mode only after review.");
    }
  };

  const handleCreateGroup = async () => {
    const path = newGroupPath.trim();
    if (!path) return;
    try {
      setError(null);
      const group = await upsertGroup({
        path,
        displayName: newGroupDisplayName,
        enabled: true,
      }).unwrap();
      setSelectedGroupId(group.id);
      setNewGroupPath("");
      setNewGroupDisplayName("");
    } catch {
      setError("Unable to create or update group.");
    }
  };

  const toggleGroupPermission = async (permission: RbacPermission, checked: boolean) => {
    if (!selectedGroup) return;
    try {
      setError(null);
      if (checked) {
        await assignGroupPermission({
          groupId: selectedGroup.id,
          data: { permissionKey: permission.key },
        }).unwrap();
      } else {
        await removeGroupPermission({
          groupId: selectedGroup.id,
          permissionKey: permission.key,
        }).unwrap();
      }
    } catch {
      setError("Unable to update group permissions.");
    }
  };

  const handleAssignGroupRealmRole = async () => {
    if (!selectedGroup || !groupRealmRoleName.trim()) return;
    try {
      setError(null);
      await assignGroupRealmRole({
        groupId: selectedGroup.id,
        data: { realmRole: groupRealmRoleName },
      }).unwrap();
      setGroupRealmRoleName("");
    } catch {
      setError("Unable to assign group realm role.");
    }
  };

  const handleRemoveGroupRealmRole = async (realmRole: string) => {
    if (!selectedGroup) return;
    try {
      setError(null);
      await removeGroupRealmRole({ groupId: selectedGroup.id, realmRole }).unwrap();
    } catch {
      setError("Unable to remove group realm role.");
    }
  };

  const handleAssignGroupSystemRole = async () => {
    if (!selectedGroup || !groupSystemClientId.trim() || !groupSystemRoleName.trim()) return;
    try {
      setError(null);
      await assignGroupSystemRole({
        groupId: selectedGroup.id,
        data: { clientId: groupSystemClientId, roleName: groupSystemRoleName },
      }).unwrap();
      setGroupSystemClientId("");
      setGroupSystemRoleName("");
    } catch {
      setError("Unable to assign group system role.");
    }
  };

  const handleRemoveGroupSystemRole = async (
    group: RbacGroup,
    clientId: string,
    roleName: string,
  ) => {
    try {
      setError(null);
      await removeGroupSystemRole({ groupId: group.id, clientId, roleName }).unwrap();
    } catch {
      setError("Unable to remove group system role.");
    }
  };

  const notifyGroupMemberSync = (warnings?: string[]) => {
    if (warnings?.length) {
      toast.warning("Group updated with warnings", warnings.join(" "));
    }
  };

  const handleAddGroupMember = async () => {
    if (!selectedGroup || !selectedGroupMemberUserId) return;
    const user = users.find((item) => item.id === selectedGroupMemberUserId);

    try {
      setError(null);
      const result = await addGroupMember({
        groupId: selectedGroup.id,
        data: { userId: selectedGroupMemberUserId },
      }).unwrap();
      setSelectedGroupMemberUserId("");
      toast.success(
        "Group member added",
        `${user?.username ?? "User"} was added to ${selectedGroup.displayName || selectedGroup.name}.`,
      );
      notifyGroupMemberSync(result.warnings);
    } catch (err) {
      const message = getApiErrorMessage(err, "Unable to add this user to the group.");
      setError(message);
      toast.error("Unable to add group member", message);
    }
  };

  const handleRemoveGroupMember = (member: RbacGroupMember) => {
    if (!selectedGroup) return;

    openModal({
      title: "Remove group member",
      content: (
        <p>
          Remove <strong>{member.username || member.email || member.userId}</strong> from{" "}
          <strong>{selectedGroup.displayName || selectedGroup.name}</strong>?
        </p>
      ),
      primaryAction: {
        label: "Remove",
        kind: "danger",
        onClick: async () => {
          try {
            setError(null);
            const result = await removeGroupMember({
              groupId: selectedGroup.id,
              userId: member.userId,
            }).unwrap();
            closeModal();
            toast.success("Group member removed", "The user's inherited access was refreshed.");
            notifyGroupMemberSync(result.warnings);
          } catch (err) {
            const message = getApiErrorMessage(err, "Unable to remove this user from the group.");
            setError(message);
            toast.error("Unable to remove group member", message);
          }
        },
      },
      secondaryAction: {
        label: "Cancel",
        onClick: closeModal,
      },
      onClose: closeModal,
    });
  };

  const handleSyncGroupMembers = async () => {
    if (!selectedGroup) return;

    try {
      setError(null);
      const result = await syncGroupMembers(selectedGroup.id).unwrap();
      toast.success(
        "Group members synced",
        `${result.memberCount} member${result.memberCount === 1 ? "" : "s"} synced from Keycloak.`,
      );
      notifyGroupMemberSync(result.warnings);
    } catch (err) {
      const message = getApiErrorMessage(err, "Unable to sync group members from Keycloak.");
      setError(message);
      toast.error("Unable to sync group members", message);
    }
  };

  if (systemsLoading || permissionsLoading) {
    return <InlineLoading description="Loading RBAC mappings..." />;
  }

  return (
    <div className="rbac-page">
      <header className="rbac-page__header">
        <div>
          <h1>RBAC Management</h1>
          <p>Manage applications and access from one focused workspace.</p>
        </div>
      </header>

      {error && (
        <InlineNotification
          kind="error"
          lowContrast
          title="RBAC update failed"
          subtitle={error}
          onClose={() => setError(null)}
        />
      )}

      <div className="rbac-workspace-context">
        <ComboBox
          id="rbac-selected-application"
          titleText="Selected application"
          placeholder="Find an application"
          items={systems}
          itemToString={(system) => (system ? `${system.displayName} (${system.clientId})` : "")}
          selectedItem={
            registeringApplication
              ? null
              : (systems.find((system) => system.clientId === activeClientId) ?? null)
          }
          onChange={({ selectedItem }) => selectedItem && handleSelectSystem(selectedItem.clientId)}
          shouldFilterItem={({ item, inputValue }) =>
            `${item.displayName} ${item.clientId}`
              .toLowerCase()
              .includes((inputValue ?? "").toLowerCase())
          }
        />
        <Tag type={registeringApplication ? "gray" : "blue"}>
          {registeringApplication
            ? "New application"
            : (systemDetail?.lifecycleStatus ?? "Loading")}
        </Tag>
        {canWriteSystems && (
          <Button
            size="sm"
            kind="tertiary"
            renderIcon={Add}
            onClick={() => {
              setRegisteringApplication(true);
              setDraftClientId("");
              setDraft({ ...toDraft(), enabled: false });
              setLifecycleReason("");
              setError(null);
              setActiveSection("applications");
              setApplicationSection("basics");
            }}
          >
            Register application
          </Button>
        )}
      </div>
      <div className="rbac-workspace-mobile-navigation">
        <Dropdown
          id="rbac-workspace-section"
          titleText="Workspace section"
          label="Choose a section"
          items={[...workspaceSections]}
          itemToString={(item) => item?.label ?? ""}
          selectedItem={currentSection}
          onChange={({ selectedItem }) => selectedItem && setActiveSection(selectedItem.id)}
        />
      </div>
      <section className="rbac-page__layout">
        <nav className="rbac-workspace-navigation" aria-label="RBAC workspace">
          <span className="rbac-folder-heading">Workspace</span>
          {workspaceFolders.map((folder) => {
            const expanded = expandedFolders.includes(folder.id);
            const containsActive = folder.sections.includes(activeSection);
            const FolderIcon = expanded ? FolderOpen : Folder;
            return (
              <div className="rbac-folder" key={folder.id}>
                <button
                  className="rbac-folder-toggle"
                  type="button"
                  aria-expanded={expanded}
                  aria-controls={`rbac-${folder.id}`}
                  onClick={() =>
                    setExpandedFolders((current) =>
                      expanded ? current.filter((id) => id !== folder.id) : [...current, folder.id],
                    )
                  }
                >
                  <span className="rbac-folder-expander" aria-hidden="true">
                    {expanded ? "−" : "+"}
                  </span>
                  <FolderIcon size={20} className="rbac-folder-icon" aria-hidden="true" />
                  <span className={containsActive ? "rbac-folder-label--active" : undefined}>
                    {folder.label}
                  </span>
                </button>
                <ul id={`rbac-${folder.id}`} className="rbac-folder-pages" hidden={!expanded}>
                  {folder.sections.map((id) => {
                    const section = workspaceSections.find((item) => item.id === id)!;
                    return (
                      <li key={section.id}>
                        <button
                          className="rbac-folder-page"
                          type="button"
                          aria-current={section.id === activeSection ? "page" : undefined}
                          onClick={() => setActiveSection(section.id)}
                        >
                          <Document size={16} aria-hidden="true" />
                          <span>{section.label}</span>
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            );
          })}
        </nav>
        <main
          ref={workspaceRef}
          className="rbac-page__main"
          aria-label={currentSection.label}
          tabIndex={0}
        >
          <p className="rbac-workspace-description">{currentSection.description}</p>
          <div hidden={activeSection !== "sync"}>
            <SyncDriftPanel />
          </div>
          <div hidden={activeSection !== "effective-access"}>
            <EffectiveAccessPanel />
          </div>
          <div hidden={activeSection !== "governance"}>
            <GovernanceToolsPanel activeClientId={activeClientId} />
          </div>
          {systemLoading && <InlineLoading description="Loading system details..." />}

          {!registeringApplication && systemDetail && (
            <section
              className="rbac-card"
              hidden={activeSection !== "lifecycle"}
              aria-label="Application lifecycle"
            >
              <div className="rbac-page__panel-header">
                <h2>Application lifecycle</h2>
                <Tag type="blue">{systemDetail.lifecycleStatus}</Tag>
              </div>
              <p>
                Save configuration before changing status. Validation checks ownership, support,
                launch settings, integration contract, and enabled access roles. Changes to
                ownership, launch settings, environment, navigation, or integration require
                validation and approval again.
              </p>
              <TextInput
                id="application-lifecycle-reason"
                labelText="Reason for status change"
                value={lifecycleReason}
                maxLength={2000}
                disabled={!canWriteSystems || transitioningSystem}
                onChange={(event) => setLifecycleReason(event.target.value)}
              />
              <div className="rbac-page__panel-header">
                {(lifecycleActions[systemDetail.lifecycleStatus] ?? []).map((status) => (
                  <Button
                    key={status}
                    kind={status === "retired" ? "danger--tertiary" : "tertiary"}
                    size="sm"
                    disabled={
                      !canWriteSystems ||
                      transitioningSystem ||
                      !lifecycleReason.trim() ||
                      ((status === "approved" || status === "active") && !canWriteRoles)
                    }
                    onClick={() => handleTransitionSystem(status)}
                  >
                    {status === "configured"
                      ? "Configure"
                      : status === "validated"
                        ? "Validate"
                        : status === "approved"
                          ? "Approve"
                          : status === "active"
                            ? "Activate"
                            : status === "suspended"
                              ? "Suspend"
                              : "Retire"}
                  </Button>
                ))}
              </div>
            </section>
          )}
          <section className="rbac-card" hidden={activeSection !== "applications" || !systemReady}>
            <div className="rbac-page__panel-header">
              <h2>Application registry</h2>
              <PermissionGuard permission={PERMISSIONS.rbacWrite}>
                <Button
                  renderIcon={Save}
                  size="sm"
                  disabled={!(draftClientId || activeClientId) || savingSystem || registeringSystem}
                  onClick={handleSaveSystem}
                >
                  {registeringApplication ? "Register application" : "Save system"}
                </Button>
              </PermissionGuard>
            </div>
            <div className="rbac-config-navigation">
              <Dropdown
                id="rbac-configuration-section"
                titleText="Configuration section"
                label="Choose settings"
                items={[...applicationSections]}
                itemToString={(item) => item?.label ?? ""}
                selectedItem={applicationSections.find((item) => item.id === applicationSection)}
                onChange={({ selectedItem }) =>
                  selectedItem && setApplicationSection(selectedItem.id)
                }
              />
              <span>Changes across all configuration sections are saved together.</span>
            </div>
            <div className="rbac-config-sections">
              <div className="rbac-form-grid" hidden={applicationSection !== "basics"}>
                <TextInput
                  id="rbac-client-id"
                  labelText="Client ID"
                  value={draftClientId || activeClientId}
                  disabled={!canWriteSystems || !registeringApplication}
                  onChange={(event) => setDraftClientId(event.target.value)}
                />
                <TextInput
                  id="rbac-display-name"
                  labelText="Display name"
                  value={draft.displayName || systemDetail?.displayName || ""}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
                />
                <TextInput
                  id="rbac-category"
                  labelText="Category"
                  value={draft.category}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, category: event.target.value })}
                />
                <TextInput
                  id="rbac-icon"
                  labelText="Icon"
                  value={draft.icon}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, icon: event.target.value })}
                />
                <TextInput
                  id="rbac-description"
                  labelText="Description"
                  value={draft.description}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, description: event.target.value })}
                />
              </div>
              <div className="rbac-form-grid" hidden={applicationSection !== "ownership"}>
                <TextInput
                  id="rbac-owner-team"
                  labelText="Owner team"
                  value={draft.ownerTeam}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, ownerTeam: event.target.value })}
                />
                <TextInput
                  id="rbac-owner-name"
                  labelText="Owner name"
                  value={draft.ownerName}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, ownerName: event.target.value })}
                />
                <TextInput
                  id="rbac-owner-email"
                  labelText="Owner email"
                  value={draft.ownerEmail}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, ownerEmail: event.target.value })}
                />
                <TextInput
                  id="rbac-support-url"
                  labelText="Support URL"
                  value={draft.supportUrl}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, supportUrl: event.target.value })}
                />
                <TextInput
                  id="rbac-documentation-url"
                  labelText="Documentation URL"
                  value={draft.documentationUrl}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, documentationUrl: event.target.value })}
                />
              </div>
              <div className="rbac-form-grid" hidden={applicationSection !== "integration"}>
                <Dropdown
                  id="application-integration-type"
                  titleText="Integration type"
                  label="Select integration type"
                  items={["external", "hosted_module"] as const}
                  selectedItem={draft.integrationType}
                  disabled={!canWriteSystems}
                  onChange={({ selectedItem }) =>
                    selectedItem && setDraft({ ...draft, integrationType: selectedItem })
                  }
                />
                <TextInput
                  id="application-contract-version"
                  labelText="Contract version"
                  value={draft.contractVersion}
                  helperText="Supported version: 1"
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, contractVersion: event.target.value })}
                />
                <TextInput
                  id="rbac-environment"
                  labelText="Environment"
                  value={draft.environment}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, environment: event.target.value })}
                />
                <TextInput
                  id="rbac-criticality"
                  labelText="Criticality"
                  value={draft.criticality}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, criticality: event.target.value })}
                />
                <Dropdown
                  id="rbac-system-type"
                  titleText="System type"
                  label="Select system type"
                  items={["platform", "external"] as const}
                  selectedItem={draft.systemType}
                  disabled={!canWriteSystems}
                  onChange={({ selectedItem }) =>
                    selectedItem && setDraft({ ...draft, systemType: selectedItem })
                  }
                />
              </div>
              <div className="rbac-form-grid" hidden={applicationSection !== "launch"}>
                <TextInput
                  id="rbac-launch-url"
                  labelText="Launch URL"
                  value={draft.launchUrl}
                  disabled={!canWriteSystems}
                  onChange={(event) => setDraft({ ...draft, launchUrl: event.target.value })}
                />
                <TextInput
                  id="rbac-authenticated-launch-url"
                  labelText="Authenticated launch URL"
                  helperText="Optional backend login URL used for cross-application SSO."
                  value={draft.authenticatedLaunchUrl}
                  disabled={!canWriteSystems}
                  onChange={(event) =>
                    setDraft({ ...draft, authenticatedLaunchUrl: event.target.value })
                  }
                />
                <TextInput
                  id="rbac-sort-order"
                  labelText="Sort order"
                  type="number"
                  value={String(draft.sortOrder)}
                  disabled={!canWriteSystems}
                  onChange={(event) =>
                    setDraft({ ...draft, sortOrder: Number(event.target.value) || 0 })
                  }
                />
                <Dropdown
                  id="rbac-launch-mode"
                  titleText="Launch mode"
                  label="Select launch mode"
                  items={["internal", "new_tab", "same_tab"] as const}
                  selectedItem={draft.launchMode}
                  disabled={!canWriteSystems}
                  onChange={({ selectedItem }) =>
                    selectedItem && setDraft({ ...draft, launchMode: selectedItem })
                  }
                />
                <Toggle
                  id="rbac-display-in-launcher"
                  labelText="Show system in launcher"
                  toggled={draft.displayInLauncher}
                  disabled={!canWriteSystems}
                  onToggle={(displayInLauncher) => setDraft({ ...draft, displayInLauncher })}
                />
                <Toggle
                  id="rbac-display-in-side-nav"
                  labelText="Show system side navigation"
                  toggled={draft.displayInSideNav}
                  disabled={!canWriteSystems}
                  onToggle={(displayInSideNav) => setDraft({ ...draft, displayInSideNav })}
                />
                <Toggle
                  id="rbac-enabled"
                  labelText="Enabled (active applications only)"
                  toggled={draft.enabled}
                  disabled={!canWriteSystems || systemDetail?.lifecycleStatus !== "active"}
                  onToggle={(enabled) => setDraft({ ...draft, enabled })}
                />
              </div>
              <div className="rbac-form-grid" hidden={applicationSection !== "navigation"}>
                <div className="rbac-form-grid__wide">
                  <TextArea
                    id="rbac-navigation"
                    labelText="Navigation JSON"
                    helperText="Module entries may independently set displayInLauncher and displayInSideNav."
                    rows={10}
                    value={draft.navigation}
                    invalid={Boolean(navigationPreview.error)}
                    invalidText={navigationPreview.error}
                    disabled={!canWriteSystems}
                    onChange={(event) => setDraft({ ...draft, navigation: event.target.value })}
                  />
                  {!navigationPreview.error && draft.navigation.trim() ? (
                    <div className="rbac-navigation-preview" aria-live="polite">
                      <span>Launcher preview</span>
                      <div className="rbac-tag-list">
                        {navigationPreview.launcherItems.length ? (
                          navigationPreview.launcherItems.map((item) => (
                            <Tag key={`launcher-${item.id}`} type="blue">
                              {item.label}
                            </Tag>
                          ))
                        ) : (
                          <small>No module launcher entries.</small>
                        )}
                      </div>
                      <span>Side navigation preview</span>
                      <div className="rbac-tag-list">
                        {navigationPreview.sideNavItems.length ? (
                          navigationPreview.sideNavItems.map((item) => (
                            <Tag key={`sidenav-${item.id}`} type="gray">
                              {item.label}
                            </Tag>
                          ))
                        ) : (
                          <small>No visible side-navigation entries.</small>
                        )}
                      </div>
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          </section>

          <section className="rbac-card" hidden={activeSection !== "access-roles" || !systemReady}>
            <div className="rbac-page__panel-header">
              <h2>Access Roles</h2>
              <div className="rbac-inline-form">
                <TextInput
                  id="rbac-new-access-role"
                  hideLabel
                  labelText="Access role"
                  placeholder="viewer"
                  value={newAccessRole}
                  disabled={!canWriteRoles}
                  onChange={(event) => setNewAccessRole(event.target.value)}
                />
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button renderIcon={Add} size="sm" onClick={handleAddAccessRole}>
                    Add
                  </Button>
                </PermissionGuard>
              </div>
            </div>
            <div className="rbac-tag-list">
              {(systemDetail?.accessRoles ?? []).map((role) => (
                <Tag
                  key={role}
                  type="cyan"
                  filter={canWriteRoles}
                  onClose={
                    canWriteRoles
                      ? (event) => {
                          event.preventDefault();
                          void handleRemoveAccessRole(role);
                        }
                      : undefined
                  }
                >
                  {role}
                </Tag>
              ))}
            </div>
          </section>

          <section className="rbac-card" hidden={activeSection !== "system-roles" || !systemReady}>
            <div className="rbac-page__panel-header">
              <h2>System Roles</h2>
              <div className="rbac-inline-form">
                <TextInput
                  id="rbac-new-role-name"
                  hideLabel
                  labelText="Role name"
                  placeholder="role_name"
                  value={newRoleName}
                  disabled={!canWriteRoles}
                  onChange={(event) => setNewRoleName(event.target.value)}
                />
                <TextInput
                  id="rbac-new-role-display"
                  hideLabel
                  labelText="Role display name"
                  placeholder="Display name"
                  value={newRoleDisplayName}
                  disabled={!canWriteRoles}
                  onChange={(event) => setNewRoleDisplayName(event.target.value)}
                />
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button
                    renderIcon={Add}
                    size="sm"
                    disabled={creatingRole}
                    onClick={handleCreateRole}
                  >
                    Create
                  </Button>
                </PermissionGuard>
              </div>
            </div>

            <div className="rbac-role-grid">
              <div className="rbac-role-list">
                {(systemDetail?.roles ?? []).map((role) => (
                  <div
                    className={`rbac-role-list__item ${role.id === selectedRole?.id ? "is-active" : ""}`}
                    key={role.id}
                  >
                    <button
                      type="button"
                      className="rbac-role-list__select"
                      aria-pressed={role.id === selectedRole?.id}
                      onClick={() => setSelectedRoleId(role.id)}
                    >
                      <span>{role.displayName || role.name}</span>
                      <small>{role.permissions.length} permissions</small>
                    </button>
                    <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                      <Button
                        hasIconOnly
                        iconDescription="Delete role"
                        kind="ghost"
                        renderIcon={TrashCan}
                        size="sm"
                        onClick={(event) => {
                          event.stopPropagation();
                          void handleDeleteRole(role);
                        }}
                      />
                    </PermissionGuard>
                  </div>
                ))}
              </div>
              <PermissionMatrix
                groupedPermissions={groupedPermissions}
                selectedKeys={rolePermissionKeys}
                disabled={!selectedRole || !canWritePermissions}
                onToggle={toggleRolePermission}
              />
            </div>
          </section>

          <section className="rbac-card" hidden={activeSection !== "realm-roles"}>
            <div className="rbac-page__panel-header">
              <h2>Realm Role Permissions</h2>
              <div className="rbac-inline-form">
                <TextInput
                  id="rbac-realm-role-name"
                  hideLabel
                  labelText="Realm role"
                  placeholder="admin"
                  value={realmRoleName}
                  disabled={!canWritePermissions}
                  onChange={(event) => setRealmRoleName(event.target.value)}
                />
              </div>
            </div>
            <div className="rbac-realm-layout">
              <div className="rbac-role-list">
                {realmRoles.map((group) => (
                  <button
                    className={`rbac-role-list__item ${group.realmRole === selectedRealmRole ? "is-active" : ""}`}
                    key={group.realmRole}
                    onClick={() => {
                      setSelectedRealmRole(group.realmRole);
                      setRealmRoleName(group.realmRole);
                    }}
                    type="button"
                  >
                    <span>{group.realmRole}</span>
                    <small>{group.permissions.length} permissions</small>
                  </button>
                ))}
              </div>
              <PermissionMatrix
                groupedPermissions={groupedPermissions}
                selectedKeys={realmPermissionKeys}
                disabled={(!selectedRealmRole && !realmRoleName.trim()) || !canWritePermissions}
                onToggle={toggleRealmPermission}
              />
            </div>
          </section>

          <section className="rbac-card" hidden={activeSection !== "groups"}>
            <div className="rbac-page__panel-header">
              <div>
                <h2>Groups</h2>
                <p>Map Keycloak groups to realm roles, system roles, and direct permissions.</p>
              </div>
              <div className="rbac-inline-form">
                <TextInput
                  id="rbac-new-group-path"
                  hideLabel
                  labelText="Group path"
                  placeholder="/data-statistics/users"
                  value={newGroupPath}
                  disabled={!canWriteRoles}
                  onChange={(event) => setNewGroupPath(event.target.value)}
                />
                <TextInput
                  id="rbac-new-group-display"
                  hideLabel
                  labelText="Group display name"
                  placeholder="Data statistics users"
                  value={newGroupDisplayName}
                  disabled={!canWriteRoles}
                  onChange={(event) => setNewGroupDisplayName(event.target.value)}
                />
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button
                    renderIcon={Add}
                    size="sm"
                    disabled={creatingGroup || !newGroupPath.trim()}
                    onClick={handleCreateGroup}
                  >
                    Save group
                  </Button>
                </PermissionGuard>
              </div>
            </div>

            <div className="rbac-realm-layout">
              <div className="rbac-role-list">
                <div className="rbac-group-search">
                  <Search
                    size="sm"
                    labelText="Find groups"
                    placeholder="Find a group"
                    closeButtonLabelText="Clear group search"
                    value={groupSearch}
                    onChange={(event) => setGroupSearch(event.target.value)}
                  />
                </div>
                {groups
                  .filter((group) =>
                    `${group.displayName} ${group.name} ${group.path}`
                      .toLowerCase()
                      .includes(groupSearch.trim().toLowerCase()),
                  )
                  .map((group) => (
                    <button
                      className={`rbac-role-list__item ${group.id === selectedGroup?.id ? "is-active" : ""}`}
                      key={group.id}
                      onClick={() => setSelectedGroupId(group.id)}
                      type="button"
                    >
                      <span>{group.displayName || group.name}</span>
                      <small>
                        {group.path} · {group.memberCount} members
                      </small>
                    </button>
                  ))}
              </div>

              <div className="rbac-group-detail">
                {selectedGroup ? (
                  <>
                    <div className="rbac-page__panel-header">
                      <div>
                        <h3>{selectedGroup.displayName || selectedGroup.name}</h3>
                        <p>{selectedGroup.path}</p>
                      </div>
                      <Tag type={selectedGroup.enabled ? "green" : "gray"}>
                        {selectedGroup.enabled ? "Enabled" : "Disabled"}
                      </Tag>
                    </div>

                    <div className="rbac-group-section-navigation">
                      <Dropdown
                        id="rbac-group-section"
                        titleText="Group section"
                        label="Choose group details"
                        items={["Members", "Role mappings", "Direct permissions"]}
                        selectedItem={groupSection}
                        onChange={({ selectedItem }) =>
                          selectedItem && setGroupSection(selectedItem)
                        }
                      />
                    </div>
                    <div className="rbac-group-detail__section" hidden={groupSection !== "Members"}>
                      <div className="rbac-group-detail__section-header">
                        <div>
                          <h4>Members</h4>
                          <p>
                            Manage Keycloak group membership. Changes are written to Keycloak and
                            synced back into the portal RBAC cache.
                          </p>
                        </div>
                        <Button
                          kind="ghost"
                          renderIcon={Renew}
                          size="sm"
                          disabled={syncingGroupMembers || !canWriteRoles}
                          onClick={handleSyncGroupMembers}
                        >
                          Sync members
                        </Button>
                      </div>

                      {!selectedGroup.keycloakGroupId && (
                        <InlineNotification
                          kind="warning"
                          lowContrast
                          title="Keycloak link missing"
                          subtitle="This group does not have a cached Keycloak group ID. The backend will try to match it by path; if that fails, run live RBAC sync or create the group in Keycloak first."
                        />
                      )}

                      <div className="rbac-inline-form rbac-group-members__add">
                        <Dropdown<User>
                          id="rbac-group-member-user"
                          titleText="Add user"
                          label="Select user"
                          items={assignableGroupUsers}
                          selectedItem={
                            assignableGroupUsers.find(
                              (user) => user.id === selectedGroupMemberUserId,
                            ) ?? undefined
                          }
                          itemToString={(user) =>
                            user ? `${user.username}${user.email ? ` · ${user.email}` : ""}` : ""
                          }
                          disabled={!canWriteRoles || addingGroupMember}
                          onChange={({ selectedItem }) =>
                            setSelectedGroupMemberUserId(selectedItem?.id ?? "")
                          }
                        />
                        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                          <Button
                            size="md"
                            renderIcon={Add}
                            disabled={!selectedGroupMemberUserId || addingGroupMember}
                            onClick={handleAddGroupMember}
                          >
                            Add member
                          </Button>
                        </PermissionGuard>
                      </div>

                      {(groupMembersLoading || groupMembersFetching) && (
                        <InlineLoading description="Loading group members..." />
                      )}

                      <DataTable
                        rows={groupMemberRows}
                        headers={[
                          { key: "username", header: "Username" },
                          { key: "email", header: "Email" },
                          { key: "actions", header: "" },
                        ]}
                      >
                        {({ rows, headers, getHeaderProps, getRowProps }) => (
                          <TableContainer className="rbac-group-members-table">
                            <Table size="lg">
                              <TableHead>
                                <TableRow>
                                  {headers.map((header) => {
                                    const { key, ...headerProps } = getHeaderProps({ header });
                                    return (
                                      <TableHeader key={key} {...headerProps}>
                                        {header.header}
                                      </TableHeader>
                                    );
                                  })}
                                </TableRow>
                              </TableHead>
                              <TableBody>
                                {rows.length === 0 ? (
                                  <TableRow>
                                    <TableCell colSpan={headers.length}>
                                      <div className="rbac-group-members-table__empty">
                                        No synced members for this group.
                                      </div>
                                    </TableCell>
                                  </TableRow>
                                ) : (
                                  rows.map((row) => {
                                    const member = groupMembers.find(
                                      (item) => item.userId === row.id,
                                    );
                                    const { key, ...rowProps } = getRowProps({ row });

                                    return (
                                      <TableRow key={key} {...rowProps}>
                                        {row.cells.map((cell) => {
                                          if (cell.info.header === "actions") {
                                            return (
                                              <TableCell key={cell.id}>
                                                <PermissionGuard
                                                  permission={PERMISSIONS.rbacRolesWrite}
                                                >
                                                  <Button
                                                    hasIconOnly
                                                    iconDescription="Remove group member"
                                                    kind="ghost"
                                                    renderIcon={TrashCan}
                                                    size="sm"
                                                    disabled={!member || removingGroupMember}
                                                    onClick={() => {
                                                      if (member) handleRemoveGroupMember(member);
                                                    }}
                                                  />
                                                </PermissionGuard>
                                              </TableCell>
                                            );
                                          }

                                          return <TableCell key={cell.id}>{cell.value}</TableCell>;
                                        })}
                                      </TableRow>
                                    );
                                  })
                                )}
                              </TableBody>
                            </Table>
                          </TableContainer>
                        )}
                      </DataTable>
                    </div>

                    <div
                      className="rbac-group-detail__section"
                      hidden={groupSection !== "Role mappings"}
                    >
                      <h4>Realm roles</h4>
                      <div className="rbac-inline-form">
                        <TextInput
                          id="rbac-group-realm-role"
                          hideLabel
                          labelText="Realm role"
                          placeholder="user"
                          value={groupRealmRoleName}
                          disabled={!canWriteRoles}
                          onChange={(event) => setGroupRealmRoleName(event.target.value)}
                        />
                        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                          <Button size="sm" renderIcon={Add} onClick={handleAssignGroupRealmRole}>
                            Add
                          </Button>
                        </PermissionGuard>
                      </div>
                      <div className="rbac-tag-list">
                        {selectedGroup.realmRoles.map((role) => (
                          <Tag
                            key={role}
                            type="purple"
                            filter={canWriteRoles}
                            onClose={
                              canWriteRoles
                                ? (event) => {
                                    event.preventDefault();
                                    void handleRemoveGroupRealmRole(role);
                                  }
                                : undefined
                            }
                          >
                            {role}
                          </Tag>
                        ))}
                      </div>
                    </div>

                    <div
                      className="rbac-group-detail__section"
                      hidden={groupSection !== "Role mappings"}
                    >
                      <h4>System roles</h4>
                      <div className="rbac-inline-form">
                        <TextInput
                          id="rbac-group-system-client"
                          hideLabel
                          labelText="Client ID"
                          placeholder="data-statistics"
                          value={groupSystemClientId}
                          disabled={!canWriteRoles}
                          onChange={(event) => setGroupSystemClientId(event.target.value)}
                        />
                        <TextInput
                          id="rbac-group-system-role"
                          hideLabel
                          labelText="Role name"
                          placeholder="data-statistics_access"
                          value={groupSystemRoleName}
                          disabled={!canWriteRoles}
                          onChange={(event) => setGroupSystemRoleName(event.target.value)}
                        />
                        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                          <Button size="sm" renderIcon={Add} onClick={handleAssignGroupSystemRole}>
                            Add
                          </Button>
                        </PermissionGuard>
                      </div>
                      <div className="rbac-tag-list">
                        {selectedGroup.systemRoles.map((role) => (
                          <Tag
                            key={`${role.clientId}:${role.roleName}`}
                            type="cyan"
                            filter={canWriteRoles}
                            onClose={
                              canWriteRoles
                                ? (event) => {
                                    event.preventDefault();
                                    void handleRemoveGroupSystemRole(
                                      selectedGroup,
                                      role.clientId,
                                      role.roleName,
                                    );
                                  }
                                : undefined
                            }
                          >
                            {role.clientId}:{role.roleName}
                          </Tag>
                        ))}
                      </div>
                    </div>

                    <div
                      className="rbac-group-detail__section"
                      hidden={groupSection !== "Direct permissions"}
                    >
                      <h4>Direct permissions</h4>
                      <PermissionMatrix
                        groupedPermissions={groupedPermissions}
                        selectedKeys={groupPermissionKeys}
                        disabled={!canWritePermissions}
                        onToggle={toggleGroupPermission}
                      />
                    </div>
                  </>
                ) : (
                  <p>No groups have been synced or created yet.</p>
                )}
              </div>
            </div>
          </section>
        </main>
      </section>
    </div>
  );
}

type PermissionMatrixProps = {
  groupedPermissions: Record<string, RbacPermission[]>;
  selectedKeys: Set<string>;
  disabled?: boolean;
  onToggle: (permission: RbacPermission, checked: boolean) => Promise<void>;
};

function PermissionMatrix({
  groupedPermissions,
  selectedKeys,
  disabled = false,
  onToggle,
}: PermissionMatrixProps) {
  const matrixId = useId();
  const [search, setSearch] = useState("");
  const visibleGroups = Object.entries(groupedPermissions)
    .map(
      ([category, permissions]) =>
        [
          category,
          permissions.filter((permission) =>
            `${category} ${permission.key} ${permissionLabel(permission)}`
              .toLowerCase()
              .includes(search.trim().toLowerCase()),
          ),
        ] as const,
    )
    .filter(([, permissions]) => permissions.length > 0);
  return (
    <div>
      <div className="rbac-permission-search">
        <Search
          size="sm"
          labelText="Find permissions"
          placeholder="Find a permission or category"
          closeButtonLabelText="Clear permission search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      {visibleGroups.length === 0 && <p className="rbac-tag-list">No matching permissions.</p>}
      <div className="rbac-permission-matrix">
        {visibleGroups.map(([category, permissions]) => (
          <div className="rbac-permission-group" key={category}>
            <h3>{category}</h3>
            {permissions.map((permission) => (
              <Checkbox
                id={`${matrixId}-${category}-${permission.key}`}
                key={permission.key}
                labelText={permissionLabel(permission)}
                checked={selectedKeys.has(permission.key)}
                disabled={disabled}
                onChange={(_, { checked }) => {
                  void onToggle(permission, checked);
                }}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
