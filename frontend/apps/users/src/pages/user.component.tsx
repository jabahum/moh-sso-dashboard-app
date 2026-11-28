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
import { useSelector } from "react-redux";
import { selectUser } from "@moh-sso/auth";

import {
  DataTablePagination,
  DataTableShell,
  ErrorState,
  RowActionsCell,
  TableStatusTag,
  useHeaderPanel,
  useToast,
} from "@moh-sso/ui";
import { UserAccessPanel } from "../components/access-control/UserAccessPanel";
import { UserFormPanel } from "../components/create-user-panel";
import { useEnableUserModal } from "../components/useEnableUserModal";
import { useResetPasswordModal } from "../components/useResetPasswordModal";
import { UserFilters } from "../components/UserFilters";
import { useListUsersQuery, useResetUserPasswordMutation, useToggleUserMutation } from "../api";
import type { User } from "../types";

import { UserActionsMenu } from "./user-actions-menu.component";
import { UserBulkActions } from "./user-bulk-actions.component";

/* -----------------------------
 * Table headers
 * ----------------------------- */
const headers = [
  { key: "username", header: "Username" },
  { key: "email", header: "Email" },
  { key: "status", header: "Status" },
  { key: "verified", header: "Email verified" },
  { key: "lastLogin", header: "Last login" },
  { key: "actions", header: "" },
  { key: "raw", header: "" },
];

/* Lower-priority columns are hidden on narrow screens so Status stays visible. */
const RESPONSIVE_COLUMN_CLASS: Record<string, string | undefined> = {
  email: "users-table__col--hide-sm",
  verified: "users-table__col--hide-md",
  lastLogin: "users-table__col--hide-md",
};

type BulkAction = "enable" | "disable" | null;

