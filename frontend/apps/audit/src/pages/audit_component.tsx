import {
  Button,
  DataTable,
  InlineLoading,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableSelectAll,
  TableSelectRow,
} from "@carbon/react";
import { useEffect, useMemo, useState } from "react";

import { AuditLogPanel } from "../components/AuditLogDrawer";
import { AuditLogFilters } from "../components/AuditLogFilters";
import { AuditMetricsPanel } from "../components/AuditMetricsPanel";

import {
  DataTableShell,
  ErrorState,
  RowActionsCell,
  TableStatusTag,
  useHeaderPanel,
  useToast,
} from "@moh-sso/ui";

import { buildAuditExportUrl, useListAuditActionsQuery, useListAuditLogsQuery } from "../api";
import type { AuditFilters, AuditLog, Cursor } from "../types";
import { AuditLogBulkActions } from "../components/audit-log-bulk-actions.component";
import { AuditLogActionsMenu } from "../components/audit-log-actions-menu.component";
import "./audit.scss";

type SuccessFilter = "true" | "false";

const headers = [
  { key: "shortId", header: "ID" },
  { key: "time", header: "Time" },
  { key: "actor", header: "Actor" },
  { key: "action", header: "Action" },
  { key: "client", header: "Client" },
  { key: "ip", header: "IP Address" },
  { key: "result", header: "Result" },
  { key: "actions", header: "" },
  { key: "raw", header: "" }, // hidden
];

function toRFC3339(d: Date) {
  return d.toISOString();
}

function getAuditLogTime(log: AuditLog) {
  if (!log.createdAt) return "—";

  const date = new Date(log.createdAt);
  if (Number.isNaN(date.getTime())) return "—";

  return date.toLocaleString();
}

function getAuditSuccess(log: AuditLog): boolean | null {
  const value = log.success ?? log.metadata?.success;

  if (typeof value === "boolean") return value;

  return null;
}

function getAuditResult(log: AuditLog) {
  const success = getAuditSuccess(log);

  if (success === true) return "success";
  if (success === false) return "failure";

  return "—";
}

function getAuditClient(log: AuditLog) {
  return log.clientId || (typeof log.metadata?.client_id === "string" ? log.metadata.client_id : "—");
}

function getAuditModule(log: AuditLog) {
  const metadataModule = log.metadata?.module;
  if (typeof metadataModule === "string" && metadataModule.trim()) {
    return metadataModule.trim();
  }

  const [module] = log.action.split(/[.:_]/);
  return module || "platform";
}

function getAuditIp(log: AuditLog) {
  return log.ip || (typeof log.metadata?.ip === "string" ? log.metadata.ip : "—");
}

function downloadFile(filename: string, content: string, type: string) {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();

  URL.revokeObjectURL(url);
}

function escapeCsvValue(value: unknown) {
  const text = String(value ?? "");
  return `"${text.replaceAll('"', '""')}"`;
}

function exportAuditLogsCsv(logs: AuditLog[]) {
  const header = ["id", "created_at", "actor", "user_id", "action", "client_id", "ip", "success"];

  const rows = logs.map((log) => [
    log.id,
    getAuditLogTime(log),
    log.username ?? "System",
    log.userId ?? "",
    log.action,
    getAuditClient(log),
    getAuditIp(log),
    getAuditResult(log),
  ]);

  const csv = [header, ...rows].map((row) => row.map(escapeCsvValue).join(",")).join("\n");

  downloadFile("audit-logs.csv", csv, "text/csv;charset=utf-8");
}

function exportAuditLogsJson(logs: AuditLog[]) {
  downloadFile("audit-logs.json", JSON.stringify(logs, null, 2), "application/json;charset=utf-8");
}

function getApiErrorMessage(error: unknown, fallback: string) {
  if (!error || typeof error !== "object") {
    return fallback;
  }

  const maybeError = error as {
    data?: {
      message?: unknown;
      error?: {
        message?: unknown;
      };
    };
    error?: unknown;
  };

  if (typeof maybeError.data?.message === "string") return maybeError.data.message;
  if (typeof maybeError.data?.error?.message === "string") return maybeError.data.error.message;
  if (typeof maybeError.error === "string") return maybeError.error;
  return fallback;
}

