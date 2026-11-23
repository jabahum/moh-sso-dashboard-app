import { Button, Tag, TextInput } from "@carbon/react";
import { useState } from "react";

import {
  useBulkAssignPermissionMutation,
  useBulkRemovePermissionMutation,
  useCopyRolePermissionsMutation,
  useCreateRoleFromTemplateMutation,
  useListSystemRolesQuery,
} from "../api";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import type { RbacRoleTemplate } from "../types";
import { useConfirm } from "./workspace/feedback";

type RoleTemplatesPanelProps = {
  templates: RbacRoleTemplate[];
  activeClientId: string;
  onMessage: (message: string) => void;
};

export function RoleTemplatesPanel({
  templates,
  activeClientId,
  onMessage,
}: RoleTemplatesPanelProps) {
  const confirmAction = useConfirm();
  const [selectedTemplate, setSelectedTemplate] = useState("");
  const [roleName, setRoleName] = useState("");
  const [sourceRoleId, setSourceRoleId] = useState("");
  const [targetRoleId, setTargetRoleId] = useState("");
  const [bulkRoleIds, setBulkRoleIds] = useState("");
  const [bulkPermissionKey, setBulkPermissionKey] = useState("");
  const { data: roles = [] } = useListSystemRolesQuery(activeClientId, { skip: !activeClientId });
  const [createRoleFromTemplate] = useCreateRoleFromTemplateMutation();
  const [copyPermissions] = useCopyRolePermissionsMutation();
  const [bulkAssignPermission] = useBulkAssignPermissionMutation();
  const [bulkRemovePermission] = useBulkRemovePermissionMutation();

  const handleCreateRole = async () => {
    if (!activeClientId || !selectedTemplate) return;
    await createRoleFromTemplate({
      clientId: activeClientId,
      data: { templateName: selectedTemplate, roleName },
    }).unwrap();
    onMessage("Role created from template.");
    setRoleName("");
  };

  const parsedRoleIds = () =>
    bulkRoleIds
      .split(",")
      .map((role) => role.trim())
      .filter(Boolean);

  const handleCopyPermissions = async () => {
    if (!targetRoleId || !sourceRoleId) return;
    const targetRole = roles.find((role) => role.id === targetRoleId);
    await copyPermissions({
      roleId: targetRoleId,
      clientId: targetRole?.clientId || activeClientId,
      data: { sourceRoleId },
    }).unwrap();
    onMessage("Permissions copied.");
  };

  const handleBulkAssign = async () => {
    await bulkAssignPermission({
      roleIds: parsedRoleIds(),
      permissionKey: bulkPermissionKey,
    }).unwrap();
    onMessage("Permission assigned to selected roles.");
  };

  const handleBulkRemove = async () => {
    const confirmed = await confirmAction({
      title: "Remove permission from roles?",
      body: (
        <p>
          Remove <code>{bulkPermissionKey}</code> from {parsedRoleIds().length} roles?
        </p>
      ),
      confirmLabel: "Remove permission",
      danger: true,
    });
    if (!confirmed) return;
    await bulkRemovePermission({
      roleIds: parsedRoleIds(),
      permissionKey: bulkPermissionKey,
    }).unwrap();
    onMessage("Permission removed from selected roles.");
  };

  return (
    <section>
      <h3>Templates & Bulk</h3>
      <div className="rbac-effective-tags">
        {templates.map((template) => (
          <Tag
            key={template.name}
            type={selectedTemplate === template.name ? "blue" : "purple"}
            onClick={() => setSelectedTemplate(template.name)}
          >
            {template.displayName}
          </Tag>
        ))}
      </div>
      <TextInput
        id="rbac-template-role-name"
        labelText="Role name override"
        placeholder={selectedTemplate || "viewer"}
        value={roleName}
        onChange={(event) => setRoleName(event.target.value)}
      />
      <PermissionGuard permission={PERMISSIONS.rbacRolesWrite}>
        <Button
          size="sm"
          disabled={!activeClientId || !selectedTemplate}
          onClick={handleCreateRole}
        >
          Create role from template
        </Button>
      </PermissionGuard>
      <div className="rbac-bulk-grid">
        <TextInput
          id="rbac-copy-source-role"
          labelText="Copy from role ID"
          value={sourceRoleId}
          onChange={(event) => setSourceRoleId(event.target.value)}
        />
        <TextInput
          id="rbac-copy-target-role"
          labelText="Copy to role ID"
          value={targetRoleId}
          onChange={(event) => setTargetRoleId(event.target.value)}
        />
        <PermissionGuard permission={PERMISSIONS.rbacPermissionsWrite}>
          <Button
            size="sm"
            disabled={!sourceRoleId || !targetRoleId}
            onClick={handleCopyPermissions}
          >
            Copy permissions
          </Button>
        </PermissionGuard>
      </div>
      <div className="rbac-bulk-grid">
        <TextInput
          id="rbac-bulk-role-ids"
          labelText="Role IDs, comma separated"
          value={bulkRoleIds}
          onChange={(event) => setBulkRoleIds(event.target.value)}
        />
        <TextInput
          id="rbac-bulk-permission"
          labelText="Permission key"
          value={bulkPermissionKey}
          onChange={(event) => setBulkPermissionKey(event.target.value)}
        />
        <PermissionGuard permission={PERMISSIONS.rbacPermissionsWrite}>
          <div className="rbac-sync-actions">
            <Button
              size="sm"
              disabled={parsedRoleIds().length === 0 || !bulkPermissionKey}
              onClick={handleBulkAssign}
            >
              Bulk assign
            </Button>
            <Button
              size="sm"
              kind="danger--tertiary"
              disabled={parsedRoleIds().length === 0 || !bulkPermissionKey}
              onClick={handleBulkRemove}
            >
              Bulk remove
            </Button>
          </div>
        </PermissionGuard>
      </div>
      <small>
        {roles.length} roles available for {activeClientId || "selected system"}.
      </small>
      <small>
        {activeClientId
          ? `Templates apply to ${activeClientId}.`
          : "Select a system before applying a template."}
      </small>
    </section>
  );
}
