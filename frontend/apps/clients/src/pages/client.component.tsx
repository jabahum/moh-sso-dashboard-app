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
import { useAuthorization } from "@moh-sso/auth";
import { useEffect, useMemo, useState } from "react";

import { ClientFilters } from "../components/ClientFilters";
import { useEnableClientModal } from "../components/useEnableClientModal";
import {
  DataTablePagination,
  DataTableShell,
  ErrorState,
  RowActionsCell,
  TableStatusTag,
  useHeaderPanel,
  useToast,
} from "@moh-sso/ui";
import { ClientFormPanel } from "../components/client-form-panel";
import { useListClientsQuery, useToggleClientMutation } from "../api";
import type { Client } from "@moh-sso/types";

import { ClientActionsMenu } from "./client-actions-menu.component";
import { ClientBulkActions } from "./client-bulk-actions.component";

/* -----------------------------
 * Filters
 * ----------------------------- */
type StatusFilter = "all" | "enabled" | "disabled";
type TypeFilter = "all" | "public" | "confidential";

const STATUS_OPTIONS = [
  { id: "all", label: "All" },
  { id: "enabled", label: "Enabled" },
  { id: "disabled", label: "Disabled" },
] as const;

const TYPE_OPTIONS = [
  { id: "all", label: "All" },
  { id: "public", label: "Public" },
  { id: "confidential", label: "Confidential" },
] as const;

/* -----------------------------
 * Table headers
 * ----------------------------- */
const headers = [
  { key: "name", header: "Name" },
  { key: "clientId", header: "Client ID" },
  { key: "type", header: "Type" },
  { key: "status", header: "Status" },
  { key: "actions", header: "" },
  { key: "raw", header: "" }, // hidden
];

