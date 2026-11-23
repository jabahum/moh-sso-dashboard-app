import { useState, type FormEvent } from "react";
import { Button, InlineLoading, Tag, TextInput } from "@carbon/react";
import { Add, TrashCan } from "@carbon/react/icons";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { useToast } from "@moh-sso/ui";

import {
  useAssignSystemRolePermissionMutation,
  useCreateSystemRoleMutation,
  useDeleteSystemRoleMutation,
  usePreviewRbacChangeMutation,
  useRemoveSystemRolePermissionMutation,
} from "../../api";
import type { RbacSystemDetail, RbacSystemRole } from "../../types";
import { ActionError, useConfirm, useKeySet, useUnsavedGuard } from "./feedback";
import { StagedPermissionEditor } from "./PermissionMatrix";
import { getApiErrorMessage, pluralize, type PermissionCategory } from "./workspace-helpers";

type SystemRolesPanelProps = {
  system: RbacSystemDetail;
  categories: PermissionCategory[];
  canWriteRoles: boolean;
  canWritePermissions: boolean;
};

export function SystemRolesPanel({
  system,
  categories,
  canWriteRoles,
  canWritePermissions,
}: SystemRolesPanelProps) {
  const toast = useToast();
  const confirm = useConfirm();
  const guard = useUnsavedGuard();
  const [createRole, { isLoading: creating }] = useCreateSystemRoleMutation();
  const [deleteRole] = useDeleteSystemRoleMutation();
  const [previewChange] = usePreviewRbacChangeMutation();
  const [assignPermission] = useAssignSystemRolePermissionMutation();
  const [removePermission] = useRemoveSystemRolePermissionMutation();

  const [selectedRoleId, setSelectedRoleId] = useState("");
  const [roleName, setRoleName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [touched, setTouched] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingRoleId, setDeletingRoleId] = useState("");

  const roles = system.roles;
  const selectedRole = roles.find((role) => role.id === selectedRoleId) ?? roles[0];
  const savedKeys = useKeySet(selectedRole?.permissions.map((permission) => permission.key));

  const trimmedName = roleName.trim();
  const nameInvalidText = !trimmedName
    ? "Enter a role name."
    : roles.some((role) => role.name === trimmedName)
      ? "A role with this name already exists."
      : "";

  const handleCreate = async (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    if (nameInvalidText || creating) return;
    try {
      setCreateError(null);
      const role = await createRole({
        clientId: system.clientId,
        data: { name: trimmedName, displayName: displayName.trim(), enabled: true },
      }).unwrap();
      setRoleName("");
      setDisplayName("");
      setTouched(false);
      toast.success("Role created", `${role.displayName || role.name} has no permissions yet.`);
      void guard(() => setSelectedRoleId(role.id));
    } catch (failure) {
      setCreateError(getApiErrorMessage(failure, "Unable to create system role."));
    }
  };

  const handleDelete = async (role: RbacSystemRole) => {
    setDeleteError(null);
    setDeletingRoleId(role.id);
    let preview;
    try {
      preview = await previewChange({
        action: "delete-role",
        resourceType: "system-role",
        resourceId: role.id,
        systemClientId: role.clientId,
        roleName: role.name,
      }).unwrap();
    } catch (failure) {
      setDeletingRoleId("");
      setDeleteError(
        getApiErrorMessage(failure, "Unable to preview the impact of deleting this role."),
      );
      return;
    }
    const confirmed = await confirm({
      title: `Delete role “${role.displayName || role.name}”?`,
      body: (
        <>
          <p>
            Risk level:{" "}
            <Tag type={preview.highRisk ? "red" : "gray"} size="sm">
              {preview.riskLevel}
            </Tag>
          </p>
          {preview.affectedUsersCount > 0 && (
            <p>{pluralize(preview.affectedUsersCount, "user")} may lose permissions.</p>
          )}
          {preview.warnings.length > 0 && (
            <ul className="rbac-confirm__list">
              {preview.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          )}
          <p>This cannot be undone.</p>
        </>
      ),
      confirmLabel: "Delete role",
      danger: true,
    });
    if (!confirmed) {
      setDeletingRoleId("");
      return;
    }
    try {
      await deleteRole({ roleId: role.id, clientId: role.clientId }).unwrap();
      toast.success("Role deleted", `${role.displayName || role.name} was deleted.`);
    } catch (failure) {
      setDeleteError(getApiErrorMessage(failure, "Unable to delete system role."));
    } finally {
      setDeletingRoleId("");
    }
  };

  return (
    <section className="rbac-card" aria-labelledby="rbac-system-roles-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-system-roles-title">System roles</h2>
          <p>Choose a role, tick the permissions it grants, then save.</p>
        </div>
      </div>

      <div className="rbac-split">
        <div className="rbac-split__list">
          <ul className="rbac-role-list" aria-label="Roles">
            {roles.map((role) => {
              const active = role.id === selectedRole?.id;
              return (
                <li className={`rbac-role-list__item${active ? " is-active" : ""}`} key={role.id}>
                  <button
                    type="button"
                    className="rbac-role-list__select"
                    aria-current={active ? "true" : undefined}
                    onClick={() => {
                      if (!active) void guard(() => setSelectedRoleId(role.id));
                    }}
                  >
                    <span>{role.displayName || role.name}</span>
                    <small>
                      {role.name}
                      {" · "}
                      {pluralize(role.permissions.length, "permission")}
                    </small>
                    {role.serviceOnly && (
                      <Tag size="sm" type="purple">
                        Service only
                      </Tag>
                    )}
                  </button>
                  <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                    {deletingRoleId === role.id ? (
                      <InlineLoading className="rbac-role-list__busy" description="" />
                    ) : (
                      <Button
                        hasIconOnly
                        iconDescription={`Delete role ${role.displayName || role.name}`}
                        tooltipPosition="left"
                        kind="ghost"
                        renderIcon={TrashCan}
                        size="sm"
                        disabled={Boolean(deletingRoleId)}
                        onClick={() => void handleDelete(role)}
                      />
                    )}
                  </PermissionGuard>
                </li>
              );
            })}
          </ul>
          {roles.length === 0 && (
            <p className="rbac-empty">No roles yet. Create one below to assign permissions.</p>
          )}
          <ActionError
            title="Role not deleted"
            message={deleteError}
            onClose={() => setDeleteError(null)}
          />

          <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
            <form className="rbac-create-form" onSubmit={(event) => void handleCreate(event)}>
              <h3>New role</h3>
              <TextInput
                id="rbac-new-role-name"
                labelText="Role name"
                helperText="The Keycloak client role name, for example “data_viewer”."
                value={roleName}
                disabled={!canWriteRoles || creating}
                invalid={touched && Boolean(roleName) && Boolean(nameInvalidText)}
                invalidText={nameInvalidText}
                onBlur={() => setTouched(true)}
                onChange={(event) => setRoleName(event.target.value)}
              />
              <TextInput
                id="rbac-new-role-display"
                labelText="Display name (optional)"
                helperText="A friendly name shown in the portal."
                value={displayName}
                disabled={!canWriteRoles || creating}
                onChange={(event) => setDisplayName(event.target.value)}
              />
              <Button
                type="submit"
                renderIcon={Add}
                size="md"
                disabled={!canWriteRoles || Boolean(nameInvalidText) || creating}
              >
                Create role
              </Button>
              <ActionError
                title="Role not created"
                message={createError}
                onClose={() => setCreateError(null)}
              />
            </form>
          </PermissionGuard>
        </div>

        <div className="rbac-split__detail">
          {selectedRole ? (
            <>
              <div className="rbac-split__detail-header">
                <h3>{selectedRole.displayName || selectedRole.name}</h3>
                <code>{selectedRole.name}</code>
              </div>
              <StagedPermissionEditor
                key={selectedRole.id}
                categories={categories}
                savedKeys={savedKeys}
                subjectLabel={`role “${selectedRole.displayName || selectedRole.name}”`}
                canEdit={canWritePermissions}
                readOnlyReason="Changing permissions requires the rbac:permissions:write permission."
                onAssign={(permissionKey) =>
                  assignPermission({
                    roleId: selectedRole.id,
                    clientId: selectedRole.clientId,
                    data: { permissionKey },
                  }).unwrap()
                }
                onRemove={(permissionKey) =>
                  removePermission({
                    roleId: selectedRole.id,
                    clientId: selectedRole.clientId,
                    permissionKey,
                  }).unwrap()
                }
              />
            </>
          ) : (
            <p className="rbac-empty">Select or create a role to manage its permissions.</p>
          )}
        </div>
      </div>
    </section>
  );
}
