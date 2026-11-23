import { useState, type FormEvent } from "react";
import { Button, Search, Tag, TextInput } from "@carbon/react";
import { ArrowRight } from "@carbon/react/icons";

import {
  useAssignRealmRolePermissionMutation,
  useListRealmRolePermissionsQuery,
  useRemoveRealmRolePermissionMutation,
} from "../../api";
import { useKeySet, useUnsavedGuard } from "./feedback";
import { StagedPermissionEditor } from "./PermissionMatrix";
import { pluralize, type PermissionCategory } from "./workspace-helpers";

type RealmPermissionsPanelProps = {
  categories: PermissionCategory[];
  canWritePermissions: boolean;
};

export function RealmPermissionsPanel({
  categories,
  canWritePermissions,
}: RealmPermissionsPanelProps) {
  const guard = useUnsavedGuard();
  const { data: realmRoles = [] } = useListRealmRolePermissionsQuery();
  const [assignPermission] = useAssignRealmRolePermissionMutation();
  const [removePermission] = useRemoveRealmRolePermissionMutation();
  const [selected, setSelected] = useState("");
  const [search, setSearch] = useState("");
  const [otherRole, setOtherRole] = useState("");

  const selectedRealmRole = selected || realmRoles[0]?.realmRole || "";
  const selectedGroup = realmRoles.find((group) => group.realmRole === selectedRealmRole);
  const savedKeys = useKeySet(selectedGroup?.permissions.map((permission) => permission.key));
  const isNewRole = Boolean(selectedRealmRole) && !selectedGroup;

  const query = search.trim().toLowerCase();
  const visibleRoles = realmRoles.filter((group) => group.realmRole.toLowerCase().includes(query));

  const select = (realmRole: string) => {
    if (realmRole !== selectedRealmRole) void guard(() => setSelected(realmRole));
  };

  const openOtherRole = (event: FormEvent) => {
    event.preventDefault();
    const realmRole = otherRole.trim();
    if (!realmRole) return;
    setOtherRole("");
    select(realmRole);
  };

  return (
    <section className="rbac-card" aria-labelledby="rbac-realm-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-realm-title">Realm permissions</h2>
          <p>Permissions granted to everyone who holds a Keycloak realm role, across the portal.</p>
        </div>
      </div>
      <div className="rbac-split">
        <div className="rbac-split__list">
          <div className="rbac-split__search">
            <Search
              size="md"
              labelText="Find realm roles"
              placeholder="Find a realm role"
              closeButtonLabelText="Clear realm role search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          <ul className="rbac-role-list" aria-label="Realm roles">
            {isNewRole && (
              <li className="rbac-role-list__item is-active">
                <button type="button" className="rbac-role-list__select" aria-current="true">
                  <span>{selectedRealmRole}</span>
                  <small>No permissions yet</small>
                </button>
              </li>
            )}
            {visibleRoles.map((group) => {
              const active = group.realmRole === selectedRealmRole;
              return (
                <li
                  className={`rbac-role-list__item${active ? " is-active" : ""}`}
                  key={group.realmRole}
                >
                  <button
                    type="button"
                    className="rbac-role-list__select"
                    aria-current={active ? "true" : undefined}
                    onClick={() => select(group.realmRole)}
                  >
                    <span>{group.realmRole}</span>
                    <small>{pluralize(group.permissions.length, "permission")}</small>
                  </button>
                </li>
              );
            })}
          </ul>
          {visibleRoles.length === 0 && (
            <p className="rbac-empty">
              {realmRoles.length ? "No realm roles match." : "No realm roles have permissions yet."}
            </p>
          )}
          <form className="rbac-create-form" onSubmit={openOtherRole}>
            <h3>Another realm role</h3>
            <TextInput
              id="rbac-realm-role-name"
              labelText="Realm role name"
              helperText="A Keycloak realm role that is not listed yet, for example “admin”."
              value={otherRole}
              disabled={!canWritePermissions}
              onChange={(event) => setOtherRole(event.target.value)}
            />
            <Button
              type="submit"
              kind="tertiary"
              size="md"
              renderIcon={ArrowRight}
              disabled={!canWritePermissions || !otherRole.trim()}
            >
              Open realm role
            </Button>
          </form>
        </div>
        <div className="rbac-split__detail">
          {selectedRealmRole ? (
            <>
              <div className="rbac-split__detail-header">
                <h3>{selectedRealmRole}</h3>
                {isNewRole && (
                  <Tag type="gray" size="sm">
                    Not saved yet
                  </Tag>
                )}
              </div>
              <StagedPermissionEditor
                key={selectedRealmRole}
                categories={categories}
                savedKeys={savedKeys}
                subjectLabel={`realm role “${selectedRealmRole}”`}
                canEdit={canWritePermissions}
                readOnlyReason="Changing permissions requires the rbac:permissions:write permission."
                onAssign={(permissionKey) =>
                  assignPermission({
                    realmRole: selectedRealmRole,
                    data: { permissionKey },
                  }).unwrap()
                }
                onRemove={(permissionKey) =>
                  removePermission({ realmRole: selectedRealmRole, permissionKey }).unwrap()
                }
              />
            </>
          ) : (
            <p className="rbac-empty">Select a realm role to manage its permissions.</p>
          )}
        </div>
      </div>
    </section>
  );
}
