import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  Dropdown,
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
import { Play, Renew, Save } from "@carbon/react/icons";
import { useModal } from "@moh-sso/ui";

import {
  useApplyIdentitySyncRunMutation,
  useCheckIdentitySyncNetworkMutation,
  useDecideIdentitySyncRowsMutation,
  useDescribeIdentitySyncMutation,
  useListServiceIdentitiesQuery,
  useResolveServiceAccountMutation,
  useSaveServiceIdentityMutation,
  useGetIdentitySyncIntegrationQuery,
  useGetIdentitySyncRunQuery,
  useListIdentitySyncRevocationsQuery,
  useListIdentitySyncRunsQuery,
  useRestoreIdentitySyncRevocationMutation,
  useResumeIdentitySyncRevocationsMutation,
  useSaveIdentitySyncIntegrationMutation,
  useStartIdentitySyncRunMutation,
} from "../api";
import type {
  IdentitySyncAccountIdFormat,
  IdentitySyncDescriptor,
  IdentitySyncIntegration,
  IdentitySyncJob,
  IdentitySyncNetworkReport,
  IdentitySyncNetworkStage,
  IdentitySyncProposal,
  IdentitySyncRevocation,
  IdentitySyncRow,
  IdentitySyncStepStatus,
} from "../types";

type Props = {
  clientId: string;
  /** Registry, run and revocation changes need rbac:write and rbac:roles:write. */
  canWrite: boolean;
};

const POLL_MS = 3000;

const proposalLabels: Record<IdentitySyncProposal, string> = {
  reuse: "Reuse existing user",
  create: "Invite new user",
  review: "Needs review",
  linked: "Already linked",
  linked_no_access: "Linked, no access",
  revoked: "Access revoked",
};

const proposalTag: Record<
  IdentitySyncProposal,
  "green" | "blue" | "magenta" | "gray" | "warm-gray" | "red"
> = {
  reuse: "green",
  create: "blue",
  review: "magenta",
  linked: "gray",
  linked_no_access: "warm-gray",
  revoked: "red",
};

const jobLabels: Record<IdentitySyncJob["kind"], string> = {
  start_run: "Reading accounts",
  apply_run: "Applying approved rows",
  resume_run: "Resuming unfinished rows",
  resume_revocations: "Retrying unfinished removals",
};

const accountIdFormats: IdentitySyncAccountIdFormat[] = ["opaque", "uuid", "positive-integer"];

export function getIdentitySyncErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === "object" && error !== null && "data" in error) {
    const data = (error as { data?: { error?: { message?: string }; message?: string } }).data;
    if (data && typeof data === "object") {
      return data.error?.message || data.message || fallback;
    }
  }
  return fallback;
}

function errorStatus(error: unknown): number | undefined {
  if (typeof error === "object" && error !== null && "status" in error) {
    const status = (error as { status?: unknown }).status;
    return typeof status === "number" ? status : undefined;
  }
  return undefined;
}

