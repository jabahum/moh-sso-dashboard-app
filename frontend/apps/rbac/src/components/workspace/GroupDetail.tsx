import { useMemo, useState, type FormEvent } from "react";
import {
  Button,
  ComboBox,
  DataTable,
  InlineLoading,
  InlineNotification,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Tag,
  TextInput,
} from "@carbon/react";
import { Add, Renew, TrashCan } from "@carbon/react/icons";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { type User } from "@moh-sso/users";
import { useToast } from "@moh-sso/ui";

import {
  useAddRbacGroupMemberMutation,
  useAssignGroupPermissionMutation,
  useAssignGroupRealmRoleMutation,
  useAssignGroupSystemRoleMutation,
  useGetRbacSystemQuery,
  useListRbacGroupMembersQuery,
  useRemoveGroupPermissionMutation,
  useRemoveGroupRealmRoleMutation,
  useRemoveGroupSystemRoleMutation,
  useRemoveRbacGroupMemberMutation,
  useSyncRbacGroupMembersMutation,
} from "../../api";
import type { RbacGroup, RbacGroupMember, RbacGroupSystemRole, RbacSystem } from "../../types";
import { ActionError, useConfirm, useKeySet, useUnsavedGuard } from "./feedback";
import { StagedPermissionEditor } from "./PermissionMatrix";
import { getApiErrorMessage, pluralize, type PermissionCategory } from "./workspace-helpers";

type GroupDetailProps = {
  group: RbacGroup;
  users: User[];
  systems: RbacSystem[];
  categories: PermissionCategory[];
  canWriteRoles: boolean;
  canWritePermissions: boolean;
};

const groupTabs = ["Members", "Role mappings", "Direct permissions"] as const;

export function GroupDetail(props: GroupDetailProps) {
  const { group } = props;
  const guard = useUnsavedGuard();
  const [tab, setTab] = useState(0);
  const groupName = group.displayName || group.name;

  return (
    <div className="rbac-group-detail">
      <div className="rbac-split__detail-header">
        <div>
          <h3>{groupName}</h3>
          <code>{group.path}</code>
        </div>
        <Tag type={group.enabled ? "green" : "gray"}>{group.enabled ? "Enabled" : "Disabled"}</Tag>
      </div>
      <Tabs
        selectedIndex={tab}
        onChange={({ selectedIndex }) => {
          if (selectedIndex !== tab) void guard(() => setTab(selectedIndex));
        }}
      >
        <TabList aria-label="Group details" className="rbac-subtabs">
          <Tab>Members ({group.memberCount})</Tab>
          <Tab>Role mappings ({group.realmRoles.length + group.systemRoles.length})</Tab>
          <Tab>Direct permissions ({group.permissions.length})</Tab>
        </TabList>
        <TabPanels>
          <TabPanel className="rbac-subtab-panel" aria-label={groupTabs[0]}>
            {tab === 0 && <GroupMembers {...props} />}
          </TabPanel>
          <TabPanel className="rbac-subtab-panel" aria-label={groupTabs[1]}>
            {tab === 1 && <GroupRoleMappings {...props} />}
          </TabPanel>
          <TabPanel className="rbac-subtab-panel" aria-label={groupTabs[2]}>
            {tab === 2 && <GroupPermissions {...props} />}
          </TabPanel>
        </TabPanels>
      </Tabs>
    </div>
  );
}

