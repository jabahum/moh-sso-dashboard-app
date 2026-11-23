import { useState, type FormEvent } from "react";
import { Button, TextInput } from "@carbon/react";
import { Add, TrashCan } from "@carbon/react/icons";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { useToast } from "@moh-sso/ui";

import { useAddSystemAccessRoleMutation, useRemoveSystemAccessRoleMutation } from "../../api";
import type { RbacSystemDetail } from "../../types";
import { ActionError, useConfirm } from "./feedback";
import { getApiErrorMessage } from "./workspace-helpers";

type AccessRolesPanelProps = {
  system: RbacSystemDetail;
  canWriteRoles: boolean;
};

export function AccessRolesPanel({ system, canWriteRoles }: AccessRolesPanelProps) {
  const toast = useToast();
  const confirm = useConfirm();
  const [addAccessRole, { isLoading: adding }] = useAddSystemAccessRoleMutation();
  const [removeAccessRole, { isLoading: removing }] = useRemoveSystemAccessRoleMutation();
  const [roleName, setRoleName] = useState("");
  const [touched, setTouched] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [removeError, setRemoveError] = useState<string | null>(null);

  const trimmed = roleName.trim();
  const duplicate = system.accessRoles.includes(trimmed);
  const invalidText = !trimmed
    ? "Enter a role name."
    : duplicate
      ? "This role already grants access."
      : "";

  const handleAdd = async (event: FormEvent) => {
    event.preventDefault();
    setTouched(true);
    if (invalidText || adding) return;
    try {
      setAddError(null);
      await addAccessRole({ clientId: system.clientId, data: { roleName: trimmed } }).unwrap();
      setRoleName("");
      setTouched(false);
      toast.success("Access role added", `${trimmed} now grants access to ${system.displayName}.`);
    } catch (failure) {
      setAddError(getApiErrorMessage(failure, "Unable to add access role."));
    }
  };

  const handleRemove = async (role: string) => {
    const confirmed = await confirm({
      title: `Remove access role “${role}”?`,
      body: (
        <p>
          Users who reach <strong>{system.displayName}</strong> only through <code>{role}</code>{" "}
          lose access to it. The Keycloak role itself is not deleted.
        </p>
      ),
      confirmLabel: "Remove access role",
      danger: true,
    });
    if (!confirmed) return;
    try {
      setRemoveError(null);
      await removeAccessRole({ clientId: system.clientId, roleName: role }).unwrap();
      toast.success("Access role removed", `${role} no longer grants access.`);
    } catch (failure) {
      setRemoveError(
        `${getApiErrorMessage(failure, "Unable to remove access role.")} Use backend force mode only after review.`,
      );
    }
  };

  return (
    <section className="rbac-card" aria-labelledby="rbac-access-roles-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-access-roles-title">Access roles</h2>
          <p>Users who hold any of these Keycloak client roles can open {system.displayName}.</p>
        </div>
      </div>
      <div className="rbac-card__body">
        {system.accessRoles.length ? (
          <ul className="rbac-item-list" aria-label="Access roles">
            {system.accessRoles.map((role) => (
              <li key={role} className="rbac-item-list__row">
                <code>{role}</code>
                <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
                  <Button
                    kind="ghost"
                    size="sm"
                    renderIcon={TrashCan}
                    disabled={removing}
                    onClick={() => void handleRemove(role)}
                  >
                    Remove<span className="cds--visually-hidden"> {role}</span>
                  </Button>
                </PermissionGuard>
              </li>
            ))}
          </ul>
        ) : (
          <p className="rbac-empty">
            No access roles yet. Users cannot open this application until at least one is added.
          </p>
        )}
        <ActionError
          title="Access role not removed"
          message={removeError}
          onClose={() => setRemoveError(null)}
        />

        <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
          <form className="rbac-create-form" onSubmit={(event) => void handleAdd(event)}>
            <h3>Add an access role</h3>
            <div className="rbac-create-form__fields">
              <TextInput
                id="rbac-new-access-role"
                labelText="Keycloak client role name"
                helperText="An existing client role on this application, for example “viewer”."
                value={roleName}
                disabled={!canWriteRoles || adding}
                invalid={touched && Boolean(invalidText) && Boolean(roleName)}
                invalidText={invalidText}
                onBlur={() => setTouched(true)}
                onChange={(event) => setRoleName(event.target.value)}
              />
            </div>
            <Button
              type="submit"
              renderIcon={Add}
              size="md"
              disabled={!canWriteRoles || Boolean(invalidText) || adding}
            >
              Add access role
            </Button>
          </form>
          <ActionError
            title="Access role not added"
            message={addError}
            onClose={() => setAddError(null)}
          />
        </PermissionGuard>
      </div>
    </section>
  );
}
