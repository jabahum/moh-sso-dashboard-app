import { useState, type FormEvent } from "react";
import { Button, Search, TextInput } from "@carbon/react";
import { Add } from "@carbon/react/icons";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { useListUsersQuery } from "@moh-sso/users";
import { useToast } from "@moh-sso/ui";

import { useListRbacGroupsQuery, useUpsertRbacGroupMutation } from "../../api";
import type { RbacSystem } from "../../types";
import { ActionError, useUnsavedGuard } from "./feedback";
import { GroupDetail } from "./GroupDetail";
import { getApiErrorMessage, pluralize, type PermissionCategory } from "./workspace-helpers";

type GroupsPanelProps = {
  systems: RbacSystem[];
  categories: PermissionCategory[];
  canWriteRoles: boolean;
  canWritePermissions: boolean;
};

export function GroupsPanel({
  systems,
  categories,
  canWriteRoles,
  canWritePermissions,
}: GroupsPanelProps) {
  const toast = useToast();
  const guard = useUnsavedGuard();
  const { data: groups = [] } = useListRbacGroupsQuery();
  const { data: users = [] } = useListUsersQuery();
  const [upsertGroup, { isLoading: saving }] = useUpsertRbacGroupMutation();
  const [selectedGroupId, setSelectedGroupId] = useState("");
  const [search, setSearch] = useState("");
  const [path, setPath] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const selectedGroup = groups.find((group) => group.id === selectedGroupId) ?? groups[0];
  const query = search.trim().toLowerCase();
  const visibleGroups = groups.filter((group) =>
    `${group.displayName} ${group.name} ${group.path}`.toLowerCase().includes(query),
  );
  const trimmedPath = path.trim();
  const existing = groups.find(
    (group) => group.path === (trimmedPath.startsWith("/") ? trimmedPath : `/${trimmedPath}`),
  );

  const select = (groupId: string) => {
    if (groupId !== selectedGroup?.id) void guard(() => setSelectedGroupId(groupId));
  };

  const handleSave = async (event: FormEvent) => {
    event.preventDefault();
    if (!trimmedPath || saving) return;
    try {
      setError(null);
      const group = await upsertGroup({
        path: trimmedPath,
        displayName,
        enabled: true,
      }).unwrap();
      setPath("");
      setDisplayName("");
      toast.success(existing ? "Group updated" : "Group created", group.path);
      void guard(() => setSelectedGroupId(group.id));
    } catch (failure) {
      setError(getApiErrorMessage(failure, "Unable to create or update group."));
    }
  };

  return (
    <section className="rbac-card" aria-labelledby="rbac-groups-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-groups-title">Groups</h2>
          <p>Map Keycloak groups to realm roles, system roles, and direct permissions.</p>
        </div>
      </div>
      <div className="rbac-split">
        <div className="rbac-split__list">
          <div className="rbac-split__search">
            <Search
              size="md"
              labelText="Find groups"
              placeholder="Find a group"
              closeButtonLabelText="Clear group search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          <ul className="rbac-role-list" aria-label="Groups">
            {visibleGroups.map((group) => {
              const active = group.id === selectedGroup?.id;
              return (
                <li className={`rbac-role-list__item${active ? " is-active" : ""}`} key={group.id}>
                  <button
                    type="button"
                    className="rbac-role-list__select"
                    aria-current={active ? "true" : undefined}
                    onClick={() => select(group.id)}
                  >
                    <span>{group.displayName || group.name}</span>
                    <small>
                      {group.path} · {pluralize(group.memberCount, "member")}
                    </small>
                  </button>
                </li>
              );
            })}
          </ul>
          {visibleGroups.length === 0 && (
            <p className="rbac-empty">
              {groups.length ? "No groups match." : "No groups have been synced or created yet."}
            </p>
          )}
          <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
            <form className="rbac-create-form" onSubmit={(event) => void handleSave(event)}>
              <h3>Add or update a group</h3>
              <TextInput
                id="rbac-new-group-path"
                labelText="Group path"
                helperText={
                  existing
                    ? "This group exists; saving updates its display name."
                    : "The full Keycloak group path, for example “/data-statistics/users”."
                }
                value={path}
                disabled={!canWriteRoles || saving}
                onChange={(event) => setPath(event.target.value)}
              />
              <TextInput
                id="rbac-new-group-display"
                labelText="Display name (optional)"
                value={displayName}
                disabled={!canWriteRoles || saving}
                onChange={(event) => setDisplayName(event.target.value)}
              />
              <Button
                type="submit"
                renderIcon={Add}
                size="md"
                disabled={!canWriteRoles || saving || !trimmedPath}
              >
                {existing ? "Update group" : "Add group"}
              </Button>
              <ActionError title="Group not saved" message={error} onClose={() => setError(null)} />
            </form>
          </PermissionGuard>
        </div>
        <div className="rbac-split__detail">
          {selectedGroup ? (
            <GroupDetail
              key={selectedGroup.id}
              group={selectedGroup}
              users={users}
              systems={systems}
              categories={categories}
              canWriteRoles={canWriteRoles}
              canWritePermissions={canWritePermissions}
            />
          ) : (
            <p className="rbac-empty">No groups have been synced or created yet.</p>
          )}
        </div>
      </div>
    </section>
  );
}
