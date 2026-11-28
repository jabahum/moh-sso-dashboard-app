import {
  DataTable,
  Table,
  TableHead,
  TableRow,
  TableHeader,
  TableBody,
  TableCell,
  TableSelectRow,
  TableSelectAll,
  InlineLoading,
  Tile,
  Tag,
  Pagination,
  Button,
} from "@carbon/react";
import { Add } from "@carbon/react/icons";
import { useEffect, useMemo, useState } from "react";

import {
  useListEmailsQuery,
  useListEmailsByStatusQuery,
  useRetryEmailMutation,
  useDeleteEmailMutation,
} from "../api";

import type { EmailStatus, EmailOutboxItem } from "../types";

import { ErrorState, PageShell, useHeaderPanel, useModal, useToast } from "@moh-sso/ui";

import EmailPanelComponent from "../components/email-outbox-panel/email-outbox-panel.component";
import { EmailDetailsContent } from "../components/email-outbox-detail/email-outbox-details-modal.component";
import { EmailOutboxActionsMenu } from "../components/email-outbox-actions-menu.component";

import "./email-outbox.scss";
import { EmailOutboxFilters } from "../components/email-outbox-filters.component";
import { EmailOutboxBulkActions } from "../components/email-outbox-bulk-actions.component";

/* -----------------------------
 * Filters
 * ----------------------------- */
type StatusFilter = "all" | Lowercase<EmailStatus>;

const STATUS_OPTIONS = [
  { id: "all", label: "All emails" },
  { id: "pending", label: "Pending" },
  { id: "processing", label: "Processing" },
  { id: "sent", label: "Sent" },
  { id: "failed", label: "Failed" },
  { id: "retry", label: "Retry" },
] as const;

/* -----------------------------
 * Table headers
 * ----------------------------- */
const headers = [
  { key: "subject", header: "Subject" },
  { key: "to", header: "Recipient" },
  { key: "status", header: "Status" },
  { key: "attempts", header: "Attempts" },
  { key: "scheduledAt", header: "Scheduled" },
  { key: "sentAt", header: "Sent" },
  { key: "createdAt", header: "Created" },
  { key: "actions", header: "" },
  { key: "raw", header: "" }, // hidden
];

function formatDate(value?: string | null) {
  if (!value) return "—";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";

  return date.toLocaleString();
}

function getStatusTagType(status?: string) {
  switch (status) {
    case "SENT":
      return "green";
    case "FAILED":
      return "red";
    case "PROCESSING":
      return "blue";
    case "RETRY":
      return "magenta";
    case "PENDING":
      return "gray";
    default:
      return "cool-gray";
  }
}

function getRecipients(item?: EmailOutboxItem) {
  return item?.message?.to?.map((recipient) => recipient.email).join(", ") || "—";
}

function getSubject(item?: EmailOutboxItem) {
  return item?.message?.subject || "—";
}

function toApiStatus(status: StatusFilter): "ALL" | EmailStatus {
  if (status === "all") return "ALL";
  return status.toUpperCase() as EmailStatus;
}

