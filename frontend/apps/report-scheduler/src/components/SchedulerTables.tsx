import { Fragment, useId, useState, type ReactNode } from "react";
import {
  Button,
  OverflowMenu,
  OverflowMenuItem,
  Pagination,
  Table,
  TableBody,
  TableCell,
  TableExpandHeader,
  TableExpandRow,
  TableExpandedRow,
  TableHead,
  TableHeader,
  TableRow,
  Tag,
} from "@carbon/react";
import type { SchedulerPermissions } from "../utils/scheduler-permissions";
import type { PortalReport, ReportExecution, ReportSchedule } from "../types";

export type ScheduleAction = "pause" | "resume" | "duplicate" | "run" | "delete";
export const readableLabel = (value: string) =>
  value.replace(/[_-]/g, " ").replace(/^./, (letter) => letter.toUpperCase());

const dateFormatter = new Intl.DateTimeFormat("en-GB", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "Africa/Kampala",
});
export const formatReportDate = (value?: string) => {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : dateFormatter.format(date);
};

function StatusTag({ status }: { status: string }) {
  const type =
    status === "completed" || status === "enabled"
      ? "green"
      : status === "failed"
        ? "red"
        : status === "retrying"
          ? "warm-gray"
          : ["queued", "generating", "polling", "delivering"].includes(status)
            ? "blue"
            : "gray";
  return (
    <Tag type={type} size="sm">
      {readableLabel(status)}
    </Tag>
  );
}