export default function AuditLogs() {
  const now = new Date();
  const start = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);

  const toast = useToast();
  const { openPanel, closePanel } = useHeaderPanel();

  /* -----------------------------
   * Filters
   * ----------------------------- */
  const [from, setFrom] = useState(toRFC3339(start));
  const [to, setTo] = useState(toRFC3339(now));
  const [action, setAction] = useState<string>();
  const [actor, setActor] = useState<string>();
  const [module, setModule] = useState<string>();
  const [clientId, setClientId] = useState<string>();
  const [success, setSuccess] = useState<SuccessFilter>();
  const [userId, setUserId] = useState<string>();
  const [ip, setIp] = useState<string>();

  /* -----------------------------
   * Cursor pagination
   * ----------------------------- */
  const [activeCursor, setActiveCursor] = useState<Cursor | null>(null);
  const [nextCursor, setNextCursor] = useState<Cursor | null>(null);
  const [hasMore, setHasMore] = useState(true);
  const [items, setItems] = useState<AuditLog[]>([]);

  /* -----------------------------
   * Query
   * ----------------------------- */
  const queryArgs = useMemo<AuditFilters>(() => {
    const q: AuditFilters = {
      from,
      to,
      action,
      client_id: clientId,
      user_id: userId,
      ip,
      success,
      limit: 50,
    };

    if (activeCursor) {
      q.cursor_id = activeCursor.cursor_id;
      q.cursor_created_at = activeCursor.cursor_created_at;
    }

    return q;
  }, [from, to, action, clientId, userId, ip, success, activeCursor]);

  const { data, isLoading, isFetching, isError, error, refetch } = useListAuditLogsQuery(
    queryArgs,
    {
      skip: !from || !to,
    },
  );
  const { data: actionCatalog } = useListAuditActionsQuery();

  /* -----------------------------
   * Append cursor data
   * ----------------------------- */
  useEffect(() => {
    if (!data) return;

    setItems((prev) => {
      if (!activeCursor) return data.items;

      const existing = new Set(prev.map((item) => item.id));
      const next = data.items.filter((item) => !existing.has(item.id));

      return [...prev, ...next];
    });

    setHasMore(data.has_more);
    setNextCursor(data.next_cursor ?? null);
  }, [data, activeCursor]);

  /* -----------------------------
   * Reset list on filter change
   * ----------------------------- */
  useEffect(() => {
    setActiveCursor(null);
    setNextCursor(null);
    setHasMore(true);
    setItems([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [from, to, action, clientId, userId, ip, success]);

  const actionOptions = useMemo(
    () => Array.from(new Set([...(actionCatalog?.actions ?? []), ...items.map((log) => log.action)])).sort(),
    [actionCatalog?.actions, items],
  );

  const moduleOptions = useMemo(
    () => Array.from(new Set(items.map(getAuditModule))).filter(Boolean).sort(),
    [items],
  );

  const visibleItems = useMemo(() => {
    const actorQuery = actor?.trim().toLowerCase();
    const moduleQuery = module?.trim().toLowerCase();

    return items.filter((log) => {
      if (actorQuery) {
        const actorText = `${log.username ?? ""} ${log.userId ?? ""}`.toLowerCase();
        if (!actorText.includes(actorQuery)) {
          return false;
        }
      }

      if (moduleQuery && !getAuditModule(log).toLowerCase().includes(moduleQuery)) {
        return false;
      }

      return true;
    });
  }, [actor, items, module]);

  /* -----------------------------
   * Rows
   * ----------------------------- */
  const rows = useMemo(
    () =>
      visibleItems.map((log) => ({
        id: log.id,
        shortId: log.id.slice(0, 8),
        time: getAuditLogTime(log),
        actor: log.username ?? "System",
        action: log.action,
        client: getAuditClient(log),
        ip: getAuditIp(log),
        result: getAuditResult(log),
        actions: "",
        raw: log,
      })),
    [visibleItems],
  );

  function clearFilters() {
    setAction(undefined);
    setActor(undefined);
    setModule(undefined);
    setClientId(undefined);
    setUserId(undefined);
    setIp(undefined);
    setSuccess(undefined);
  }

  function handleView(log: AuditLog) {
    openPanel({
      title: "Audit Log",
      content: <AuditLogPanel log={log} onClose={closePanel} />,
      size: "md",
    });
  }

  function handleExportCsv(logs: AuditLog[]) {
    exportAuditLogsCsv(logs);

    toast.success({
      title: "Audit logs exported",
      subtitle: `${logs.length} audit log(s) exported as CSV.`,
    });
  }

  function handleExportJson(logs: AuditLog[]) {
    exportAuditLogsJson(logs);

    toast.success({
      title: "Audit logs exported",
      subtitle: `${logs.length} audit log(s) exported as JSON.`,
    });
  }

  function handleExportFiltered(format: "csv" | "json") {
    const link = document.createElement("a");
    link.href = buildAuditExportUrl(queryArgs, format);
    link.download = format === "csv" ? "audit-logs.csv" : "audit-logs.json";
    link.click();

    toast.success({
      title: "Audit export started",
      subtitle: `Filtered audit logs are being exported as ${format.toUpperCase()}.`,
    });
  }

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (isLoading) return <InlineLoading description="Loading audit logs…" />;

  return (
    <DataTableShell
      title="Audit Logs"
      description="Security and administrative activity across the platform. RBAC governance events remain in the RBAC audit tab."
      rows={rows}
      headers={headers}
      getRowId={(row) => row.id}
      isLoading={isLoading}
      loadingDescription="Loading audit logs…"
      emptyTitle="No audit logs found"
      emptyDescription="No audit events match the selected filters."
      emptyActions={
        <Button kind="tertiary" onClick={clearFilters}>
          Clear filters
        </Button>
      }
      tableState={
        isError ? (
          <ErrorState
            title="Failed to load audit logs"
            description={getApiErrorMessage(error, "Failed to load audit logs")}
            primaryAction={{
              label: "Retry",
              onClick: refetch,
            }}
          />
        ) : undefined
      }
      topContent={<AuditMetricsPanel from={from} to={to} />}
      filters={
        <AuditLogFilters
          action={action}
          actor={actor}
          module={module}
          clientId={clientId}
          userId={userId}
          ip={ip}
          success={success}
          actionOptions={actionOptions}
          moduleOptions={moduleOptions}
          onFromChange={setFrom}
          onToChange={setTo}
          onActionChange={setAction}
          onActorChange={setActor}
          onModuleChange={setModule}
          onClientChange={setClientId}
          onUserChange={setUserId}
          onIpChange={setIp}
          onSuccessChange={setSuccess}
          onClear={clearFilters}
          onExportLoadedCsv={() => handleExportCsv(items)}
          onExportLoadedJson={() => handleExportJson(items)}
          onExportFilteredCsv={() => handleExportFiltered("csv")}
          onExportFilteredJson={() => handleExportFiltered("json")}
        />
      }
    >
      {({ rows, headers }) => (
        <>
          <DataTable rows={rows} headers={headers}>
            {({
              rows,
              headers,
              getHeaderProps,
              getRowProps,
              getSelectionProps,
              selectedRows,
            }) => {
              const selectedLogs = selectedRows.map(
                (row) => row.cells.find((cell) => cell.info.header === "raw")?.value as AuditLog,
              );

              return (
                <>
                  <AuditLogBulkActions
                    logs={selectedLogs}
                    onExportCsv={() => handleExportCsv(selectedLogs)}
                    onExportJson={() => handleExportJson(selectedLogs)}
                  />

                  <Table size="lg">
                    <TableHead>
                      <TableRow>
                        <TableSelectAll {...getSelectionProps()} />

                        {headers
                          .filter((header) => header.key !== "raw")
                          .map((header) => {
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
                      {rows.map((row) => {
                        const raw = row.cells.find((cell) => cell.info.header === "raw")
                          ?.value as AuditLog;

                        const { key, ...rowProps } = getRowProps({ row });

                        return (
                          <TableRow key={key} {...rowProps}>
                            <TableSelectRow {...getSelectionProps({ row })} />

                            {row.cells.map((cell) => {
                              if (cell.info.header === "raw") return null;

                              if (cell.info.header === "result") {
                                return (
                                  <TableCell key={cell.id}>
                                    <TableStatusTag
                                      status={String(cell.value)}
                                      kind={
                                        cell.value === "success"
                                          ? "green"
                                          : cell.value === "failure"
                                            ? "red"
                                            : "gray"
                                      }
                                    />
                                  </TableCell>
                                );
                              }

                              if (cell.info.header === "actions") {
                                return (
                                  <RowActionsCell key={cell.id}>
                                    {row.isSelected && (
                                      <AuditLogActionsMenu
                                        log={raw}
                                        onView={() => handleView(raw)}
                                      />
                                    )}
                                  </RowActionsCell>
                                );
                              }

                              return <TableCell key={cell.id}>{cell.value || "—"}</TableCell>;
                            })}
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </>
              );
            }}
          </DataTable>

          {hasMore && (
            <div className="audit-page__load-more">
              <Button
                kind="secondary"
                disabled={isFetching || !nextCursor?.cursor_id || !nextCursor?.cursor_created_at}
                onClick={() => setActiveCursor(nextCursor)}
              >
                {isFetching ? "Loading…" : "Load more"}
              </Button>
            </div>
          )}
        </>
      )}
    </DataTableShell>
  );
}