export default function EmailOutbox() {
  const { openPanel, closePanel } = useHeaderPanel();
  const { openModal, closeModal } = useModal();
  const toast = useToast();

  const [bulkAction, setBulkAction] = useState<"retry" | "delete" | null>(null);

  /* -----------------------------
   * Filters
   * ----------------------------- */
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");

  /* -----------------------------
   * Pagination
   * ----------------------------- */
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const offset = (page - 1) * pageSize;
  const apiStatus = toApiStatus(statusFilter);

  /* -----------------------------
   * Data
   * ----------------------------- */
  const allEmailsQuery = useListEmailsQuery(
    {
      limit: pageSize,
      offset,
    },
    {
      skip: apiStatus !== "ALL",
    },
  );

  const statusEmailsQuery = useListEmailsByStatusQuery(
    {
      status: apiStatus,
      limit: pageSize,
      offset,
    },
    {
      skip: apiStatus === "ALL",
    },
  );

  const [retryEmail] = useRetryEmailMutation();
  const [deleteEmail] = useDeleteEmailMutation();

  const emails = useMemo(() => {
    if (apiStatus === "ALL") {
      return allEmailsQuery.data ?? [];
    }

    return statusEmailsQuery.data ?? [];
  }, [apiStatus, allEmailsQuery.data, statusEmailsQuery.data]);

  const isLoading = allEmailsQuery.isLoading || statusEmailsQuery.isLoading;
  // Gate only the very first load: a status filter starts a new query whose own
  // first load must not unmount the page.
  const [firstLoadDone, setFirstLoadDone] = useState(false);
  useEffect(() => {
    if (!isLoading) setFirstLoadDone(true);
  }, [isLoading]);
  const isFetching = allEmailsQuery.isFetching || statusEmailsQuery.isFetching;
  const isError = allEmailsQuery.isError || statusEmailsQuery.isError;
  const error = allEmailsQuery.error || statusEmailsQuery.error;

  /* -----------------------------
   * Reset page on filter change
   * ----------------------------- */
  useEffect(() => {
    setPage(1);
  }, [statusFilter, search]);

  /* -----------------------------
   * Client-side search within current page
   * ----------------------------- */
  const filteredEmails = useMemo(() => {
    const q = search.trim().toLowerCase();

    if (!q) return emails;

    return emails.filter((item) => {
      const subject = getSubject(item).toLowerCase();
      const recipients = getRecipients(item).toLowerCase();
      const status = item.status?.toLowerCase() ?? "";

      return (
        subject.includes(q) ||
        recipients.includes(q) ||
        status.includes(q) ||
        item.id.toLowerCase().includes(q)
      );
    });
  }, [emails, search]);

  /* -----------------------------
   * Rows
   * ----------------------------- */
  const rows = useMemo(() => {
    return filteredEmails.map((item) => ({
      id: item.id,
      subject: getSubject(item),
      to: getRecipients(item),
      status: item.status,
      attempts: `${item.attempts ?? 0}/${item.max_attempts ?? "—"}`,
      scheduledAt: formatDate(item.scheduled_at),
      sentAt: formatDate(item.sent_at),
      createdAt: formatDate(item.created_at),
      actions: "",
      raw: item,
    }));
  }, [filteredEmails]);

  const refetchEmails = () => {
    if (apiStatus === "ALL") {
      allEmailsQuery.refetch();
      return;
    }

    statusEmailsQuery.refetch();
  };

  const handleOpenSendEmailPanel = () => {
    openPanel({
      title: "Send email",
      content: (
        <EmailPanelComponent
          onSuccess={() => {
            closePanel();
            refetchEmails();
          }}
        />
      ),
      size: "md",
    });
  };

  const handleRetry = async (item: EmailOutboxItem) => {
    try {
      await retryEmail(item.id).unwrap();

      toast.success({
        title: "Email re-queued",
        subtitle: getSubject(item),
      });
    } catch {
      toast.error({
        title: "Retry failed",
        subtitle: "Please check the email worker logs and try again.",
      });
    }
  };

  const deleteEmailRecord = async (item: EmailOutboxItem) => {
    try {
      await deleteEmail(item.id).unwrap();
      closeModal();

      toast.success({
        title: "Email deleted",
        subtitle: getSubject(item),
      });
    } catch {
      toast.error({
        title: "Delete failed",
        subtitle: "Please try again.",
      });
    }
  };

  const handleView = (item: EmailOutboxItem) => {
    openModal({
      title: "Email details",
      onClose: closeModal,
      content: <EmailDetailsContent email={item} />,
      size: "lg",
      secondaryAction: {
        label: "Close",
        onClick: closeModal,
      },
    });
  };

  const handleDelete = (item: EmailOutboxItem) => {
    openModal({
      title: "Delete email record",
      onClose: closeModal,
      content: (
        <p>
          Delete email record <strong>{getSubject(item)}</strong>?
        </p>
      ),
      primaryAction: {
        label: "Delete",
        kind: "danger",
        onClick: () => void deleteEmailRecord(item),
      },
      secondaryAction: {
        label: "Cancel",
        onClick: closeModal,
      },
    });
  };

  const deleteSelectedEmails = async (selectedEmails: EmailOutboxItem[]) => {
    try {
      setBulkAction("delete");

      await Promise.all(selectedEmails.map((email) => deleteEmail(email.id).unwrap()));
      closeModal();

      toast.success({
        title: "Emails deleted",
        subtitle: `${selectedEmails.length} email record(s) deleted.`,
      });
    } catch {
      toast.error({
        title: "Delete failed",
        subtitle: "Some emails could not be deleted.",
      });
    } finally {
      setBulkAction(null);
    }
  };

  const handleBulkDelete = (selectedEmails: EmailOutboxItem[]) => {
    openModal({
      title: "Delete email records",
      onClose: closeModal,
      content: (
        <p>
          Delete <strong>{selectedEmails.length}</strong> selected email record(s)?
        </p>
      ),
      primaryAction: {
        label: "Delete",
        kind: "danger",
        onClick: () => void deleteSelectedEmails(selectedEmails),
      },
      secondaryAction: {
        label: "Cancel",
        onClick: closeModal,
      },
    });
  };

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (!firstLoadDone && isLoading) return <InlineLoading description="Loading emails…" />;

  return (
    <>
      <div className="email-outbox-page__content">
        <PageShell
          title="Email Outbox"
          subtitle="View, filter, retry, and delete queued email records."
          actions={
            <>
              {isFetching && !isLoading && <InlineLoading description="Refreshing emails…" />}
              <Button renderIcon={Add} onClick={handleOpenSendEmailPanel} disabled={isLoading || isError}>
                Send email
              </Button>
            </>
          }
        >
        {isLoading ? <InlineLoading description="Loading emails…" /> : isError ? (
        <ErrorState title="Failed to load emails" description={(error as any)?.data?.message ?? "Failed to load emails"}
          primaryAction={{ label: "Retry", onClick: refetchEmails }} />
      ) : <>
      {/* Filters */}
        <Tile className="email-outbox-page__filter-tile">
          <EmailOutboxFilters
            search={search}
            status={statusFilter}
            statusOptions={STATUS_OPTIONS}
            onSearchChange={setSearch}
            onStatusChange={setStatusFilter}
            onReset={() => {
              setSearch("");
              setStatusFilter("all");
            }}
          />
        </Tile>

        {/* Table */}
        <Tile className="email-outbox-page__table-tile">
          <DataTable rows={rows} headers={headers}>
            {({ rows, headers, getHeaderProps, getRowProps, getSelectionProps, selectedRows }) => {
              const selectedEmails = selectedRows.map(
                (row) =>
                  row.cells.find((cell) => cell.info.header === "raw")?.value as EmailOutboxItem,
              );

              return (
                <>
                  <EmailOutboxBulkActions
                    emails={selectedEmails}
                    loadingAction={bulkAction}
                    onRetry={async () => {
                      try {
                        setBulkAction("retry");

                        await Promise.all(
                          selectedEmails.map((email) => retryEmail(email.id).unwrap()),
                        );

                        toast.success({
                          title: "Emails re-queued",
                          subtitle: `${selectedEmails.length} email(s) re-queued.`,
                        });
                      } catch {
                        toast.error({
                          title: "Retry failed",
                          subtitle: "Some emails could not be re-queued.",
                        });
                      } finally {
                        setBulkAction(null);
                      }
                    }}
                    onDelete={async () => {
                      handleBulkDelete(selectedEmails);
                    }}
                  />

                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableSelectAll {...getSelectionProps()} />

                        {headers
                          .filter((header) => header.key !== "raw")
                          .map((header) => (
                            <TableHeader {...getHeaderProps({ header })}>
                              {header.header}
                            </TableHeader>
                          ))}
                      </TableRow>
                    </TableHead>

                    <TableBody>
                      {rows.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={headers.length}>
                            <div className="email-outbox-page__empty-table">No emails found.</div>
                          </TableCell>
                        </TableRow>
                      ) : (
                        rows.map((row) => {
                          const email = row.cells.find((cell) => cell.info.header === "raw")
                            ?.value as EmailOutboxItem;

                          return (
                            <TableRow {...getRowProps({ row })}>
                              <TableSelectRow {...getSelectionProps({ row })} />

                              {row.cells.map((cell) => {
                                if (cell.info.header === "raw") return null;

                                if (cell.info.header === "subject") {
                                  return (
                                    <TableCell key={cell.id}>
                                      <div className="email-outbox-page__table-title">
                                        <strong>{getSubject(email)}</strong>

                                        <span className="email-outbox-page__recipient-preview">
                                          {getRecipients(email)}
                                        </span>
                                      </div>
                                    </TableCell>
                                  );
                                }

                                if (cell.info.header === "status") {
                                  return (
                                    <TableCell key={cell.id}>
                                      <Tag type={getStatusTagType(String(cell.value))}>
                                        {String(cell.value)}
                                      </Tag>
                                    </TableCell>
                                  );
                                }

                                if (cell.info.header === "actions") {
                                  return (
                                    <TableCell key={cell.id}>
                                      {row.isSelected && (
                                        <EmailOutboxActionsMenu
                                          email={email}
                                          isMutating={Boolean(bulkAction)}
                                          onView={() => handleView(email)}
                                          onRetry={() => handleRetry(email)}
                                          onDelete={() => handleDelete(email)}
                                        />
                                      )}
                                    </TableCell>
                                  );
                                }

                                return <TableCell key={cell.id}>{cell.value || "—"}</TableCell>;
                              })}
                            </TableRow>
                          );
                        })
                      )}
                    </TableBody>
                  </Table>
                </>
              );
            }}
          </DataTable>

          <Pagination
            page={page}
            pageSize={pageSize}
            pageSizes={[10, 20, 50, 100]}
            totalItems={emails.length < pageSize ? offset + emails.length : offset + pageSize + 1}
            onChange={({ page, pageSize }) => {
              setPage(page);
              setPageSize(pageSize);
            }}
          />
        </Tile>
      </>}
      </PageShell>
      </div>
    </>
  );
}