function formatTime(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

/** Rows a reviewer may decide: reuse or create rows that were never attempted. */
export function isDecidable(row: IdentitySyncRow): boolean {
  return (row.proposal === "reuse" || row.proposal === "create") && row.attempts === 0;
}

export function IdentitySyncPanel({ clientId, canWrite }: Props) {
  if (!clientId) {
    return <p>Select an application to manage its identity sync.</p>;
  }
  return (
    <div className="rbac-identity-sync">
      <IntegrationCard clientId={clientId} canWrite={canWrite} />
      <RunsCard clientId={clientId} canWrite={canWrite} />
      <RevocationsCard clientId={clientId} canWrite={canWrite} />
    </div>
  );
}

// ---------------------------------------------------------------- integration

type IntegrationDraft = {
  origin: string;
  syncClientId: string;
  accessRole: string;
  credentialRef: string;
  accountIdFormat: IdentitySyncAccountIdFormat;
  enabled: boolean;
  reason: string;
  allowContractDowngrade: boolean;
};

function toIntegrationDraft(
  clientId: string,
  integration?: IdentitySyncIntegration | null,
): IntegrationDraft {
  return {
    origin: integration?.origin ?? "",
    syncClientId: integration?.syncClientId ?? `${clientId}-identity-sync`,
    accessRole: integration?.accessRole ?? `${clientId}_access`,
    credentialRef: integration?.credentialRef ?? `keycloak:${clientId}-identity-sync`,
    accountIdFormat: integration?.accountIdFormat ?? "opaque",
    enabled: integration?.enabled ?? false,
    reason: "",
    allowContractDowngrade: false,
  };
}

function IntegrationCard({ clientId, canWrite }: Props) {
  const {
    data: integration,
    error,
    isLoading,
    isFetching,
  } = useGetIdentitySyncIntegrationQuery(clientId);
  // The API answers data: null when no integration is registered; a 404 is
  // still accepted from servers that predate that.
  const notRegistered =
    (!isLoading && !error && integration === null) || errorStatus(error) === 404;
  const [draft, setDraft] = useState<IntegrationDraft>(() => toIntegrationDraft(clientId));
  const [message, setMessage] = useState<{ kind: "error" | "success"; text: string } | null>(null);
  const [downgradeRefused, setDowngradeRefused] = useState(false);
  const [descriptor, setDescriptor] = useState<IdentitySyncDescriptor | null>(null);
  const [save, { isLoading: saving }] = useSaveIdentitySyncIntegrationMutation();
  const [describe, { isLoading: describing }] = useDescribeIdentitySyncMutation();
  const [checkNetwork, { isLoading: checkingNetwork }] = useCheckIdentitySyncNetworkMutation();
  const [networkReport, setNetworkReport] = useState<IdentitySyncNetworkReport | null>(null);
  const { data: identities } = useListServiceIdentitiesQuery(clientId);
  const [resolveServiceAccount, { isLoading: resolving }] = useResolveServiceAccountMutation();
  const [saveServiceIdentity, { isLoading: binding }] = useSaveServiceIdentityMutation();

  useEffect(() => {
    setDraft(toIntegrationDraft(clientId, integration));
    setDescriptor(null);
    setNetworkReport(null);
    setDowngradeRefused(false);
    setMessage(null);
  }, [clientId, integration]);

  const update = (patch: Partial<IntegrationDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  const handleSave = async () => {
    setMessage(null);
    try {
      const { allowContractDowngrade, ...rest } = draft;
      await save({
        clientId,
        data: {
          ...rest,
          reason: draft.reason.trim(),
          ...(allowContractDowngrade ? { allowContractDowngrade } : {}),
        },
      }).unwrap();
      setDowngradeRefused(false);
      setMessage({ kind: "success", text: "Integration saved." });
    } catch (err) {
      setDowngradeRefused(
        errorStatus(err) === 409 && getIdentitySyncErrorMessage(err, "").includes("1.0"),
      );
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to save the integration."),
      });
    }
  };

  const handleDescribe = async () => {
    setMessage(null);
    try {
      setDescriptor(await describe(clientId).unwrap());
    } catch (err) {
      setDescriptor(null);
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to reach the application."),
      });
    }
  };

  // Checks the origin being edited when it differs from the saved one, so an
  // operator can test a new origin before saving it.
  const handleCheckNetwork = async () => {
    setMessage(null);
    const origin = draft.origin.trim();
    try {
      setNetworkReport(
        await checkNetwork({
          clientId,
          origin: origin !== "" && origin !== integration?.origin ? origin : undefined,
        }).unwrap(),
      );
    } catch (err) {
      setNetworkReport(null);
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to check the network path."),
      });
    }
  };

  const syncClient = draft.syncClientId.trim();
  const serviceIdentity = identities?.find((identity) => identity.clientId === syncClient);

  // Registers (or re-enables) the sync client as a service identity of this
  // application. The subject is looked up in Keycloak, never typed by hand.
  const handleBindServiceIdentity = async () => {
    setMessage(null);
    try {
      const account = await resolveServiceAccount({ clientId, oauthClientId: syncClient }).unwrap();
      await saveServiceIdentity({
        clientId,
        oauthClientId: syncClient,
        data: {
          subject: account.subject,
          scopes: serviceIdentity?.scopes ?? [],
          enabled: true,
          reason: draft.reason.trim(),
        },
      }).unwrap();
      setMessage({
        kind: "success",
        text: `Service identity ${syncClient} registered (Keycloak service account ${account.subject}).`,
      });
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to register the service identity."),
      });
    }
  };

  const loadError = error && !notRegistered;
  const canSave =
    canWrite &&
    draft.origin.trim() !== "" &&
    draft.syncClientId.trim() !== "" &&
    draft.accessRole.trim() !== "" &&
    draft.credentialRef.trim() !== "" &&
    draft.reason.trim() !== "";

  return (
    <section className="rbac-card" aria-label="Identity sync integration">
      <div className="rbac-page__panel-header">
        <h2>Integration</h2>
        {integration ? (
          <>
            <Tag type={integration.enabled ? "green" : "gray"}>
              {integration.enabled ? "Enabled" : "Disabled"}
            </Tag>
            <Tag type="blue">Contract {integration.contractVersion || "unknown"}</Tag>
          </>
        ) : (
          notRegistered && <Tag type="gray">Not registered</Tag>
        )}
      </div>
      <p>
        Registers how the portal reaches this application&apos;s identity-sync endpoint. The
        credential is a reference (<code>keycloak:&lt;sync client&gt;</code>, <code>env:NAME</code>{" "}
        or <code>file:/path</code>); the secret itself is never entered here.
      </p>
      {isLoading && <InlineLoading description="Loading integration..." />}
      {loadError && (
        <InlineNotification
          kind="error"
          lowContrast
          title="Unable to load the integration"
          subtitle={getIdentitySyncErrorMessage(
            error,
            "The identity-sync registry is unavailable.",
          )}
        />
      )}
      {message && (
        <InlineNotification
          kind={message.kind}
          lowContrast
          title={message.kind === "error" ? "Integration not saved" : "Saved"}
          subtitle={message.text}
          onClose={() => setMessage(null)}
        />
      )}
      {!isLoading && !loadError && (
        <div className="rbac-identity-sync__form">
          <TextInput
            id="identity-sync-origin"
            labelText="Application origin"
            helperText="HTTPS scheme and host only, for example https://clims.health.go.ug"
            value={draft.origin}
            disabled={!canWrite}
            onChange={(event) => update({ origin: event.target.value })}
          />
          <TextInput
            id="identity-sync-client"
            labelText="Sync client ID"
            helperText="An enabled service identity of this application"
            value={draft.syncClientId}
            disabled={!canWrite}
            onChange={(event) => update({ syncClientId: event.target.value })}
          />
          <TextInput
            id="identity-sync-access-role"
            labelText="Access role"
            helperText="Granted after linking; removed when access is revoked"
            value={draft.accessRole}
            disabled={!canWrite}
            onChange={(event) => update({ accessRole: event.target.value })}
          />
          <TextInput
            id="identity-sync-credential"
            labelText="Credential reference"
            helperText="keycloak:<sync client ID> reads the secret from Keycloak when needed; env:NAME or file:/path read it from the backend host"
            value={draft.credentialRef}
            disabled={!canWrite}
            onChange={(event) => update({ credentialRef: event.target.value })}
          />
          <Dropdown
            id="identity-sync-format"
            titleText="Account ID format (contract 1.0 only)"
            label="Choose a format"
            items={accountIdFormats}
            selectedItem={draft.accountIdFormat}
            disabled={!canWrite}
            onChange={({ selectedItem }) =>
              selectedItem && update({ accountIdFormat: selectedItem })
            }
          />
          <Toggle
            id="identity-sync-enabled"
            labelText="Enabled"
            labelA="Disabled"
            labelB="Enabled"
            toggled={draft.enabled}
            disabled={!canWrite}
            onToggle={(enabled) => update({ enabled })}
          />
          <TextInput
            id="identity-sync-reason"
            labelText="Reason for this change"
            helperText="Recorded in the audit log"
            value={draft.reason}
            disabled={!canWrite}
            onChange={(event) => update({ reason: event.target.value })}
          />
          <div className="rbac-identity-sync__binding" aria-label="Sync client service identity">
            <span>
              Service identity <code>{syncClient || "—"}</code>:{" "}
            </span>
            {serviceIdentity ? (
              <Tag size="sm" type={serviceIdentity.enabled ? "green" : "red"}>
                {serviceIdentity.enabled ? "Registered" : "Disabled"}
              </Tag>
            ) : (
              <Tag size="sm" type="gray">
                Not registered
              </Tag>
            )}
            {canWrite && syncClient !== "" && !serviceIdentity?.enabled && (
              <Button
                size="sm"
                kind="tertiary"
                disabled={draft.reason.trim() === "" || resolving || binding}
                onClick={handleBindServiceIdentity}
              >
                {serviceIdentity ? "Enable service identity" : "Register service identity"}
              </Button>
            )}
            {canWrite && syncClient !== "" && !serviceIdentity?.enabled && (
              <small className="rbac-identity-sync__muted">
                The sync client must be a service identity of this application before the
                integration can be enabled. Enter a reason first.
              </small>
            )}
          </div>
          {downgradeRefused && (
            <Checkbox
              id="identity-sync-downgrade"
              labelText="Accept that this application now answers like contract 1.0"
              checked={draft.allowContractDowngrade}
              onChange={(_, { checked }) => update({ allowContractDowngrade: checked })}
            />
          )}
          <div className="rbac-identity-sync__actions">
            {canWrite && (
              <Button
                size="sm"
                renderIcon={Save}
                disabled={!canSave || saving}
                onClick={handleSave}
              >
                {saving ? "Saving..." : notRegistered ? "Register" : "Save integration"}
              </Button>
            )}
            {integration && (
              <Button
                size="sm"
                kind="tertiary"
                disabled={describing || isFetching}
                onClick={handleDescribe}
              >
                {describing ? "Checking..." : "Check connection"}
              </Button>
            )}
            {canWrite && (integration || draft.origin.trim() !== "") && (
              <Button
                size="sm"
                kind="ghost"
                disabled={checkingNetwork || isFetching}
                onClick={handleCheckNetwork}
              >
                {checkingNetwork ? "Checking network..." : "Check network"}
              </Button>
            )}
          </div>
          {networkReport && <NetworkReport report={networkReport} />}
          {descriptor && (
            <p className="rbac-identity-sync__descriptor">
              Live descriptor: contract {descriptor.version}, application{" "}
              <strong>{descriptor.application}</strong>, account IDs {descriptor.accountIdFormat},
              capabilities {descriptor.capabilities.join(", ")}.
            </p>
          )}
        </div>
      )}
    </section>
  );
}

