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
import { Add, Attachment, Link as LinkIcon } from "@carbon/react/icons";
import { useEffect, useMemo, useState } from "react";

import {
  useListAllAnnouncementsAdminQuery,
  usePublishAnnouncementMutation,
  useArchiveAnnouncementMutation,
  useDeleteAnnouncementMutation,
  useSetAnnouncementPinnedMutation,
  useDraftAnnouncementMutation,
} from "../api";

import type { Announcement, AnnouncementLevel } from "../types";

import { ErrorState, PageShell, useHeaderPanel, useModal, useToast } from "@moh-sso/ui";

import { formatDateTime, getStatusTagType, getTagType, isAnnouncementActive } from "@moh-sso/utils";

import { AnnouncementFilters } from "../components/announcement-filters.component";
import { AnnouncementBulkActions } from "../components/announcement-bulk-actions.component";
import { AnnouncementActionsMenu } from "../components/announcement-actions-menu.component";
import { ManageAnnouncementsPanel } from "../components/manage-announcement-panel";
import "./announcements.scss";

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

/* -----------------------------
 * Filters
 * ----------------------------- */
type StatusFilter = "all" | "published" | "draft" | "scheduled" | "archived";
type LevelFilter = "all" | Lowercase<AnnouncementLevel>;
type AudienceFilter = "all" | "all_users" | "admins_only" | "specific_roles";
type PinFilter = "all" | "pinned" | "unpinned";

const STATUS_OPTIONS = [
  { id: "all", label: "All" },
  { id: "published", label: "Published" },
  { id: "draft", label: "Draft" },
  { id: "scheduled", label: "Scheduled" },
  { id: "archived", label: "Archived" },
] as const;

const LEVEL_OPTIONS = [
  { id: "all", label: "All" },
  { id: "info", label: "Info" },
  { id: "success", label: "Success" },
  { id: "warning", label: "Warning" },
  { id: "critical", label: "Critical" },
] as const;

const AUDIENCE_OPTIONS = [
  { id: "all", label: "All" },
  { id: "all_users", label: "All users" },
  { id: "admins_only", label: "Admins only" },
  { id: "specific_roles", label: "Specific roles" },
] as const;

const PIN_OPTIONS = [
  { id: "all", label: "All" },
  { id: "pinned", label: "Pinned" },
  { id: "unpinned", label: "Unpinned" },
] as const;

/* -----------------------------
 * Table headers
 * ----------------------------- */
const headers = [
  { key: "title", header: "Title" },
  { key: "level", header: "Level" },
  { key: "audience", header: "Audience" },
  { key: "status", header: "Status" },
  { key: "pinned", header: "Pinned" },
  { key: "attachments", header: "Files" },
  { key: "active", header: "Active" },
  { key: "publishAt", header: "Publish at" },
  { key: "expiresAt", header: "Expires at" },
  { key: "actions", header: "" },
  { key: "raw", header: "" }, // hidden
];

