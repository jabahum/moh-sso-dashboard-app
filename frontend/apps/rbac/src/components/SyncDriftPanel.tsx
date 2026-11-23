import { useMemo, useState } from "react";
import { Button, InlineLoading, InlineNotification, Tag, TextArea } from "@carbon/react";
import { Renew, Save } from "@carbon/react/icons";

import { useApplyRbacSyncMutation, useGetRbacDriftQuery, usePreviewRbacSyncMutation } from "../api";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import type { RbacDriftReport, RbacSyncPayload, RbacSyncPreview } from "../types";
import { useConfirm } from "./workspace/feedback";

type SyncDriftPanelProps = {
  onSynced?: () => void;
};

export function SyncDriftPanel({ onSynced }: SyncDriftPanelProps) {
  const confirmAction = useConfirm();
  const { data: defaultReport, isFetching, refetch } = useGetRbacDriftQuery();
  const [realmExportText, setRealmExportText] = useState("");
  const [preview, setPreview] = useState<RbacSyncPreview | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);

  const [previewSync, { isLoading: previewing }] = usePreviewRbacSyncMutation();
  const [applySync, { isLoading: applying }] = useApplyRbacSyncMutation();

  const activeReport = preview?.report ?? defaultReport;
  const hasRealmExport = realmExportText.trim().length > 0;

  const parsedPayload = useMemo<RbacSyncPayload | null>(() => {
    if (!hasRealmExport) return null;
    try {
      return {
        source: "realm-export",
        realmExport: JSON.parse(realmExportText),
      };
    } catch {
      return null;
    }
  }, [hasRealmExport, realmExportText]);

  const handlePreview = async () => {
    if (!parsedPayload) {
      setLocalError("Paste a valid Keycloak realm-export JSON object before previewing sync.");
      return;
    }

    try {
      setLocalError(null);
      const result = await previewSync(parsedPayload).unwrap();
      setPreview(result);
    } catch {
      setLocalError("Unable to preview RBAC sync from the realm export.");
    }
  };

  const handleApply = async () => {
    if (!parsedPayload) {
      setLocalError("Paste a valid Keycloak realm-export JSON object before applying sync.");
      return;
    }
    const confirmed = await confirmAction({
      title: "Apply Keycloak sync to portal RBAC?",
      body: <p>New roles will not receive permissions automatically.</p>,
      confirmLabel: "Apply sync",
    });
    if (!confirmed) return;

    try {
      setLocalError(null);
      const result = await applySync(parsedPayload).unwrap();
      setPreview(result.preview);
      onSynced?.();
      refetch();
    } catch {
      setLocalError("Unable to apply RBAC sync from the realm export.");
    }
  };

  return (
    <section className="rbac-card">
      <div className="rbac-page__panel-header">
        <div>
          <h2>Sync & Drift</h2>
          <p>Compare portal RBAC with Keycloak clients and client roles.</p>
        </div>
        <Button kind="ghost" renderIcon={Renew} size="sm" onClick={() => refetch()}>
          Refresh
        </Button>
      </div>

      {localError && (
        <InlineNotification
          kind="error"
          lowContrast
          title="Sync check failed"
          subtitle={localError}
          onClose={() => setLocalError(null)}
        />
      )}

      {isFetching && <InlineLoading description="Checking RBAC drift..." />}

      {activeReport && <DriftReport report={activeReport} preview={preview} />}

      <div className="rbac-sync-grid">
        <TextArea
          id="rbac-realm-export"
          labelText="Keycloak realm-export.json"
          helperText="Paste a realm export to preview or apply a system/role sync. Permissions stay unmapped until assigned."
          value={realmExportText}
          rows={8}
          onChange={(event) => setRealmExportText(event.target.value)}
        />
        <div className="rbac-sync-actions">
          <Button disabled={!hasRealmExport || previewing} onClick={handlePreview}>
            Preview sync
          </Button>
          <PermissionGuard permission={PERMISSIONS.rbacWrite}>
            <Button
              disabled={!hasRealmExport || applying}
              kind="primary"
              renderIcon={Save}
              onClick={handleApply}
            >
              Apply sync
            </Button>
          </PermissionGuard>
        </div>
      </div>
    </section>
  );
}

function DriftReport({
  report,
  preview,
}: {
  report: RbacDriftReport;
  preview: RbacSyncPreview | null;
}) {
  const warnings = report.warnings ?? [];
  const systems = report.systems ?? [];

  return (
    <div className="rbac-drift">
      {warnings.map((warning) => (
        <InlineNotification
          key={warning}
          kind="warning"
          lowContrast
          title="Drift source warning"
          subtitle={warning}
        />
      ))}

      <div className="rbac-drift-summary">
        <SummaryTile label="Systems in sync" value={report.summary.systemsInSync} />
        <SummaryTile label="Missing systems" value={report.summary.systemsMissingInRbac} />
        <SummaryTile label="Stale systems" value={report.summary.systemsStaleInRbac} />
        <SummaryTile label="Missing roles" value={report.summary.rolesMissingInRbac} />
        <SummaryTile label="Stale roles" value={report.summary.rolesStaleInRbac} />
        <SummaryTile label="Realm roles" value={report.summary.realmRolesMissing} />
      </div>

      {preview && (
        <div className="rbac-sync-preview">
          <Tag type="blue">{preview.systemsToCreate} systems to create</Tag>
          <Tag type="cyan">{preview.systemsToUpdate} systems to update</Tag>
          <Tag type="purple">{preview.rolesToCreate} roles to create</Tag>
          <Tag type="green">{preview.accessRolesToAdd} access roles to add</Tag>
        </div>
      )}

      <div className="rbac-drift-table">
        {systems.map((system) => {
          const missingRoles = system.missingRolesInRbac ?? [];
          const staleRoles = system.staleRolesInRbac ?? [];

          return (
            <div className="rbac-drift-row" key={system.clientId}>
              <div>
                <strong>{system.displayName || system.clientId}</strong>
                <small>{system.clientId}</small>
              </div>
              <Tag type={system.rbacStatus === "present" ? "green" : "red"}>
                RBAC {system.rbacStatus}
              </Tag>
              <Tag type={system.keycloakStatus === "present" ? "green" : "red"}>
                Keycloak {system.keycloakStatus}
              </Tag>
              <small>Missing roles: {missingRoles.join(", ") || "none"}</small>
              <small>Stale roles: {staleRoles.join(", ") || "none"}</small>
            </div>
          );
        })}
        {systems.length === 0 && <small>No drift rows to display.</small>}
      </div>
    </div>
  );
}

function SummaryTile({ label, value }: { label: string; value: number }) {
  return (
    <div className="rbac-summary-tile">
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}