// ---------------------------------------------------------------- network

const networkStageLabels: Record<IdentitySyncNetworkStage["name"], string> = {
  origin: "Origin",
  dns: "DNS lookup",
  address_policy: "Address policy",
  connect: "Connection",
  tls: "TLS",
  http: "Identity-sync endpoint",
};

const networkTagTypes: Record<
  IdentitySyncNetworkStage["status"],
  "green" | "warm-gray" | "red" | "cool-gray"
> = {
  ok: "green",
  warn: "warm-gray",
  fail: "red",
  skipped: "cool-gray",
};

const networkStatusLabels: Record<IdentitySyncNetworkStage["status"], string> = {
  ok: "OK",
  warn: "Check",
  fail: "Failed",
  skipped: "Skipped",
};

function NetworkReport({ report }: { report: IdentitySyncNetworkReport }) {
  return (
    <div className="rbac-identity-sync__network" aria-live="polite">
      <p>
        {report.ok ? "The portal server can reach " : "The portal server cannot reach "}
        <strong>{report.origin}</strong>
        {report.addresses?.length ? ` (${report.addresses.join(", ")})` : ""}.
      </p>
      <ul>
        {report.stages.map((stage) => (
          <li key={stage.name}>
            <Tag size="sm" type={networkTagTypes[stage.status]}>
              {networkStatusLabels[stage.status]}
            </Tag>
            <span className="rbac-identity-sync__network-stage">
              {networkStageLabels[stage.name]}
            </span>
            {stage.detail && <span className="rbac-identity-sync__muted">{stage.detail}</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}

// ---------------------------------------------------------------- runs

function JobStatus({ jobs }: { jobs: IdentitySyncJob[] }) {
  const latest = jobs[0];
  if (!latest) return null;
  if (latest.status === "running") {
    return (
      <InlineLoading description={`${jobLabels[latest.kind]}... started by ${latest.actor}`} />
    );
  }
  if (latest.status === "failed") {
    return (
      <InlineNotification
        kind="error"
        lowContrast
        title={`${jobLabels[latest.kind]} failed`}
        subtitle={latest.errorMessage ?? latest.error ?? "The job failed."}
      />
    );
  }
  return null;
}

function RunsCard({ clientId, canWrite }: Props) {
  const [polling, setPolling] = useState(0);
  const { data, isLoading, error } = useListIdentitySyncRunsQuery(clientId, {
    pollingInterval: polling,
  });
  const [startRun, { isLoading: starting }] = useStartIdentitySyncRunMutation();
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const jobs = useMemo(() => data?.jobs ?? [], [data?.jobs]);
  const runs = useMemo(() => data?.runs ?? [], [data?.runs]);
  const running = jobs.some((job) => job.status === "running");

  useEffect(() => {
    setPolling(running || runs.some((run) => run.status === "applying") ? POLL_MS : 0);
  }, [running, runs]);

  useEffect(() => setSelectedRunId(null), [clientId]);

  // Open the run a finished start job produced.
  const latest = jobs[0];
  useEffect(() => {
    if (latest?.kind === "start_run" && latest.status === "succeeded" && latest.runId) {
      setSelectedRunId(latest.runId);
    }
  }, [latest?.id, latest?.kind, latest?.status, latest?.runId]);

  const handleStart = async () => {
    setMessage(null);
    try {
      await startRun(clientId).unwrap();
      setPolling(POLL_MS);
    } catch (err) {
      setMessage(getIdentitySyncErrorMessage(err, "Unable to start a run."));
    }
  };

  return (
    <section className="rbac-card" aria-label="Identity sync runs">
      <div className="rbac-page__panel-header">
        <h2>Batch runs</h2>
        {canWrite && (
          <Button size="sm" renderIcon={Play} disabled={starting || running} onClick={handleStart}>
            Start run
          </Button>
        )}
      </div>
      <p>
        A run reads every eligible account in the application and proposes a match for each. Nothing
        changes in the application or Keycloak until approved rows are applied.
      </p>
      {message && (
        <InlineNotification
          kind="error"
          lowContrast
          title="Run not started"
          subtitle={message}
          onClose={() => setMessage(null)}
        />
      )}
      {error && (
        <InlineNotification
          kind="error"
          lowContrast
          title="Unable to load runs"
          subtitle={getIdentitySyncErrorMessage(error, "Identity-sync runs are unavailable.")}
        />
      )}
      <JobStatus jobs={jobs} />
      {isLoading ? (
        <InlineLoading description="Loading runs..." />
      ) : runs.length === 0 ? (
        <p>No runs yet.</p>
      ) : (
        <div className="rbac-identity-sync__table">
          <Table size="sm" aria-label="Recent runs">
            <TableHead>
              <TableRow>
                <TableHeader>Started</TableHeader>
                <TableHeader>By</TableHeader>
                <TableHeader>Status</TableHeader>
                <TableHeader>
                  <span className="cds--visually-hidden">Open</span>
                </TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {runs.map((run) => (
                <TableRow key={run.id} isSelected={run.id === selectedRunId}>
                  <TableCell>{formatTime(run.createdAt)}</TableCell>
                  <TableCell>{run.createdBy}</TableCell>
                  <TableCell>
                    <Tag
                      type={
                        run.status === "completed"
                          ? "green"
                          : run.status === "failed"
                            ? "red"
                            : "gray"
                      }
                    >
                      {run.status}
                    </Tag>
                  </TableCell>
                  <TableCell>
                    <Button size="sm" kind="ghost" onClick={() => setSelectedRunId(run.id)}>
                      {run.id === selectedRunId ? "Open" : "Review"}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {selectedRunId && (
        <RunDetail
          clientId={clientId}
          runId={selectedRunId}
          canWrite={canWrite}
          jobRunning={running}
          onJobStarted={() => setPolling(POLL_MS)}
        />
      )}
    </section>
  );
}

type RunDetailProps = Props & {
  runId: string;
  jobRunning: boolean;
  onJobStarted: () => void;
};

const proposalFilterItems: ("all" | IdentitySyncProposal)[] = [
  "all",
  "reuse",
  "create",
  "review",
  "linked",
  "linked_no_access",
  "revoked",
];

function RunDetail({ clientId, runId, canWrite, jobRunning, onJobStarted }: RunDetailProps) {
  const { openModal, closeModal } = useModal();
  const [polling, setPolling] = useState(0);
  const { data, isLoading, error } = useGetIdentitySyncRunQuery(
    { clientId, runId },
    { pollingInterval: polling },
  );
  const [decide, { isLoading: deciding }] = useDecideIdentitySyncRowsMutation();
  const [apply, { isLoading: applying }] = useApplyIdentitySyncRunMutation();
  const [filter, setFilter] = useState<"all" | IdentitySyncProposal>("all");
  const [selected, setSelected] = useState<string[]>([]);
  const [note, setNote] = useState("");
  const [message, setMessage] = useState<{ kind: "error" | "success"; text: string } | null>(null);

  const applyingNow = data?.run.status === "applying" || jobRunning;
  useEffect(() => setPolling(applyingNow ? POLL_MS : 0), [applyingNow]);
  useEffect(() => {
    setSelected([]);
    setFilter("all");
    setMessage(null);
  }, [runId]);

  const rows = useMemo(
    () => (data?.rows ?? []).filter((row) => filter === "all" || row.proposal === filter),
    [data?.rows, filter],
  );
  const decidable = rows.filter(isDecidable);
  const approvedPending = (data?.rows ?? []).some(
    (row) => row.decision === "approved" && row.attempts === 0,
  );
  const unfinished = (data?.rows ?? []).some(
    (row) =>
      row.decision === "approved" &&
      row.attempts > 0 &&
      !(row.linkStatus === "linked" && row.grantStatus === "granted") &&
      row.linkStatus !== "conflict" &&
      row.linkStatus !== "ineligible",
  );

  const toggle = (accountId: string, checked: boolean) =>
    setSelected((current) =>
      checked ? [...current, accountId] : current.filter((id) => id !== accountId),
    );

  const handleDecide = async (decision: "approve" | "reject") => {
    setMessage(null);
    try {
      await decide({ clientId, runId, decision, accountIds: selected, note: note.trim() }).unwrap();
      setMessage({
        kind: "success",
        text: `${selected.length} row${selected.length === 1 ? "" : "s"} ${decision === "approve" ? "approved" : "rejected"}.`,
      });
      setSelected([]);
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to record the decision."),
      });
    }
  };

  const startApply = async (resume: boolean) => {
    setMessage(null);
    try {
      await apply({ clientId, runId, resume }).unwrap();
      onJobStarted();
      setPolling(POLL_MS);
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to start applying the run."),
      });
    }
  };

  const handleApply = (resume: boolean) =>
    openModal({
      title: resume ? "Resume unfinished rows" : "Apply approved rows",
      onClose: closeModal,
      content: (
        <p>
          {resume
            ? "Retry the approved rows that did not finish? Linked rows only have their access grant retried."
            : "Apply the approved rows? Each account is re-checked, linked in the application, and granted access."}
        </p>
      ),
      primaryAction: {
        label: resume ? "Resume" : "Apply",
        onClick: () => {
          closeModal();
          void startApply(resume);
        },
      },
      secondaryAction: { label: "Cancel", onClick: closeModal },
    });

  if (isLoading) return <InlineLoading description="Loading run..." />;
  if (error || !data) {
    return (
      <InlineNotification
        kind="error"
        lowContrast
        title="Unable to load the run"
        subtitle={getIdentitySyncErrorMessage(error, "The run could not be loaded.")}
      />
    );
  }

  const counts = data.counts;
  return (
    <div className="rbac-identity-sync__run" aria-label="Run review">
      <h3>
        Run started {formatTime(data.run.createdAt)} by {data.run.createdBy}
      </h3>
      <div className="rbac-identity-sync__counts">
        {(Object.keys(proposalLabels) as IdentitySyncProposal[]).map((proposal) => (
          <Tag key={proposal} type={proposalTag[proposal]}>
            {proposalLabels[proposal]}: {counts[`proposal:${proposal}`] ?? 0}
          </Tag>
        ))}
        <Tag type="cyan">Approved: {counts["decision:approved"] ?? 0}</Tag>
        {(counts.error ?? 0) > 0 && <Tag type="red">With errors: {counts.error}</Tag>}
      </div>
      {message && (
        <InlineNotification
          kind={message.kind}
          lowContrast
          title={message.kind === "error" ? "Action failed" : "Done"}
          subtitle={message.text}
          onClose={() => setMessage(null)}
        />
      )}
      {applyingNow && <InlineLoading description="Applying. Rows update as they finish..." />}
      <div className="rbac-identity-sync__toolbar">
        <Dropdown
          id="identity-sync-proposal-filter"
          titleText="Show"
          label="Filter rows"
          items={proposalFilterItems}
          itemToString={(item) => (item === "all" ? "All rows" : item ? proposalLabels[item] : "")}
          selectedItem={filter}
          onChange={({ selectedItem }) => selectedItem && setFilter(selectedItem)}
        />
        {canWrite && (
          <>
            <TextInput
              id="identity-sync-review-note"
              labelText="Review note (optional)"
              value={note}
              maxLength={1000}
              onChange={(event) => setNote(event.target.value)}
            />
            <div className="rbac-identity-sync__actions">
              <Button
                size="sm"
                disabled={selected.length === 0 || deciding || applyingNow}
                onClick={() => handleDecide("approve")}
              >
                Approve selected ({selected.length})
              </Button>
              <Button
                size="sm"
                kind="secondary"
                disabled={selected.length === 0 || deciding || applyingNow}
                onClick={() => handleDecide("reject")}
              >
                Reject selected
              </Button>
              <Button
                size="sm"
                kind="tertiary"
                disabled={!approvedPending || applying || applyingNow}
                onClick={() => handleApply(false)}
              >
                Apply approved rows
              </Button>
              <Button
                size="sm"
                kind="ghost"
                renderIcon={Renew}
                disabled={!unfinished || applying || applyingNow}
                onClick={() => handleApply(true)}
              >
                Resume unfinished
              </Button>
            </div>
          </>
        )}
      </div>
      <div className="rbac-identity-sync__table rbac-identity-sync__table--wide">
        <Table size="sm" aria-label="Run rows">
          <TableHead>
            <TableRow>
              {canWrite && (
                <TableHeader>
                  <Checkbox
                    id={`identity-sync-select-all-${runId}`}
                    labelText="Select all decidable rows"
                    hideLabel
                    checked={
                      decidable.length > 0 &&
                      decidable.every((row) => selected.includes(row.accountId))
                    }
                    disabled={decidable.length === 0}
                    onChange={(_, { checked }) =>
                      setSelected(checked ? decidable.map((row) => row.accountId) : [])
                    }
                  />
                </TableHeader>
              )}
              <TableHeader>Account</TableHeader>
              <TableHeader>Email</TableHeader>
              <TableHeader>Proposal</TableHeader>
              <TableHeader>Keycloak user</TableHeader>
              <TableHeader>Decision</TableHeader>
              <TableHeader>Link / access</TableHeader>
              <TableHeader>Attempts</TableHeader>
              <TableHeader>Last error</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.id}>
                {canWrite && (
                  <TableCell>
                    <Checkbox
                      id={`identity-sync-row-${row.id}`}
                      labelText={`Select account ${row.accountId}`}
                      hideLabel
                      checked={selected.includes(row.accountId)}
                      disabled={!isDecidable(row)}
                      onChange={(_, { checked }) => toggle(row.accountId, checked)}
                    />
                  </TableCell>
                )}
                <TableCell>
                  <code>{row.accountId}</code>
                </TableCell>
                <TableCell>{row.emailHint || "—"}</TableCell>
                <TableCell>
                  <Tag size="sm" type={proposalTag[row.proposal]}>
                    {proposalLabels[row.proposal]}
                  </Tag>
                </TableCell>
                <TableCell>
                  {row.proposedSubject ? <code>{row.proposedSubject}</code> : "—"}
                </TableCell>
                <TableCell>
                  {row.decision}
                  {row.reviewedBy && (
                    <small className="rbac-identity-sync__muted"> by {row.reviewedBy}</small>
                  )}
                </TableCell>
                <TableCell>
                  {row.linkStatus} / {row.grantStatus}
                </TableCell>
                <TableCell>{row.attempts}</TableCell>
                <TableCell>
                  {row.lastError ? (
                    <Tag size="sm" type="red">
                      {row.lastError}
                    </Tag>
                  ) : (
                    ""
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- revocations

const stepTag: Record<IdentitySyncStepStatus, "green" | "gray" | "red" | "warm-gray"> = {
  done: "green",
  pending: "gray",
  failed: "red",
  unsupported: "warm-gray",
};

function StepTags({ revocation }: { revocation: IdentitySyncRevocation }) {
  return (
    <span className="rbac-identity-sync__steps">
      <Tag size="sm" type={stepTag[revocation.roleRemoved]}>
        Role {revocation.roleRemoved}
      </Tag>
      <Tag size="sm" type={stepTag[revocation.consentsRevoked]}>
        Consents {revocation.consentsRevoked}
      </Tag>
      <Tag size="sm" type={stepTag[revocation.sessionsRevoked]}>
        Sessions {revocation.sessionsRevoked}
      </Tag>
    </span>
  );
}

function RevocationsCard({ clientId, canWrite }: Props) {
  const { openModal, closeModal } = useModal();
  const [includeRestored, setIncludeRestored] = useState(false);
  const [polling, setPolling] = useState(0);
  const { data, isLoading, error } = useListIdentitySyncRevocationsQuery(
    { clientId, all: includeRestored },
    { pollingInterval: polling },
  );
  const [resume, { isLoading: resuming }] = useResumeIdentitySyncRevocationsMutation();
  const [restore, { isLoading: restoring }] = useRestoreIdentitySyncRevocationMutation();
  const [restoreReason, setRestoreReason] = useState("");
  const [message, setMessage] = useState<{ kind: "error" | "success"; text: string } | null>(null);
  const jobs = useMemo(
    () => (data?.jobs ?? []).filter((job) => job.kind === "resume_revocations"),
    [data?.jobs],
  );
  const running = (data?.jobs ?? []).some((job) => job.status === "running");
  const revocations = data?.revocations ?? [];
  const unfinished = revocations.some((r) => !r.restoredAt && !r.complete);

  useEffect(() => setPolling(running ? POLL_MS : 0), [running]);

  const handleResume = async () => {
    setMessage(null);
    try {
      await resume(clientId).unwrap();
      setPolling(POLL_MS);
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to retry the removals."),
      });
    }
  };

  const restoreRevocation = async (subject: string) => {
    setMessage(null);
    try {
      await restore({ clientId, subject, reason: restoreReason.trim() }).unwrap();
      setMessage({ kind: "success", text: "Revocation cleared. Access was not granted again." });
    } catch (err) {
      setMessage({
        kind: "error",
        text: getIdentitySyncErrorMessage(err, "Unable to clear the revocation."),
      });
    }
  };

  const handleRestore = (subject: string) =>
    openModal({
      title: "Clear revocation",
      onClose: closeModal,
      content: (
        <p>Clear this revocation? Access is not granted again; later syncs may propose it.</p>
      ),
      primaryAction: {
        label: "Clear revocation",
        kind: "danger",
        onClick: () => {
          closeModal();
          void restoreRevocation(subject);
        },
      },
      secondaryAction: { label: "Cancel", onClick: closeModal },
    });

  return (
    <section className="rbac-card" aria-label="Identity sync revocations">
      <div className="rbac-page__panel-header">
        <h2>Access removals</h2>
        {canWrite && (
          <Button
            size="sm"
            kind="tertiary"
            renderIcon={Renew}
            disabled={!unfinished || resuming || running}
            onClick={handleResume}
          >
            Retry unfinished
          </Button>
        )}
      </div>
      <p>
        Removing this application&apos;s access role ends the person&apos;s sessions in it at once.
        Steps that failed stay listed here until they are retried.
      </p>
      {message && (
        <InlineNotification
          kind={message.kind}
          lowContrast
          title={message.kind === "error" ? "Action failed" : "Done"}
          subtitle={message.text}
          onClose={() => setMessage(null)}
        />
      )}
      {error && (
        <InlineNotification
          kind="error"
          lowContrast
          title="Unable to load removals"
          subtitle={getIdentitySyncErrorMessage(
            error,
            "Identity-sync revocations are unavailable.",
          )}
        />
      )}
      <JobStatus jobs={jobs} />
      <div className="rbac-identity-sync__toolbar">
        <Toggle
          id="identity-sync-include-restored"
          labelText="Include restored"
          labelA="Active only"
          labelB="All"
          toggled={includeRestored}
          onToggle={setIncludeRestored}
        />
        {canWrite && (
          <TextInput
            id="identity-sync-restore-reason"
            labelText="Reason for clearing a revocation"
            value={restoreReason}
            onChange={(event) => setRestoreReason(event.target.value)}
          />
        )}
      </div>
      {isLoading ? (
        <InlineLoading description="Loading removals..." />
      ) : revocations.length === 0 ? (
        <p>No access removals recorded.</p>
      ) : (
        <div className="rbac-identity-sync__table rbac-identity-sync__table--wide">
          <Table size="sm" aria-label="Access removals">
            <TableHead>
              <TableRow>
                <TableHeader>Keycloak user</TableHeader>
                <TableHeader>Removed</TableHeader>
                <TableHeader>By / reason</TableHeader>
                <TableHeader>Steps</TableHeader>
                <TableHeader>Status</TableHeader>
                {canWrite && (
                  <TableHeader>
                    <span className="cds--visually-hidden">Actions</span>
                  </TableHeader>
                )}
              </TableRow>
            </TableHead>
            <TableBody>
              {revocations.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>
                    <code>{r.subject}</code>
                  </TableCell>
                  <TableCell>{formatTime(r.revokedAt)}</TableCell>
                  <TableCell className="rbac-identity-sync__wrap">
                    {r.revokedBy}
                    <small className="rbac-identity-sync__muted"> {r.reason}</small>
                  </TableCell>
                  <TableCell>
                    <StepTags revocation={r} />
                    {r.lastError && (
                      <small className="rbac-identity-sync__muted"> {r.lastError}</small>
                    )}
                  </TableCell>
                  <TableCell>
                    {r.restoredAt ? (
                      <Tag size="sm" type="gray">
                        Restored by {r.restoredBy}
                      </Tag>
                    ) : r.complete ? (
                      <Tag size="sm" type={r.sessionsStillOpen ? "warm-gray" : "green"}>
                        {r.sessionsStillOpen ? "Done; sessions expire on their own" : "Done"}
                      </Tag>
                    ) : (
                      <Tag size="sm" type="red">
                        Incomplete
                      </Tag>
                    )}
                  </TableCell>
                  {canWrite && (
                    <TableCell>
                      {!r.restoredAt && (
                        <Button
                          size="sm"
                          kind="ghost"
                          disabled={restoring || restoreReason.trim() === ""}
                          onClick={() => handleRestore(r.subject)}
                        >
                          Clear revocation
                        </Button>
                      )}
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  );
}