function GroupMembers({ group, users, canWriteRoles }: GroupDetailProps) {
  const toast = useToast();
  const confirm = useConfirm();
  const groupName = group.displayName || group.name;
  const {
    data: members = [],
    isLoading,
    isFetching,
  } = useListRbacGroupMembersQuery(group.id, { skip: !group.id });
  const [addMember, { isLoading: adding }] = useAddRbacGroupMemberMutation();
  const [removeMember, { isLoading: removing }] = useRemoveRbacGroupMemberMutation();
  const [syncMembers, { isLoading: syncing }] = useSyncRbacGroupMembersMutation();
  const [selectedUserId, setSelectedUserId] = useState("");
  const [error, setError] = useState<{ title: string; message: string } | null>(null);

  const memberIds = useMemo(() => new Set(members.map((member) => member.userId)), [members]);
  const assignableUsers = useMemo(
    () => users.filter((user) => !memberIds.has(user.id)),
    [memberIds, users],
  );
  const rows = useMemo(
    () =>
      members.map((member) => ({
        id: member.userId,
        username: member.username || "—",
        email: member.email || "—",
        userId: member.userId,
      })),
    [members],
  );

  const notifyWarnings = (warnings?: string[]) => {
    if (warnings?.length) toast.warning("Group updated with warnings", warnings.join(" "));
  };

  const handleAdd = async () => {
    if (!selectedUserId) return;
    const user = users.find((item) => item.id === selectedUserId);
    try {
      setError(null);
      const result = await addMember({
        groupId: group.id,
        data: { userId: selectedUserId },
      }).unwrap();
      setSelectedUserId("");
      toast.success("Group member added", `${user?.username ?? "User"} was added to ${groupName}.`);
      notifyWarnings(result.warnings);
    } catch (failure) {
      setError({
        title: "Member not added",
        message: getApiErrorMessage(failure, "Unable to add this user to the group."),
      });
    }
  };

  const handleRemove = async (member: RbacGroupMember) => {
    const confirmed = await confirm({
      title: "Remove group member",
      body: (
        <p>
          Remove <strong>{member.username || member.email || member.userId}</strong> from{" "}
          <strong>{groupName}</strong>? They lose any access inherited from this group.
        </p>
      ),
      confirmLabel: "Remove",
      danger: true,
    });
    if (!confirmed) return;
    try {
      setError(null);
      const result = await removeMember({ groupId: group.id, userId: member.userId }).unwrap();
      toast.success("Group member removed", "The user's inherited access was refreshed.");
      notifyWarnings(result.warnings);
    } catch (failure) {
      setError({
        title: "Member not removed",
        message: getApiErrorMessage(failure, "Unable to remove this user from the group."),
      });
    }
  };

  const handleSync = async () => {
    try {
      setError(null);
      const result = await syncMembers(group.id).unwrap();
      toast.success(
        "Group members synced",
        `${pluralize(result.memberCount, "member")} synced from Keycloak.`,
      );
      notifyWarnings(result.warnings);
    } catch (failure) {
      setError({
        title: "Members not synced",
        message: getApiErrorMessage(failure, "Unable to sync group members from Keycloak."),
      });
    }
  };

  return (
    <div className="rbac-stack">
      <div className="rbac-section-header">
        <p>Membership is written to Keycloak and synced back into the portal RBAC cache.</p>
        <Button
          kind="ghost"
          renderIcon={Renew}
          size="md"
          disabled={syncing || !canWriteRoles}
          onClick={() => void handleSync()}
        >
          Sync members
        </Button>
      </div>

      {!group.keycloakGroupId && (
        <InlineNotification
          kind="warning"
          lowContrast
          hideCloseButton
          title="Keycloak link missing"
          subtitle="This group does not have a cached Keycloak group ID. The backend will try to match it by path; if that fails, run live RBAC sync or create the group in Keycloak first."
        />
      )}

      <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
        <div className="rbac-create-form rbac-create-form--inline">
          <ComboBox<User>
            id="rbac-group-member-user"
            titleText="Add a user"
            helperText="Only users who are not already members are listed."
            placeholder="Find a user by name or email"
            items={assignableUsers}
            selectedItem={assignableUsers.find((user) => user.id === selectedUserId) ?? null}
            itemToString={(user) =>
              user ? `${user.username}${user.email ? ` · ${user.email}` : ""}` : ""
            }
            shouldFilterItem={({ item, inputValue }) =>
              `${item.username} ${item.email ?? ""}`
                .toLowerCase()
                .includes((inputValue ?? "").toLowerCase())
            }
            disabled={!canWriteRoles || adding}
            onChange={({ selectedItem }) => setSelectedUserId(selectedItem?.id ?? "")}
          />
          <Button
            size="md"
            renderIcon={Add}
            disabled={!selectedUserId || adding}
            onClick={() => void handleAdd()}
          >
            Add member
          </Button>
        </div>
      </PermissionGuard>

      <ActionError
        title={error?.title ?? ""}
        message={error?.message}
        onClose={() => setError(null)}
      />

      {(isLoading || isFetching) && <InlineLoading description="Loading group members..." />}

      <DataTable
        rows={rows}
        headers={[
          { key: "username", header: "Username" },
          { key: "email", header: "Email" },
          { key: "actions", header: "" },
        ]}
      >
        {({ rows: tableRows, headers, getHeaderProps, getRowProps, getTableProps }) => (
          <TableContainer className="rbac-table">
            <Table {...getTableProps()} size="lg" aria-label={`Members of ${groupName}`}>
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
                {tableRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={headers.length}>
                      <div className="rbac-empty">No synced members for this group.</div>
                    </TableCell>
                  </TableRow>
                ) : (
                  tableRows.map((row) => {
                    const member = members.find((item) => item.userId === row.id);
                    const { key, ...rowProps } = getRowProps({ row });
                    return (
                      <TableRow key={key} {...rowProps}>
                        {row.cells.map((cell) =>
                          cell.info.header === "actions" ? (
                            <TableCell key={cell.id} className="rbac-table__actions">
                              <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                                <Button
                                  hasIconOnly
                                  iconDescription="Remove group member"
                                  tooltipPosition="left"
                                  kind="ghost"
                                  renderIcon={TrashCan}
                                  size="sm"
                                  disabled={!member || removing}
                                  onClick={() => {
                                    if (member) void handleRemove(member);
                                  }}
                                />
                              </PermissionGuard>
                            </TableCell>
                          ) : (
                            <TableCell key={cell.id}>{cell.value}</TableCell>
                          ),
                        )}
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
  );
}

function GroupRoleMappings({ group, systems, canWriteRoles }: GroupDetailProps) {
  const toast = useToast();
  const confirm = useConfirm();
  const groupName = group.displayName || group.name;
  const [assignRealmRole, { isLoading: assigningRealm }] = useAssignGroupRealmRoleMutation();
  const [removeRealmRole] = useRemoveGroupRealmRoleMutation();
  const [assignSystemRole, { isLoading: assigningSystem }] = useAssignGroupSystemRoleMutation();
  const [removeSystemRole] = useRemoveGroupSystemRoleMutation();
  const [realmRole, setRealmRole] = useState("");
  const [clientId, setClientId] = useState("");
  const [roleName, setRoleName] = useState("");
  const [realmError, setRealmError] = useState<string | null>(null);
  const [systemError, setSystemError] = useState<string | null>(null);

  const { currentData: system, isFetching: loadingRoles } = useGetRbacSystemQuery(clientId, {
    skip: !clientId,
  });
  const assignedSystemRoles = new Set(
    group.systemRoles.map((role) => `${role.clientId}:${role.roleName}`),
  );
  // Service-only roles cannot be granted to people, so groups cannot hold them.
  const assignableRoles = (system?.roles ?? []).filter(
    (role) => !role.serviceOnly && !assignedSystemRoles.has(`${clientId}:${role.name}`),
  );

  const trimmedRealmRole = realmRole.trim();
  const realmDuplicate = group.realmRoles.includes(trimmedRealmRole);

  const handleAddRealmRole = async (event: FormEvent) => {
    event.preventDefault();
    if (!trimmedRealmRole || realmDuplicate) return;
    try {
      setRealmError(null);
      await assignRealmRole({ groupId: group.id, data: { realmRole: trimmedRealmRole } }).unwrap();
      setRealmRole("");
      toast.success("Realm role mapped", `${groupName} members now hold ${trimmedRealmRole}.`);
    } catch (failure) {
      setRealmError(getApiErrorMessage(failure, "Unable to assign group realm role."));
    }
  };

  const handleRemoveRealmRole = async (role: string) => {
    const confirmed = await confirm({
      title: `Remove realm role “${role}”?`,
      body: (
        <p>
          Members of <strong>{groupName}</strong> stop inheriting <code>{role}</code> from this
          group.
        </p>
      ),
      confirmLabel: "Remove realm role",
      danger: true,
    });
    if (!confirmed) return;
    try {
      setRealmError(null);
      await removeRealmRole({ groupId: group.id, realmRole: role }).unwrap();
      toast.success("Realm role removed", `${role} was removed from ${groupName}.`);
    } catch (failure) {
      setRealmError(getApiErrorMessage(failure, "Unable to remove group realm role."));
    }
  };

  const handleAddSystemRole = async (event: FormEvent) => {
    event.preventDefault();
    if (!clientId || !roleName) return;
    try {
      setSystemError(null);
      await assignSystemRole({ groupId: group.id, data: { clientId, roleName } }).unwrap();
      setRoleName("");
      toast.success("System role mapped", `${groupName} members now hold ${clientId}:${roleName}.`);
    } catch (failure) {
      setSystemError(getApiErrorMessage(failure, "Unable to assign group system role."));
    }
  };

  const handleRemoveSystemRole = async (role: RbacGroupSystemRole) => {
    const label = `${role.clientId}:${role.roleName}`;
    const confirmed = await confirm({
      title: `Remove system role “${role.displayName || role.roleName}”?`,
      body: (
        <p>
          Members of <strong>{groupName}</strong> stop inheriting <code>{label}</code> from this
          group.
        </p>
      ),
      confirmLabel: "Remove system role",
      danger: true,
    });
    if (!confirmed) return;
    try {
      setSystemError(null);
      await removeSystemRole({
        groupId: group.id,
        clientId: role.clientId,
        roleName: role.roleName,
      }).unwrap();
      toast.success("System role removed", `${label} was removed from ${groupName}.`);
    } catch (failure) {
      setSystemError(getApiErrorMessage(failure, "Unable to remove group system role."));
    }
  };

  return (
    <div className="rbac-mapping-grid">
      <section className="rbac-stack" aria-labelledby="rbac-group-realm-roles">
        <h4 id="rbac-group-realm-roles">Realm roles</h4>
        {group.realmRoles.length ? (
          <ul className="rbac-item-list" aria-label="Realm roles">
            {group.realmRoles.map((role) => (
              <li key={role} className="rbac-item-list__row">
                <code>{role}</code>
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button
                    kind="ghost"
                    size="sm"
                    renderIcon={TrashCan}
                    onClick={() => void handleRemoveRealmRole(role)}
                  >
                    Remove<span className="cds--visually-hidden"> {role}</span>
                  </Button>
                </PermissionGuard>
              </li>
            ))}
          </ul>
        ) : (
          <p className="rbac-empty">No realm roles mapped.</p>
        )}
        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
          <form className="rbac-create-form" onSubmit={(event) => void handleAddRealmRole(event)}>
            <TextInput
              id="rbac-group-realm-role"
              labelText="Realm role name"
              helperText="A Keycloak realm role, for example “user”."
              value={realmRole}
              disabled={!canWriteRoles || assigningRealm}
              invalid={realmDuplicate}
              invalidText="The group already holds this realm role."
              onChange={(event) => setRealmRole(event.target.value)}
            />
            <Button
              type="submit"
              size="md"
              renderIcon={Add}
              disabled={!canWriteRoles || !trimmedRealmRole || realmDuplicate || assigningRealm}
            >
              Add realm role
            </Button>
          </form>
        </PermissionGuard>
        <ActionError
          title="Realm role not updated"
          message={realmError}
          onClose={() => setRealmError(null)}
        />
      </section>

      <section className="rbac-stack" aria-labelledby="rbac-group-system-roles">
        <h4 id="rbac-group-system-roles">System roles</h4>
        {group.systemRoles.length ? (
          <ul className="rbac-item-list" aria-label="System roles">
            {group.systemRoles.map((role) => (
              <li key={`${role.clientId}:${role.roleName}`} className="rbac-item-list__row">
                <span className="rbac-item-list__text">
                  <span>{role.displayName || role.roleName}</span>
                  <code>
                    {role.clientId}:{role.roleName}
                  </code>
                </span>
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button
                    kind="ghost"
                    size="sm"
                    renderIcon={TrashCan}
                    onClick={() => void handleRemoveSystemRole(role)}
                  >
                    Remove
                    <span className="cds--visually-hidden">
                      {" "}
                      {role.clientId}:{role.roleName}
                    </span>
                  </Button>
                </PermissionGuard>
              </li>
            ))}
          </ul>
        ) : (
          <p className="rbac-empty">No system roles mapped.</p>
        )}
        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
          <form className="rbac-create-form" onSubmit={(event) => void handleAddSystemRole(event)}>
            <ComboBox<RbacSystem>
              id="rbac-group-system-client"
              titleText="Application"
              placeholder="Find an application"
              items={systems}
              itemToString={(item) => (item ? `${item.displayName} (${item.clientId})` : "")}
              selectedItem={systems.find((item) => item.clientId === clientId) ?? null}
              shouldFilterItem={({ item, inputValue }) =>
                `${item.displayName} ${item.clientId}`
                  .toLowerCase()
                  .includes((inputValue ?? "").toLowerCase())
              }
              disabled={!canWriteRoles || assigningSystem}
              onChange={({ selectedItem }) => {
                setClientId(selectedItem?.clientId ?? "");
                setRoleName("");
              }}
            />
            <ComboBox
              key={clientId}
              id="rbac-group-system-role"
              titleText="Role"
              helperText={
                clientId && !loadingRoles && assignableRoles.length === 0
                  ? "This application has no other roles that a group can hold."
                  : "Service-only roles are not listed; they cannot be given to people."
              }
              placeholder={clientId ? "Choose a role" : "Choose an application first"}
              items={assignableRoles}
              itemToString={(item) =>
                item
                  ? `${item.displayName || item.name}${item.displayName ? ` (${item.name})` : ""}`
                  : ""
              }
              selectedItem={assignableRoles.find((role) => role.name === roleName) ?? null}
              disabled={!canWriteRoles || !clientId || loadingRoles || assigningSystem}
              onChange={({ selectedItem }) => setRoleName(selectedItem?.name ?? "")}
            />
            <Button
              type="submit"
              size="md"
              renderIcon={Add}
              disabled={!canWriteRoles || !clientId || !roleName || assigningSystem}
            >
              Add system role
            </Button>
          </form>
        </PermissionGuard>
        <ActionError
          title="System role not updated"
          message={systemError}
          onClose={() => setSystemError(null)}
        />
      </section>
    </div>
  );
}

function GroupPermissions({ group, categories, canWritePermissions }: GroupDetailProps) {
  const [assignPermission] = useAssignGroupPermissionMutation();
  const [removePermission] = useRemoveGroupPermissionMutation();
  const savedKeys = useKeySet(group.permissions.map((permission) => permission.key));
  return (
    <StagedPermissionEditor
      key={group.id}
      categories={categories}
      savedKeys={savedKeys}
      subjectLabel={`group “${group.displayName || group.name}”`}
      canEdit={canWritePermissions}
      readOnlyReason="Changing permissions requires the rbac:permissions:write permission."
      onAssign={(permissionKey) =>
        assignPermission({ groupId: group.id, data: { permissionKey } }).unwrap()
      }
      onRemove={(permissionKey) => removePermission({ groupId: group.id, permissionKey }).unwrap()}
    />
  );
}
