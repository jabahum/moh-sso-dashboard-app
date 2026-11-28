import { useEffect, useState } from "react";
import {
  Button,
  Checkbox,
  InlineLoading,
  InlineNotification,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tag,
  TextInput,
  Toggle,
} from "@carbon/react";
import { Add, Edit, Save } from "@carbon/react/icons";

import {
  useListPlatformScopesQuery,
  useListServiceIdentitiesQuery,
  useResolveServiceAccountMutation,
  useSaveServiceIdentityMutation,
} from "../api";
import type { PlatformScope, ServiceIdentity } from "../types";
import { getIdentitySyncErrorMessage } from "./IdentitySyncPanel";

type Props = {
  clientId: string;
  /** Binding needs both rbac:write and rbac:roles:write. */
  canWrite: boolean;
};

type Draft = {
  editing: boolean;
  oauthClientId: string;
  subject: string;
  scopes: string[];
  enabled: boolean;
  reason: string;
};

const emptyDraft: Draft = {
  editing: false,
  oauthClientId: "",
  subject: "",
  scopes: [],
  enabled: true,
  reason: "",
};

/**
 * Machine clients bound to an application and the platform scopes each may use.
 * Scope names and descriptions come from the portal's scope catalogue.
 */
export function ServiceIdentitiesPanel({ clientId, canWrite }: Props) {
  if (!clientId) {
    return (
      <section className="rbac-card" aria-label="Machine clients">
        <p className="rbac-identity-sync__muted">Select a saved application first.</p>
      </section>
    );
  }
  if (!canWrite) {
    return (
      <section className="rbac-card" aria-label="Machine clients">
        <p className="rbac-identity-sync__muted">
          Viewing and binding machine clients needs both RBAC write and role write permissions.
        </p>
      </section>
    );
  }
  return <ServiceIdentitiesCard key={clientId} clientId={clientId} />;
}