export default function ClientsPage() {
  const { openPanel, closePanel } = useHeaderPanel();
  const { openEnableClientModal } = useEnableClientModal();
  const toast = useToast();
  const { can } = useAuthorization();

  const [bulkAction, setBulkAction] = useState<"enable" | "disable" | null>(null);

  /* -----------------------------
   * Filters
   * ----------------------------- */
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [typeFilter, setTypeFilter] = useState<TypeFilter>("all");

  /* -----------------------------
   * Pagination
   * ----------------------------- */
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  /* -----------------------------
   * Data
   * ----------------------------- */
  const { data: clients = [], isLoading, isError, error, refetch } = useListClientsQuery();

  const [toggleClient] = useToggleClientMutation();

  /* -----------------------------
   * Reset page on filter change
   * ----------------------------- */
  useEffect(() => {
    setPage(1);
  }, [statusFilter, typeFilter, search]);

  /* -----------------------------
   * Filtering
   * ----------------------------- */
  const filteredClients = useMemo(() => {
    const query = search.trim().toLowerCase();
    return clients.filter((c) => {
      if (
        query &&
        ![c.name, c.clientId].some((value) =>
          value?.toLowerCase().includes(query),
        )
      ) {
        return false;
      }
      if (statusFilter === "enabled" && !c.enabled) return false;
      if (statusFilter === "disabled" && c.enabled) return false;
      if (typeFilter === "public" && !c.publicClient) return false;
      if (typeFilter === "confidential" && c.publicClient) return false;
      return true;
    });
  }, [clients, statusFilter, typeFilter, search]);

  /* -----------------------------
   * Pagination slice
   * ----------------------------- */
  const paginatedClients = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredClients.slice(start, start + pageSize);
  }, [filteredClients, page, pageSize]);

  /* -----------------------------
   * Rows
   * ----------------------------- */
  const rows = paginatedClients.map((c) => ({
    id: c.id,
    name: c.name,
    clientId: c.clientId,
    type: c.publicClient ? "Public" : "Confidential",
    status: c.enabled ? "Enabled" : "Disabled",
    actions: "",
    raw: c,
  }));

  const clearFilters = () => {
    setSearch("");
    setStatusFilter("all");
    setTypeFilter("all");
  };

  const hasActiveFilters = search.trim() !== "" || statusFilter !== "all" || typeFilter !== "all";
  const canOnboard = can("clients:write");

  const openOnboardPanel = () =>
    openPanel({
      title: "Onboard application",
      content: <ClientFormPanel mode="create" onSuccess={closePanel} />,
      size: "md",
    });

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (isLoading) return <InlineLoading description="Loading clients…" />;

  return (
    <DataTableShell
      title="Clients"
      description="Registered applications and services integrated with the platform."
      topContent={
        canOnboard ? <Button onClick={openOnboardPanel}>Onboard application</Button> : undefined
      }
      rows={rows}
      headers={headers}
      getRowId={(row) => row.id}
      isLoading={isLoading}
      loadingDescription="Loading clients…"
      tableState={
        isError ? (
          <ErrorState
            title="Failed to load clients"
            description={getApiErrorMessage(error, "Failed to load clients")}
            primaryAction={{ label: "Retry", onClick: refetch }}
          />
        ) : undefined
      }
      searchValue={search}
      onSearch={setSearch}
      searchPlaceholder="Search clients by name or client ID"
      resultsDescription={
        !isLoading && !isError ? `${filteredClients.length} of ${clients.length} clients` : undefined
      }
      emptyActions={
        clients.length === 0 ? (
          canOnboard ? (
            <Button onClick={openOnboardPanel}>Onboard application</Button>
          ) : undefined
        ) : hasActiveFilters ? (
          <Button kind="ghost" onClick={clearFilters}>
            Clear filters
          </Button>
        ) : undefined
      }
      emptyTitle={clients.length === 0 ? "No clients yet" : "No matching clients"}
      emptyDescription={
        clients.length === 0
          ? "Applications onboarded to the platform will appear here."
          : "No clients match the current search or filters."
      }
      filters={
        <ClientFilters
          onReset={clearFilters}
          hasActiveFilters={hasActiveFilters}
          status={statusFilter}
          type={typeFilter}
          statusOptions={STATUS_OPTIONS}
          typeOptions={TYPE_OPTIONS}
          onStatusChange={setStatusFilter}
          onTypeChange={setTypeFilter}
        />
      }
    >
      {({ rows, headers }) => (
        <>
        <DataTable rows={rows} headers={headers}>
          {({ rows, headers, getHeaderProps, getRowProps, getSelectionProps, selectedRows }) => {
            const selectedClients = selectedRows.map(
              (r) => r.cells.find((c) => c.info.header === "raw")?.value as Client,
            );

            return (
              <>
                <ClientBulkActions
                  clients={selectedClients}
                  loadingAction={bulkAction}
                  onEnable={async () => {
                    try {
                      setBulkAction("enable");

                      await Promise.all(
                        selectedClients.map((c) =>
                          toggleClient({
                            id: c.id,
                            enabled: true,
                          }).unwrap(),
                        ),
                      );

                      toast.success({
                        title: "Clients enabled",
                        subtitle: `${selectedClients.length} client(s) enabled.`,
                      });
                    } catch {
                      toast.error({
                        title: "Enable failed",
                        subtitle: "Some clients could not be enabled.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onDisable={async () => {
                    try {
                      setBulkAction("disable");

                      await Promise.all(
                        selectedClients.map((c) =>
                          toggleClient({
                            id: c.id,
                            enabled: false,
                          }).unwrap(),
                        ),
                      );

                      toast.warning({
                        title: "Clients disabled",
                        subtitle: `${selectedClients.length} client(s) disabled.`,
                      });
                    } catch {
                      toast.error({
                        title: "Disable failed",
                        subtitle: "Some clients could not be disabled.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                />

                <Table>
                  <TableHead>
                    <TableRow>
                      <TableSelectAll {...getSelectionProps()} />
                      {headers
                        .filter((h) => h.key !== "raw")
                        .map((h) => (
                          <TableHeader {...getHeaderProps({ header: h })}>{h.header}</TableHeader>
                        ))}
                    </TableRow>
                  </TableHead>

                  <TableBody>
                    {rows.map((row) => {
                      const client = row.cells.find((c) => c.info.header === "raw")
                        ?.value as Client;

                      return (
                        <TableRow {...getRowProps({ row })}>
                          <TableSelectRow {...getSelectionProps({ row })} />

                          {row.cells.map((cell) => {
                            if (cell.info.header === "raw") return null;

                            if (cell.info.header === "status") {
                              return (
                                <TableCell key={cell.id}>
                                  <TableStatusTag
                                    status={String(cell.value)}
                                    kind={client.enabled ? "green" : "red"}
                                  />
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "actions") {
                              return (
                                <RowActionsCell key={cell.id}>
                                  <ClientActionsMenu
                                    client={client}
                                    onEdit={() => {
                                      openPanel({
                                        title: "Edit client",
                                        content: (
                                          <ClientFormPanel
                                            key={`edit-client-${client.id}`}
                                            mode="edit"
                                            initialClient={client}
                                            onSuccess={closePanel}
                                          />
                                        ),
                                        size: "md",
                                      });
                                    }}
                                    onToggleStatus={() => {
                                      openEnableClientModal({
                                        clientName: client.name,
                                        enabled: client.enabled,
                                        onConfirm: async () => {
                                          try {
                                            await toggleClient({
                                              id: client.id,
                                              enabled: !client.enabled,
                                            }).unwrap();

                                            toast.success({
                                              title: "Client updated",
                                              subtitle: `${client.name} updated successfully.`,
                                            });
                                          } catch {
                                            toast.error({
                                              title: "Update failed",
                                              subtitle: `Failed to update ${client.name}.`,
                                            });
                                          }
                                        },
                                      });
                                    }}
                                  />
                                </RowActionsCell>
                              );
                            }

                            return <TableCell key={cell.id}>{cell.value}</TableCell>;
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

        {/* ================= PAGINATION ================= */}
        <DataTablePagination
          page={page}
          pageSize={pageSize}
          totalItems={filteredClients.length}
          onChange={({ page, pageSize }) => {
            setPage(page);
            setPageSize(pageSize);
          }}
        />
        </>
      )}
    </DataTableShell>
  );
}

function getApiErrorMessage(error: unknown, fallback: string): string {
  if (typeof error !== "object" || error === null) {
    return fallback;
  }

  const data = "data" in error ? error.data : undefined;
  if (typeof data === "object" && data !== null && "message" in data) {
    const message = data.message;
    if (typeof message === "string" && message.trim() !== "") {
      return message;
    }
  }

  if ("message" in error && typeof error.message === "string" && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}