export function AnnouncementsPage() {
  const toast = useToast();
  const { openPanel, closePanel } = useHeaderPanel();
  const { openModal, closeModal } = useModal();

  const [bulkAction, setBulkAction] = useState<
    "publish" | "draft" | "archive" | "pin" | "unpin" | "delete" | null
  >(null);

  /* -----------------------------
   * Filters
   * ----------------------------- */
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [levelFilter, setLevelFilter] = useState<LevelFilter>("all");
  const [audienceFilter, setAudienceFilter] = useState<AudienceFilter>("all");
  const [pinFilter, setPinFilter] = useState<PinFilter>("all");

  /* -----------------------------
   * Pagination
   * ----------------------------- */
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  /* -----------------------------
   * Data
   * ----------------------------- */
  const {
    data: announcements = [],
    isLoading,
    isError,
    error,
    refetch,
  } = useListAllAnnouncementsAdminQuery();

  const [publishAnnouncement] = usePublishAnnouncementMutation();
  const [archiveAnnouncement] = useArchiveAnnouncementMutation();
  const [deleteAnnouncement] = useDeleteAnnouncementMutation();
  const [setAnnouncementPinned] = useSetAnnouncementPinnedMutation();
  const [draftAnnouncement] = useDraftAnnouncementMutation();

  /* -----------------------------
   * Reset page on filter change
   * ----------------------------- */
  useEffect(() => {
    setPage(1);
  }, [search, statusFilter, levelFilter, audienceFilter, pinFilter]);

  /* -----------------------------
   * Filtering
   * ----------------------------- */
  const filteredAnnouncements = useMemo(() => {
    const q = search.trim().toLowerCase();

    return announcements.filter((item) => {
      const status = item.status.toLowerCase();
      const level = item.level.toLowerCase();
      const audience = item.audience_type.toLowerCase();

      if (statusFilter !== "all" && status !== statusFilter) return false;
      if (levelFilter !== "all" && level !== levelFilter) return false;
      if (audienceFilter !== "all" && audience !== audienceFilter) return false;
      if (pinFilter === "pinned" && !item.is_pinned) return false;
      if (pinFilter === "unpinned" && item.is_pinned) return false;

      if (!q) return true;

      return (
        item.title.toLowerCase().includes(q) ||
        item.message.toLowerCase().includes(q) ||
        item.status.toLowerCase().includes(q) ||
        item.level.toLowerCase().includes(q) ||
        item.audience_type.toLowerCase().includes(q) ||
        (item.summary?.toLowerCase().includes(q) ?? false) ||
        (item.tag?.toLowerCase().includes(q) ?? false)
      );
    });
  }, [announcements, search, statusFilter, levelFilter, audienceFilter, pinFilter]);

  /* -----------------------------
   * Pagination slice
   * ----------------------------- */
  const paginatedAnnouncements = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredAnnouncements.slice(start, start + pageSize);
  }, [filteredAnnouncements, page, pageSize]);

  useEffect(() => {
    const maxPage = Math.max(1, Math.ceil(filteredAnnouncements.length / pageSize));

    if (page > maxPage) {
      setPage(maxPage);
    }
  }, [filteredAnnouncements.length, page, pageSize]);

  /* -----------------------------
   * Rows
   * ----------------------------- */
  const rows = paginatedAnnouncements.map((item) => ({
    id: item.id,
    title: item.title,
    level: item.level,
    audience: item.audience_type,
    status: item.status,
    pinned: item.is_pinned ? "Yes" : "No",
    attachments: String(item.attachment_count ?? item.attachments?.length ?? 0),
    active: isAnnouncementActive(item) ? "Yes" : "No",
    publishAt: formatDateTime(item.publish_at),
    expiresAt: formatDateTime(item.expires_at),
    actions: "",
    raw: item,
  }));

  /* -----------------------------
   * Panel handlers
   * ----------------------------- */
  const handleOpenCreatePanel = () => {
    openPanel({
      title: "Create announcement",
      content: (
        <ManageAnnouncementsPanel
          mode="create"
          onSuccess={() => {
            closePanel();
            refetch();
          }}
          onCancel={closePanel}
        />
      ),
      size: "md",
    });
  };

  const handleOpenEditPanel = (announcement: Announcement) => {
    openPanel({
      title: "Edit announcement",
      content: (
        <ManageAnnouncementsPanel
          mode="edit"
          announcement={announcement}
          onSuccess={() => {
            closePanel();
            refetch();
          }}
          onCancel={closePanel}
        />
      ),
      size: "md",
    });
  };

  /* -----------------------------
   * Row action handlers
   * ----------------------------- */
  const handlePublish = async (announcement: Announcement) => {
    await publishAnnouncement(announcement.id).unwrap();

    toast.success({
      title: "Announcement published",
      subtitle: `${announcement.title} has been published.`,
    });
  };

  const handleMoveToDraft = async (announcement: Announcement) => {
    await draftAnnouncement(announcement.id).unwrap();

    toast.warning({
      title: "Moved to draft",
      subtitle: `${announcement.title} has been moved to draft.`,
    });
  };

  const handleArchive = async (announcement: Announcement) => {
    await archiveAnnouncement(announcement.id).unwrap();

    toast.warning({
      title: "Announcement archived",
      subtitle: `${announcement.title} has been archived.`,
    });
  };

  const handleTogglePin = async (announcement: Announcement) => {
    await setAnnouncementPinned({
      id: announcement.id,
      body: {
        is_pinned: !announcement.is_pinned,
      },
    }).unwrap();

    toast.success({
      title: announcement.is_pinned ? "Announcement unpinned" : "Announcement pinned",
      subtitle: `${announcement.title} updated successfully.`,
    });
  };

  const deleteOneAnnouncement = async (announcement: Announcement) => {
    try {
      await deleteAnnouncement(announcement.id).unwrap();
      closeModal();

      toast.error({
        title: "Announcement deleted",
        subtitle: `${announcement.title} was deleted.`,
      });
    } catch {
      toast.error({
        title: "Delete failed",
        subtitle: "The announcement could not be deleted.",
      });
    }
  };

  const handleDelete = (announcement: Announcement) => {
    openModal({
      title: "Delete announcement",
      onClose: closeModal,
      content: (
        <p>
          Delete <strong>{announcement.title}</strong>? This action should only be used when you're
          sure.
        </p>
      ),
      primaryAction: {
        label: "Delete",
        kind: "danger",
        onClick: () => void deleteOneAnnouncement(announcement),
      },
      secondaryAction: {
        label: "Cancel",
        onClick: closeModal,
      },
    });
  };

  const deleteSelectedAnnouncements = async (selectedAnnouncements: Announcement[]) => {
    try {
      setBulkAction("delete");

      await Promise.all(selectedAnnouncements.map((item) => deleteAnnouncement(item.id).unwrap()));
      closeModal();

      toast.error({
        title: "Announcements deleted",
        subtitle: `${selectedAnnouncements.length} announcement(s) deleted.`,
      });
    } catch {
      toast.error({
        title: "Delete failed",
        subtitle: "Some announcements could not be deleted.",
      });
    } finally {
      setBulkAction(null);
    }
  };

  const handleBulkDelete = (selectedAnnouncements: Announcement[]) => {
    openModal({
      title: "Delete announcements",
      onClose: closeModal,
      content: (
        <p>
          Delete <strong>{selectedAnnouncements.length}</strong> selected announcement(s)?
        </p>
      ),
      primaryAction: {
        label: "Delete",
        kind: "danger",
        onClick: () => void deleteSelectedAnnouncements(selectedAnnouncements),
      },
      secondaryAction: {
        label: "Cancel",
        onClick: closeModal,
      },
    });
  };

  return (
    <div className="announcements-page">
      <PageShell
        title="Announcements"
        subtitle="Manage platform notices, alerts, drafts, schedules, and archived updates."
        actions={
          <Button renderIcon={Add} onClick={handleOpenCreatePanel} disabled={isLoading || isError}>
            New announcement
          </Button>
        }
      >
      {isLoading ? <InlineLoading description="Loading announcements…" /> : isError ? (
        <ErrorState title="Failed to load announcements" description={getApiErrorMessage(error, "Failed to load announcements")}
          primaryAction={{ label: "Retry", onClick: refetch }} />
      ) : <>
      {/* Filters */}
      <Tile className="announcements-page__filter-tile">
        <AnnouncementFilters
          search={search}
          status={statusFilter}
          level={levelFilter}
          audience={audienceFilter}
          pin={pinFilter}
          statusOptions={STATUS_OPTIONS}
          levelOptions={LEVEL_OPTIONS}
          audienceOptions={AUDIENCE_OPTIONS}
          pinOptions={PIN_OPTIONS}
          onSearchChange={setSearch}
          onStatusChange={setStatusFilter}
          onLevelChange={setLevelFilter}
          onAudienceChange={setAudienceFilter}
          onPinChange={setPinFilter}
          onReset={() => {
            setSearch("");
            setStatusFilter("all");
            setLevelFilter("all");
            setAudienceFilter("all");
            setPinFilter("all");
          }}
        />
      </Tile>

      {/* Table */}
      <Tile className="announcements-page__table-tile">
        <DataTable rows={rows} headers={headers}>
          {({ rows, headers, getHeaderProps, getRowProps, getSelectionProps, selectedRows }) => {
            const selectedAnnouncements = selectedRows.map(
              (row) => row.cells.find((cell) => cell.info.header === "raw")?.value as Announcement,
            );

            return (
              <>
                <AnnouncementBulkActions
                  announcements={selectedAnnouncements}
                  loadingAction={bulkAction}
                  onPublish={async () => {
                    try {
                      setBulkAction("publish");

                      await Promise.all(
                        selectedAnnouncements.map((item) => publishAnnouncement(item.id).unwrap()),
                      );

                      toast.success({
                        title: "Announcements published",
                        subtitle: `${selectedAnnouncements.length} announcement(s) published.`,
                      });
                    } catch {
                      toast.error({
                        title: "Publish failed",
                        subtitle: "Some announcements could not be published.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onMoveToDraft={async () => {
                    try {
                      setBulkAction("draft");

                      await Promise.all(
                        selectedAnnouncements.map((item) => draftAnnouncement(item.id).unwrap()),
                      );

                      toast.warning({
                        title: "Moved to draft",
                        subtitle: `${selectedAnnouncements.length} announcement(s) moved to draft.`,
                      });
                    } catch {
                      toast.error({
                        title: "Draft failed",
                        subtitle: "Some announcements could not be moved to draft.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onArchive={async () => {
                    try {
                      setBulkAction("archive");

                      await Promise.all(
                        selectedAnnouncements.map((item) => archiveAnnouncement(item.id).unwrap()),
                      );

                      toast.warning({
                        title: "Announcements archived",
                        subtitle: `${selectedAnnouncements.length} announcement(s) archived.`,
                      });
                    } catch {
                      toast.error({
                        title: "Archive failed",
                        subtitle: "Some announcements could not be archived.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onPin={async () => {
                    try {
                      setBulkAction("pin");

                      await Promise.all(
                        selectedAnnouncements.map((item) =>
                          setAnnouncementPinned({
                            id: item.id,
                            body: { is_pinned: true },
                          }).unwrap(),
                        ),
                      );

                      toast.success({
                        title: "Announcements pinned",
                        subtitle: `${selectedAnnouncements.length} announcement(s) pinned.`,
                      });
                    } catch {
                      toast.error({
                        title: "Pin failed",
                        subtitle: "Some announcements could not be pinned.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onUnpin={async () => {
                    try {
                      setBulkAction("unpin");

                      await Promise.all(
                        selectedAnnouncements.map((item) =>
                          setAnnouncementPinned({
                            id: item.id,
                            body: { is_pinned: false },
                          }).unwrap(),
                        ),
                      );

                      toast.success({
                        title: "Announcements unpinned",
                        subtitle: `${selectedAnnouncements.length} announcement(s) unpinned.`,
                      });
                    } catch {
                      toast.error({
                        title: "Unpin failed",
                        subtitle: "Some announcements could not be unpinned.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onDelete={async () => {
                    handleBulkDelete(selectedAnnouncements);
                  }}
                />

                <Table>
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
                      const announcement = row.cells.find((cell) => cell.info.header === "raw")
                        ?.value as Announcement;

                      const { key, ...rowProps } = getRowProps({ row });

                      return (
                        <TableRow key={key} {...rowProps}>
                          <TableSelectRow {...getSelectionProps({ row })} />

                          {row.cells.map((cell) => {
                            if (cell.info.header === "raw") return null;

                            if (cell.info.header === "title") {
                              const attachmentCount =
                                announcement.attachment_count ??
                                announcement.attachments?.length ??
                                0;
                              const hasLink = Boolean(announcement.link_url?.trim());

                              return (
                                <TableCell key={cell.id}>
                                  <div className="announcements-page__table-title">
                                    <strong>{announcement.title}</strong>

                                    {announcement.summary && (
                                      <span className="announcements-page__summary">
                                        {announcement.summary}
                                      </span>
                                    )}

                                    {announcement.tag && (
                                      <span>
                                        <Tag size="sm" type="warm-gray">
                                          {announcement.tag}
                                        </Tag>
                                      </span>
                                    )}

                                    {(hasLink || attachmentCount > 0) && (
                                      <span className="announcements-page__tag-list">
                                        {hasLink && (
                                          <Tag size="sm" type="blue">
                                            <span className="announcements-page__tag-content">
                                              <LinkIcon size={12} />
                                              Link
                                            </span>
                                          </Tag>
                                        )}

                                        {attachmentCount > 0 && (
                                          <Tag size="sm" type="cyan">
                                            <span className="announcements-page__tag-content">
                                              <Attachment size={12} />
                                              {attachmentCount === 1
                                                ? "1 attachment"
                                                : `${attachmentCount} attachments`}
                                            </span>
                                          </Tag>
                                        )}
                                      </span>
                                    )}
                                  </div>
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "attachments") {
                              const attachmentCount =
                                announcement.attachment_count ??
                                announcement.attachments?.length ??
                                0;

                              return (
                                <TableCell key={cell.id}>
                                  {attachmentCount > 0 ? (
                                    <Tag type="cyan" size="sm">
                                      <span className="announcements-page__tag-content">
                                        <Attachment size={12} />
                                        {attachmentCount}
                                      </span>
                                    </Tag>
                                  ) : (
                                    <Tag type="gray" size="sm">
                                      None
                                    </Tag>
                                  )}
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "level") {
                              return (
                                <TableCell key={cell.id}>
                                  <Tag type={getTagType(announcement.level)}>
                                    {announcement.level}
                                  </Tag>
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "status") {
                              return (
                                <TableCell key={cell.id}>
                                  <Tag type={getStatusTagType(announcement.status)}>
                                    {announcement.status}
                                  </Tag>
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "pinned") {
                              return (
                                <TableCell key={cell.id}>
                                  <Tag type={announcement.is_pinned ? "warm-gray" : "gray"}>
                                    {announcement.is_pinned ? "Pinned" : "No"}
                                  </Tag>
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "active") {
                              const active = isAnnouncementActive(announcement);

                              return (
                                <TableCell key={cell.id}>
                                  <Tag type={active ? "green" : "gray"}>
                                    {active ? "Active" : "Inactive"}
                                  </Tag>
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "actions") {
                              return (
                                <TableCell key={cell.id}>
                                  <AnnouncementActionsMenu
                                    announcement={announcement}
                                    onEdit={() => handleOpenEditPanel(announcement)}
                                    onPublish={async () => {
                                      try {
                                        await handlePublish(announcement);
                                      } catch {
                                        toast.error({
                                          title: "Publish failed",
                                          subtitle: "Failed to publish announcement.",
                                        });
                                      }
                                    }}
                                    onMoveToDraft={async () => {
                                      try {
                                        await handleMoveToDraft(announcement);
                                      } catch {
                                        toast.error({
                                          title: "Draft failed",
                                          subtitle: "Failed to move announcement to draft.",
                                        });
                                      }
                                    }}
                                    onTogglePin={async () => {
                                      try {
                                        await handleTogglePin(announcement);
                                      } catch {
                                        toast.error({
                                          title: "Pin update failed",
                                          subtitle: "Failed to update pin state.",
                                        });
                                      }
                                    }}
                                    onArchive={async () => {
                                      try {
                                        await handleArchive(announcement);
                                      } catch {
                                        toast.error({
                                          title: "Archive failed",
                                          subtitle: "Failed to archive announcement.",
                                        });
                                      }
                                    }}
                                    onDelete={async () => {
                                      try {
                                        await handleDelete(announcement);
                                      } catch {
                                        toast.error({
                                          title: "Delete failed",
                                          subtitle: "Failed to delete announcement.",
                                        });
                                      }
                                    }}
                                  />
                                </TableCell>
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

        <Pagination
          page={page}
          pageSize={pageSize}
          pageSizes={[10, 20, 30, 50]}
          totalItems={filteredAnnouncements.length}
          onChange={({ page, pageSize }) => {
            setPage(page);
            setPageSize(pageSize);
          }}
        />
      </Tile>
      </>}
      </PageShell>
    </div>
  );
}