export default function UsersPage() {
  const currentUser = useSelector(selectUser);
  const { openPanel, closePanel } = useHeaderPanel();
  const { openEnableUserModal } = useEnableUserModal();
  const { openResetPasswordModal } = useResetPasswordModal();
  const toast = useToast();

  const [bulkAction, setBulkAction] = useState<BulkAction>(null);

  /* -----------------------------
   * Filters
   * ----------------------------- */
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [roleFilter, setRoleFilter] = useState("all");
  const [neverLoggedIn, setNeverLoggedIn] = useState(false);

  /* -----------------------------
   * Pagination
   * ----------------------------- */
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  /* -----------------------------
   * Data
   * ----------------------------- */
  const { data: users = [], isLoading, isError, error, refetch } = useListUsersQuery();

  const [toggleUser] = useToggleUserMutation();
  const [resetUserPassword] = useResetUserPasswordMutation();

  useEffect(() => {
    setPage(1);
  }, [statusFilter, roleFilter, neverLoggedIn, search]);

  /* -----------------------------
   * Roles
   * ----------------------------- */
  const roles = useMemo(() => {
    const set = new Set<string>();
    users.forEach((u) => {
      u.realmRoles?.forEach((r) => set.add(r));
    });
    return ["all", ...Array.from(set)];
  }, [users]);

  /* -----------------------------
   * Filtering
   * ----------------------------- */
  const filteredUsers = useMemo(() => {
    const query = search.trim().toLowerCase();
    return users.filter((u) => {
      if (
        query &&
        ![u.username, u.email, u.firstName, u.lastName].some((value) =>
          value?.toLowerCase().includes(query),
        )
      ) {
        return false;
      }
      const isEnabled = u.enabled ?? u.isActive ?? false;
      if (statusFilter === "active" && !isEnabled) return false;
      if (statusFilter === "disabled" && isEnabled) return false;
      if (neverLoggedIn && u.lastLoginAt) return false;
      if (roleFilter !== "all" && !u.realmRoles?.includes(roleFilter)) return false;
      return true;
    });
  }, [users, statusFilter, roleFilter, neverLoggedIn, search]);

  /* -----------------------------
   * Pagination
   * ----------------------------- */
  const paginatedUsers = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredUsers.slice(start, start + pageSize);
  }, [filteredUsers, page, pageSize]);

  /* -----------------------------
   * Rows
   * ----------------------------- */
  const rows = paginatedUsers.map((u) => ({
    id: u.id,
    username: u.username,
    email: u.email ?? "—",
    status: (u.enabled ?? u.isActive) ? "Active" : "Disabled",
    verified: u.emailVerified ? "Verified" : "Not verified",
    lastLogin: u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString() : "Never",
    actions: "",
    raw: u,
  }));

  const clearFilters = () => {
    setSearch("");
    setStatusFilter("all");
    setRoleFilter("all");
    setNeverLoggedIn(false);
  };

  const hasActiveFilters =
    search.trim() !== "" || statusFilter !== "all" || roleFilter !== "all" || neverLoggedIn;

  // Like Report Scheduler: only a loading indicator until the first data is ready.
  if (isLoading) return <InlineLoading description="Loading users…" />;

  return (
    <DataTableShell
      title="Users"
      description="Manage users, roles, and access to applications."
      rows={rows}
      headers={headers}
      getRowId={(row) => row.id}
      isLoading={isLoading}
      loadingDescription="Loading users…"
      tableState={
        isError ? (
          <ErrorState
            title="Failed to load users"
            description={getApiErrorMessage(error, "Failed to load users")}
            primaryAction={{ label: "Retry", onClick: refetch }}
          />
        ) : undefined
      }
      searchValue={search}
      onSearch={setSearch}
      searchPlaceholder="Search users by name or email"
      resultsDescription={
        !isLoading && !isError ? `${filteredUsers.length} of ${users.length} users` : undefined
      }
      emptyActions={
        users.length > 0 && hasActiveFilters ? (
          <Button kind="ghost" onClick={clearFilters}>
            Clear filters
          </Button>
        ) : undefined
      }
      emptyTitle={users.length === 0 ? "No users yet" : "No matching users"}
      emptyDescription={
        users.length === 0
          ? "Users will appear here once they are created or synced."
          : "No users match the current search or filters."
      }
      filters={
        <UserFilters
          onReset={clearFilters}
          status={statusFilter}
          roles={roles}
          selectedRole={roleFilter}
          neverLoggedIn={neverLoggedIn}
          hasActiveFilters={hasActiveFilters}
          onStatusChange={setStatusFilter}
          onRoleChange={setRoleFilter}
          onToggleNeverLoggedIn={() => {
            setNeverLoggedIn((v) => !v);
          }}
        />
      }
    >
      {({ rows, headers }) => (
        <DataTable rows={rows} headers={headers}>
          {({ rows, headers, getHeaderProps, getRowProps, getSelectionProps, selectedRows }) => {
            const selectedUsers = selectedRows.map(
              (r) => r.cells.find((c) => c.info.header === "raw")?.value as User,
            );

            return (
              <>
                {/* ================= BULK ACTIONS ================= */}
                <UserBulkActions
                  users={selectedUsers}
                  currentUserId={currentUser?.id}
                  loadingAction={bulkAction}
                  onEnable={async () => {
                    try {
                      setBulkAction("enable");

                      await Promise.all(
                        selectedUsers.map((u) => toggleUser({ id: u.id, enabled: true }).unwrap()),
                      );

                      toast.success({
                        title: "Users enabled",
                        subtitle: `${selectedUsers.length} user(s) enabled.`,
                      });
                    } catch {
                      toast.error({
                        title: "Enable failed",
                        subtitle: "Some users could not be enabled.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                  onDisable={async () => {
                    if (selectedUsers.some((user) => user.id === currentUser?.id)) {
                      toast.error({
                        title: "Cannot disable your own account",
                        subtitle: "Deselect your account before disabling other users.",
                      });
                      return;
                    }
                    try {
                      setBulkAction("disable");

                      await Promise.all(
                        selectedUsers.map((u) => toggleUser({ id: u.id, enabled: false }).unwrap()),
                      );

                      toast.warning({
                        title: "Users disabled",
                        subtitle: `${selectedUsers.length} user(s) disabled.`,
                      });
                    } catch {
                      toast.error({
                        title: "Disable failed",
                        subtitle: "Some users could not be disabled.",
                      });
                    } finally {
                      setBulkAction(null);
                    }
                  }}
                />

                {/* ================= TABLE ================= */}
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableSelectAll {...getSelectionProps()} />
                      {headers
                        .filter((h) => h.key !== "raw")
                        .map((h) => (
                          <TableHeader
                            {...getHeaderProps({ header: h })}
                            className={RESPONSIVE_COLUMN_CLASS[h.key]}
                          >
                            {h.header}
                          </TableHeader>
                        ))}
                    </TableRow>
                  </TableHead>

                  <TableBody>
                    {rows.map((row) => {
                      const user = row.cells.find((c) => c.info.header === "raw")?.value as User;

                      return (
                        <TableRow {...getRowProps({ row })}>
                          <TableSelectRow {...getSelectionProps({ row })} />

                          {row.cells.map((cell) => {
                            if (cell.info.header === "raw") return null;

                            if (cell.info.header === "status") {
                              return (
                                <TableCell key={cell.id}>
                                  <TableStatusTag status={String(cell.value)} />
                                </TableCell>
                              );
                            }

                            if (cell.info.header === "actions") {
                              return (
                                <RowActionsCell key={cell.id}>
                                  <UserActionsMenu
                                    user={user}
                                    isCurrentUser={user.id === currentUser?.id}
                                    onEdit={() => {
                                      openPanel({
                                        title: "Edit user",
                                        content: (
                                          <UserFormPanel
                                            key={`edit-user-${user.id}`}
                                            mode="edit"
                                            initialUser={user}
                                            onSuccess={closePanel}
                                          />
                                        ),
                                        size: "md",
                                      });
                                    }}
                                    onManageRoles={() => {
                                      openPanel({
                                        title: `Roles: ${user.username}`,
                                        size: "lg",
                                        content: (
                                          <UserAccessPanel
                                            key={`user-access-${user.id}`}
                                            userId={user.id}
                                          />
                                        ),
                                      });
                                    }}
                                    onToggleStatus={() => {
                                      const isEnabled = user.enabled ?? user.isActive ?? false;

                                      openEnableUserModal({
                                        username: user.username,
                                        enabled: isEnabled,
                                        onConfirm: async () => {
                                          try {
                                            await toggleUser({
                                              id: user.id,
                                              enabled: !isEnabled,
                                            }).unwrap();

                                            toast.success({
                                              title: "User updated",
                                              subtitle: `${user.username} ${
                                                isEnabled ? "disabled" : "enabled"
                                              }.`,
                                            });
                                          } catch (err) {
                                            toast.error({
                                              title: "Update failed",
                                              subtitle: getApiErrorMessage(
                                                err,
                                                `Failed to update ${user.username}.`,
                                              ),
                                            });
                                          }
                                        },
                                      });
                                    }}
                                    onResetPassword={() => {
                                      openResetPasswordModal({
                                        username: user.username,
                                        email: user.email,
                                        onConfirm: async () => {
                                          try {
                                            await resetUserPassword(user.id).unwrap();

                                            toast.success({
                                              title: "Password reset sent",
                                              subtitle: user.email
                                                ? `Reset email sent to ${user.email}.`
                                                : `Password reset started for ${user.username}.`,
                                            });
                                          } catch (err) {
                                            toast.error({
                                              title: "Password reset failed",
                                              subtitle: getApiErrorMessage(
                                                err,
                                                `Failed to reset password for ${user.username}.`,
                                              ),
                                            });
                                          }
                                        },
                                      });
                                    }}
                                  />
                                </RowActionsCell>
                              );
                            }

                            return (
                              <TableCell
                                key={cell.id}
                                className={RESPONSIVE_COLUMN_CLASS[cell.info.header]}
                              >
                                {cell.value}
                              </TableCell>
                            );
                          })}
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>

                <DataTablePagination
                  page={page}
                  pageSize={pageSize}
                  totalItems={filteredUsers.length}
                  onChange={({ page, pageSize }) => {
                    setPage(page);
                    setPageSize(pageSize);
                  }}
                />
              </>
            );
          }}
        </DataTable>
      )}
    </DataTableShell>
  );
}

function getApiErrorMessage(error: unknown, fallback: string): string {
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

  if (typeof maybeError.data?.message === "string") {
    return maybeError.data.message;
  }

  if (typeof maybeError.data?.error?.message === "string") {
    return maybeError.data.error.message;
  }

  if (typeof maybeError.error === "string") {
    return maybeError.error;
  }

  return fallback;
}