function ServiceIdentitiesCard({ clientId }: { clientId: string }) {
  const { data: identities, isLoading, error } = useListServiceIdentitiesQuery(clientId);
  const { data: catalogue, error: catalogueError } = useListPlatformScopesQuery();
  const [resolve, { isLoading: resolving }] = useResolveServiceAccountMutation();
  const [save, { isLoading: saving }] = useSaveServiceIdentityMutation();
  const [draft, setDraft] = useState<Draft | null>(null);
  const [message, setMessage] = useState<{ kind: "error" | "success"; text: string } | null>(null);

  useEffect(() => {
    setDraft(null);
    setMessage(null);
  }, [clientId]);

  const machineScopes = (catalogue ?? []).filter((scope) => scope.tokens.includes("machine"));
  const describe = (name: string) => catalogue?.find((scope) => scope.name === name)?.description;
  const update = (patch: Partial<Draft>) =>
    setDraft((current) => (current ? { ...current, ...patch } : current));

  const startNew = () => {
    setMessage(null);
    setDraft({ ...emptyDraft });
  };

  const startEdit = (identity: ServiceIdentity) => {
    setMessage(null);
    setDraft({
      editing: true,
      oauthClientId: identity.clientId,
      subject: identity.subject,
      scopes: [...identity.scopes],
      enabled: identity.enabled,
      reason: "",
    });
  };

  // The subject is looked up in Keycloak, never typed by hand.
  const handleResolve = async () => {
    if (!draft) return;
    setMessage(null);
    try {
      const result = await resolve({
        clientId,
        oauthClientId: draft.oauthClientId.trim(),
      }).unwrap();
      update({ subject: result.subject });
    } catch (err) {
      update({ subject: "" });
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to find the client's service account."),
      });
    }
  };

  const handleSave = async () => {
    if (!draft) return;
    setMessage(null);
    try {
      await save({
        clientId,
        oauthClientId: draft.oauthClientId.trim(),
        data: {
          subject: draft.subject,
          scopes: draft.scopes,
          enabled: draft.enabled,
          reason: draft.reason.trim(),
        },
      }).unwrap();
      setDraft(null);
      setMessage({
        kind: "success",
        text: `${draft.oauthClientId.trim()} takes effect on its next request.`,
      });
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to save the machine client."),
      });
    }
  };

  const toggleScope = (name: string, checked: boolean) =>
    update({
      scopes: checked
        ? [...(draft?.scopes ?? []), name]
        : (draft?.scopes ?? []).filter((scope) => scope !== name),
    });

  const canSave =
    !!draft &&
    draft.oauthClientId.trim() !== "" &&
    draft.subject !== "" &&
    draft.reason.trim() !== "" &&
    !saving;

  return (
    <section className="rbac-card" aria-label="Machine clients">
      <div className="rbac-page__panel-header">
        <h2>Machine clients</h2>
        {!draft && (
          <Button size="sm" kind="tertiary" renderIcon={Add} onClick={startNew}>
            Bind a machine client
          </Button>
        )}
      </div>
      <p>
        Backend service accounts this application uses to call the portal&apos;s platform API. Each
        client can use only the scopes granted here, and only while the application is active and
        enabled. Changes take effect on the next request and are audited.
      </p>
      {message && (
        <InlineNotification
          kind={message.kind}
          lowContrast
          hideCloseButton
          title={message.kind === "error" ? "Not saved" : "Saved"}
          subtitle={message.text}
        />
      )}
      {catalogueError && (
        <InlineNotification
          kind="warning"
          lowContrast
          hideCloseButton
          title="Scope descriptions unavailable"
          subtitle={getIdentitySyncErrorMessage(
            catalogueError,
            "The scope catalogue could not be loaded.",
          )}
        />
      )}

      {isLoading ? (
        <InlineLoading description="Loading machine clients..." />
      ) : error ? (
        <InlineNotification
          kind="error"
          lowContrast
          hideCloseButton
          title="Machine clients unavailable"
          subtitle={getIdentitySyncErrorMessage(error, "Unable to load machine clients.")}
        />
      ) : identities && identities.length > 0 ? (
        <div className="rbac-identity-sync__table">
          <Table size="sm" aria-label="Bound machine clients">
            <TableHead>
              <TableRow>
                <TableHeader>Client</TableHeader>
                <TableHeader>Scopes</TableHeader>
                <TableHeader>Status</TableHeader>
                <TableHeader aria-label="Actions" />
              </TableRow>
            </TableHead>
            <TableBody>
              {identities.map((identity) => (
                <TableRow key={identity.clientId}>
                  <TableCell>
                    <code>{identity.clientId}</code>
                  </TableCell>
                  <TableCell>
                    {identity.scopes.length === 0 ? (
                      <span className="rbac-identity-sync__muted">No platform scopes</span>
                    ) : (
                      identity.scopes.map((scope) => (
                        <Tag key={scope} size="sm" type="blue" title={describe(scope)}>
                          {scope}
                        </Tag>
                      ))
                    )}
                  </TableCell>
                  <TableCell>
                    <Tag size="sm" type={identity.enabled ? "green" : "cool-gray"}>
                      {identity.enabled ? "Enabled" : "Disabled"}
                    </Tag>
                  </TableCell>
                  <TableCell>
                    <Button
                      size="sm"
                      kind="ghost"
                      renderIcon={Edit}
                      disabled={!!draft}
                      onClick={() => startEdit(identity)}
                    >
                      Edit
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        <p className="rbac-identity-sync__muted">
          No machine clients are bound to this application.
        </p>
      )}

      {draft && (
        <div className="rbac-service-identities__form">
          <h3>{draft.editing ? `Edit ${draft.oauthClientId}` : "Bind a machine client"}</h3>
          <div className="rbac-service-identities__client">
            <TextInput
              id="service-identity-client"
              labelText="Keycloak client ID"
              helperText="A confidential client with only service accounts enabled, for example alerts-platform"
              value={draft.oauthClientId}
              disabled={draft.editing}
              onChange={(event) => update({ oauthClientId: event.target.value, subject: "" })}
            />
            {!draft.editing && (
              <Button
                size="md"
                kind="tertiary"
                disabled={draft.oauthClientId.trim() === "" || resolving}
                onClick={handleResolve}
              >
                {resolving ? "Looking up..." : "Look up service account"}
              </Button>
            )}
          </div>
          {draft.subject && (
            <p className="rbac-identity-sync__muted">
              Service account <code>{draft.subject}</code>
            </p>
          )}

          <fieldset className="rbac-service-identities__scopes">
            <legend>Platform scopes</legend>
            {machineScopes.length === 0 && (
              <p className="rbac-identity-sync__muted">No scopes are available.</p>
            )}
            {machineScopes.map((scope: PlatformScope) => (
              <div key={scope.name} className="rbac-service-identities__scope">
                <Checkbox
                  id={`service-identity-scope-${scope.name}`}
                  labelText={scope.name}
                  checked={draft.scopes.includes(scope.name)}
                  onChange={(_, { checked }) => toggleScope(scope.name, checked)}
                />
                <p className="rbac-identity-sync__muted">{scope.description}</p>
              </div>
            ))}
            {draft.scopes
              .filter((name) => !machineScopes.some((scope) => scope.name === name))
              .map((name) => (
                <p key={name} className="rbac-identity-sync__muted">
                  <Tag size="sm" type="red">
                    {name}
                  </Tag>{" "}
                  is no longer in the scope catalogue; untick it to save.
                  <Button size="sm" kind="ghost" onClick={() => toggleScope(name, false)}>
                    Untick
                  </Button>
                </p>
              ))}
          </fieldset>

          <Toggle
            id="service-identity-enabled"
            labelText="Status"
            labelA="Disabled"
            labelB="Enabled"
            toggled={draft.enabled}
            onToggle={(enabled) => update({ enabled })}
          />
          <TextInput
            id="service-identity-reason"
            labelText="Reason for this change"
            helperText="Recorded in the audit log"
            value={draft.reason}
            onChange={(event) => update({ reason: event.target.value })}
          />
          <div className="rbac-identity-sync__actions">
            <Button size="sm" renderIcon={Save} disabled={!canSave} onClick={handleSave}>
              {saving ? "Saving..." : "Save"}
            </Button>
            <Button size="sm" kind="ghost" onClick={() => setDraft(null)}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