function PagedTable<T>({
  label,
  headers,
  rows,
  rowKey,
  renderRow,
  expansion,
  empty,
}: {
  label: string;
  headers: string[];
  rows: T[];
  rowKey: (row: T) => string;
  renderRow: (row: T) => ReactNode;
  expansion?: {
    selectedId?: string;
    onToggle: (id?: string) => void;
    label: (row: T) => string;
    content: ReactNode;
  };
  empty: ReactNode;
}) {
  const expansionHeaderId = useId();
  const [pagination, setPagination] = useState({ page: 1, pageSize: 10 });
  const page = Math.min(pagination.page, Math.max(1, Math.ceil(rows.length / pagination.pageSize)));
  const visibleRows = rows.slice((page - 1) * pagination.pageSize, page * pagination.pageSize);
  return (
    <div className="report-scheduler__table-container">
      <div
        className="report-scheduler__table-scroll"
        role="region"
        aria-label={`${label}, scroll to see all columns`}
        tabIndex={0}
      >
        <Table aria-label={label} size="lg" useZebraStyles>
          <TableHead>
            <TableRow>
              {expansion && (
                <TableExpandHeader id={expansionHeaderId} aria-label="Execution details" />
              )}
              {headers.map((header) => (
                <TableHeader key={header}>{header}</TableHeader>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {visibleRows.map((row) => {
              const id = rowKey(row);
              if (!expansion) return <TableRow key={id}>{renderRow(row)}</TableRow>;
              const expanded = expansion.selectedId === id;
              const detailsId = `${expansionHeaderId}-${id}`;
              return (
                <Fragment key={id}>
                  <TableExpandRow
                    expandHeader={expansionHeaderId}
                    aria-label={`${expanded ? "Collapse" : "Expand"} execution details for ${expansion.label(row)}`}
                    aria-controls={detailsId}
                    isExpanded={expanded}
                    onExpand={() => expansion.onToggle(expanded ? undefined : id)}
                  >
                    {renderRow(row)}
                  </TableExpandRow>
                  <TableExpandedRow id={detailsId} colSpan={headers.length + 1}>
                    {expanded && expansion.content}
                  </TableExpandedRow>
                </Fragment>
              );
            })}
            {!rows.length && (
              <TableRow>
                <TableCell colSpan={headers.length + (expansion ? 1 : 0)}>
                  <div className="report-scheduler__empty">{empty}</div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {rows.length > 0 && (
        <Pagination
          page={page}
          pageSize={pagination.pageSize}
          pageSizes={[10, 20, 50]}
          totalItems={rows.length}
          itemsPerPageText="Rows per page"
          onChange={({ page, pageSize }) => {
            setPagination({ page, pageSize });
            expansion?.onToggle(undefined);
          }}
        />
      )}
      <p className="report-scheduler__table-note">
        Up to 100 recent records. All times are in East Africa Time (Kampala).
      </p>
    </div>
  );
}

export function SchedulesTable({
  rows,
  access,
  busy,
  pending,
  onEdit,
  onAction,
  empty,
}: {
  rows: ReportSchedule[];
  access: SchedulerPermissions;
  busy: boolean;
  pending?: { id: string; action: ScheduleAction };
  onEdit: (schedule: ReportSchedule) => void;
  onAction: (action: ScheduleAction, schedule: ReportSchedule) => void;
  empty: ReactNode;
}) {
  return (
    <PagedTable
      label="Scheduled reports"
      headers={["Report", "Recurrence", "Reporting period", "Status", "Next run · EAT", "Actions"]}
      rows={rows}
      rowKey={(row) => row.id}
      empty={empty}
      renderRow={(schedule) => (
        <>
          <TableCell>
            <strong>{schedule.reportName}</strong>
            <span className="report-scheduler__cell-secondary">
              {schedule.outputFormat.toUpperCase()}
            </span>
          </TableCell>
          <TableCell>
            {readableLabel(schedule.frequency)}
            <span className="report-scheduler__cell-secondary">
              {schedule.timing?.timeOfDay || "08:00"} · {schedule.timezone}
            </span>
          </TableCell>
          <TableCell>{readableLabel(schedule.periodStrategy)}</TableCell>
          <TableCell>
            <StatusTag status={schedule.enabled ? "enabled" : "paused"} />
          </TableCell>
          <TableCell>
            {schedule.enabled
              ? schedule.nextRunAt
                ? formatReportDate(schedule.nextRunAt)
                : "Calculating…"
              : "Paused"}
          </TableCell>
          <TableCell>
            <div className="report-scheduler__row-actions">
              {access.execute && (
                <Button
                  kind="ghost"
                  size="sm"
                  disabled={busy}
                  aria-label={`Run ${schedule.reportName} now`}
                  onClick={() => onAction("run", schedule)}
                >
                  {pending?.id === schedule.id && pending.action === "run"
                    ? "Starting…"
                    : "Run now"}
                </Button>
              )}
              {(access.update || access.create || access.delete) && (
                <OverflowMenu flipped ariaLabel={`Actions for ${schedule.reportName}`}>
                  {access.update && (
                    <OverflowMenuItem
                      itemText="Edit schedule"
                      disabled={busy}
                      onClick={() => onEdit(schedule)}
                    />
                  )}
                  {access.update && (
                    <OverflowMenuItem
                      itemText={schedule.enabled ? "Pause schedule" : "Resume schedule"}
                      disabled={busy}
                      onClick={() => onAction(schedule.enabled ? "pause" : "resume", schedule)}
                    />
                  )}
                  {access.create && (
                    <OverflowMenuItem
                      itemText="Duplicate schedule"
                      disabled={busy}
                      onClick={() => onAction("duplicate", schedule)}
                    />
                  )}
                  {access.delete && (
                    <OverflowMenuItem
                      itemText="Delete schedule"
                      isDelete
                      hasDivider
                      disabled={busy}
                      onClick={() => onAction("delete", schedule)}
                    />
                  )}
                </OverflowMenu>
              )}
            </div>
          </TableCell>
        </>
      )}
    />
  );
}

export function ExecutionsTable({
  rows,
  schedules,
  onDetails,
  onRetry,
  retrying,
  canRetry,
  selectedId,
  expandedDetails,
  empty,
}: {
  rows: ReportExecution[];
  schedules: ReportSchedule[];
  onDetails: (id?: string) => void;
  onRetry: (id: string) => void;
  retrying: boolean;
  canRetry: boolean;
  selectedId?: string;
  expandedDetails: ReactNode;
  empty: ReactNode;
}) {
  return (
    <PagedTable
      label="Execution history"
      expansion={{
        selectedId,
        onToggle: onDetails,
        label: (execution) =>
          schedules.find((schedule) => schedule.id === execution.scheduleId)?.reportName ||
          execution.reportId,
        content: expandedDetails,
      }}
      headers={["Report", "Status", "Scheduled · EAT", "Attempts", "Format", "Actions"]}
      rows={rows}
      rowKey={(row) => row.id}
      empty={empty}
      renderRow={(execution) => (
        <>
          <TableCell>
            <strong>
              {schedules.find((schedule) => schedule.id === execution.scheduleId)?.reportName ||
                execution.reportId}
            </strong>
            <span className="report-scheduler__cell-secondary">
              {readableLabel(execution.triggerType)} run
            </span>
            {execution.errorMessage && (
              <span className="report-scheduler__cell-error">{execution.errorMessage}</span>
            )}
          </TableCell>
          <TableCell>
            <StatusTag status={execution.status} />
            {execution.nextRetryAt && (
              <span className="report-scheduler__cell-secondary">
                Retry: {formatReportDate(execution.nextRetryAt)}
              </span>
            )}
          </TableCell>
          <TableCell>{formatReportDate(execution.scheduledFor || execution.createdAt)}</TableCell>
          <TableCell>
            {execution.generationAttempts} / {execution.maxGenerationAttempts}
          </TableCell>
          <TableCell>{execution.outputFormat.toUpperCase()}</TableCell>
          <TableCell>
            <div className="report-scheduler__row-actions">
              {canRetry && execution.status === "failed" && execution.scheduleId && (
                <Button
                  kind="ghost"
                  size="sm"
                  disabled={retrying}
                  aria-label={`Retry ${execution.reportId}`}
                  onClick={() => onRetry(execution.id)}
                >
                  Retry
                </Button>
              )}
            </div>
          </TableCell>
        </>
      )}
    />
  );
}

export function DeliveredReportsTable({ rows, empty }: { rows: PortalReport[]; empty: ReactNode }) {
  return (
    <PagedTable
      label="Delivered reports"
      headers={["Report", "File", "Delivered · EAT", "Actions"]}
      rows={rows}
      rowKey={(row) => row.deliveryId}
      empty={empty}
      renderRow={(report) => (
        <>
          <TableCell>
            <strong>{report.reportName}</strong>
          </TableCell>
          <TableCell>{report.artifact.fileName}</TableCell>
          <TableCell>{formatReportDate(report.deliveredAt)}</TableCell>
          <TableCell>
            {report.artifact.downloadUrl ? (
              <Button
                kind="ghost"
                size="sm"
                href={report.artifact.downloadUrl}
                aria-label={`Download ${report.artifact.fileName}`}
              >
                Download
              </Button>
            ) : (
              <span className="report-scheduler__cell-secondary">Unavailable</span>
            )}
          </TableCell>
        </>
      )}
    />
  );
}
