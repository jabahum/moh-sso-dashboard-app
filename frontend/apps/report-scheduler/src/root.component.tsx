import { useRef, useState } from "react";
import { useHeaderPanel } from "@moh-sso/ui";
import {
  Button,
  InlineLoading,
  InlineNotification,
  Select,
  SelectItem,
  TextInput,
  Tile,
} from "@carbon/react";
import type { MicrofrontendRuntimeProps } from "@moh-sso/microfrontend";
import {
  useDeleteReportScheduleMutation,
  useDuplicateReportScheduleMutation,
  useGetHealthBIReportsQuery,
  useGetPortalReportsQuery,
  useGetReportExecutionQuery,
  useGetReportExecutionsQuery,
  useGetReportSchedulerModuleQuery,
  useGetReportSchedulerOverviewQuery,
  useGetReportSchedulesQuery,
  usePauseReportScheduleMutation,
  useResumeReportScheduleMutation,
  useRunReportScheduleNowMutation,
  useRetryReportExecutionMutation,
} from "./api";
import type { ReportSchedule } from "./types";
import { ScheduleFormPanel } from "./components/ScheduleFormPanel";
import "./report-scheduler.scss";

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
  const currentUserId =
    props.auth?.user && typeof props.auth.user === "object" && "id" in props.auth.user
      ? String((props.auth.user as { id?: unknown }).id ?? "")
      : "";

  const [scheduleSearch, setScheduleSearch] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState("all");
  const [executionSearch, setExecutionSearch] = useState("");
  const [executionStatus, setExecutionStatus] = useState("all");

  const moduleQuery = useGetReportSchedulerModuleQuery();
  const overviewQuery = useGetReportSchedulerOverviewQuery(undefined, { pollingInterval: 30_000 });
  const reportsQuery = useGetHealthBIReportsQuery();
  const schedulesQuery = useGetReportSchedulesQuery({
    limit: 100,
    search: scheduleSearch.trim() || undefined,
    enabled: scheduleEnabled === "all" ? undefined : scheduleEnabled === "enabled",
  });
  const executionsQuery = useGetReportExecutionsQuery(
    {
      limit: 100,
      search: executionSearch.trim() || undefined,
      status: executionStatus === "all" ? undefined : executionStatus,
    },
    { pollingInterval: 30_000 },
  );
  const portalReportsQuery = useGetPortalReportsQuery(100);
  const [deleteSchedule] = useDeleteReportScheduleMutation();
  const [pauseSchedule] = usePauseReportScheduleMutation();
  const [resumeSchedule] = useResumeReportScheduleMutation();
  const [duplicateSchedule] = useDuplicateReportScheduleMutation();
  const [runNow] = useRunReportScheduleNowMutation();
  const [retryExecution, retryExecutionState] = useRetryReportExecutionMutation();

  const [selectedExecutionId, setSelectedExecutionId] = useState<string>();
  const [message, setMessage] = useState("");
  const { openPanel, closePanel } = useHeaderPanel();
  const panelInstance = useRef(0);

  const executionDetailQuery = useGetReportExecutionQuery(selectedExecutionId ?? "", {
    skip: !selectedExecutionId,
    pollingInterval: 15_000,
  });
  const openSchedulePanel = (schedule?: ReportSchedule) => {
    panelInstance.current += 1;
    openPanel({
      title: schedule ? "Edit schedule" : "Create schedule",
      size: "lg",
      content: (
        <ScheduleFormPanel
          key={panelInstance.current}
          initialSchedule={schedule}
          currentUserId={currentUserId}
          onCancel={closePanel}
          onSuccess={() => {
            setMessage(schedule ? "Report schedule updated." : "Report schedule created.");
            closePanel();
            void schedulesQuery.refetch();
          }}
        />
      ),
    });
  };

  const mutateSchedule = async (
    action: "pause" | "resume" | "duplicate" | "run" | "delete",
    schedule: ReportSchedule,
  ) => {
    try {
      if (action === "pause") await pauseSchedule(schedule.id).unwrap();
      if (action === "resume") await resumeSchedule(schedule.id).unwrap();
      if (action === "duplicate") await duplicateSchedule(schedule.id).unwrap();
      if (action === "run") await runNow(schedule.id).unwrap();
      if (action === "delete") await deleteSchedule(schedule.id).unwrap();
      setMessage(action === "run" ? "Report execution started." : `Schedule ${action} completed.`);
      await schedulesQuery.refetch();
      await portalReportsQuery.refetch();
      await executionsQuery.refetch();
      await overviewQuery.refetch();
    } catch {
      setMessage(`Unable to ${action} this schedule.`);
    }
  };

  if (moduleQuery.isLoading) return <InlineLoading description="Loading Report Scheduler…" />;

  return (
    <div className="moh-microfrontend-root report-scheduler">
      <header>
        <h1>Report Scheduler</h1>
        <p>Schedule Health BI reports and deliver them by email or through the portal.</p>
      </header>

      {(moduleQuery.isError || reportsQuery.isError) && (
        <InlineNotification
          kind="error"
          title="Report Scheduler is unavailable"
          subtitle="Check the SSO backend and Health BI integration."
          hideCloseButton
        />
      )}
      {moduleQuery.data && !moduleQuery.data.healthBiEnabled && (
        <InlineNotification
          kind="warning"
          title="Health BI is not configured"
          subtitle="Set HEALTH_BI_BASE_URL before creating schedules."
          hideCloseButton
        />
      )}
      {overviewQuery.data && !overviewQuery.data.workerHealthy && (
        <InlineNotification
          kind="warning"
          title="Scheduler worker heartbeat is stale"
          subtitle={
            overviewQuery.data.workerLastCycleError ||
            "The background scheduler has not reported a healthy cycle recently."
          }
          hideCloseButton
        />
      )}
      {message && <InlineNotification kind="info" title={message} hideCloseButton />}

      <div className="report-scheduler__summary">
        <Tile>
          <strong>{overviewQuery.data?.enabledSchedules ?? 0}</strong>
          <span>Enabled schedules</span>
        </Tile>
        <Tile>
          <strong>{overviewQuery.data?.executions24h ?? 0}</strong>
          <span>Executions · 24h</span>
        </Tile>
        <Tile>
          <strong>{Math.round(overviewQuery.data?.successRate24h ?? 0)}%</strong>
          <span>Success rate · 24h</span>
        </Tile>
        <Tile>
          <strong>{overviewQuery.data?.failed24h ?? 0}</strong>
          <span>Failed · 24h</span>
        </Tile>
        <Tile>
          <strong>{overviewQuery.data?.retryingNow ?? 0}</strong>
          <span>Retrying now</span>
        </Tile>
        <Tile>
          <strong>{overviewQuery.data?.deliveryFailures24h ?? 0}</strong>
          <span>Delivery failures · 24h</span>
        </Tile>
      </div>

      <Tile>
        <div className="report-scheduler__heading-row">
          <h2>Scheduler health</h2>
          <span>{overviewQuery.data?.workerHealthy ? "Healthy" : "Attention required"}</span>
        </div>
        <div className="report-scheduler__status-grid">
          <div>
            <strong>Worker heartbeat</strong>
            <p>
              {overviewQuery.data?.workerLastHeartbeatAt
                ? new Date(overviewQuery.data.workerLastHeartbeatAt).toLocaleString()
                : "No heartbeat recorded"}
            </p>
          </div>
          <div>
            <strong>Execution states</strong>
            <p>
              {(overviewQuery.data?.executionStatuses ?? [])
                .map((item) => `${item.status}: ${item.count}`)
                .join(" · ") || "No executions"}
            </p>
          </div>
          <div>
            <strong>Delivery states</strong>
            <p>
              {(overviewQuery.data?.deliveryStatuses ?? [])
                .map((item) => `${item.status}: ${item.count}`)
                .join(" · ") || "No deliveries"}
            </p>
          </div>
        </div>
      </Tile>

      <Tile>
        <div className="report-scheduler__heading-row">
          <h2>Scheduled reports</h2>
          <Button size="sm" onClick={() => openSchedulePanel()}>
            Create schedule
          </Button>
        </div>
        <div className="report-scheduler__filters">
          <TextInput
            id="report-schedule-search"
            labelText="Search schedules"
            placeholder="Report or Health BI ID"
            value={scheduleSearch}
            onChange={(event) => setScheduleSearch(event.target.value)}
          />
          <Select
            id="report-schedule-enabled"
            labelText="Schedule state"
            value={scheduleEnabled}
            onChange={(event) => setScheduleEnabled(event.target.value)}
          >
            <SelectItem value="all" text="All schedules" />
            <SelectItem value="enabled" text="Enabled" />
            <SelectItem value="paused" text="Paused" />
          </Select>
        </div>
        <div className="report-scheduler__list">
          {(schedulesQuery.data ?? []).map((schedule) => (
            <article key={schedule.id}>
              <div className="report-scheduler__schedule-copy">
                <strong>{schedule.reportName}</strong>
                <p>
                  {schedule.frequency} · {schedule.periodStrategy} ·{" "}
                  {schedule.outputFormat.toUpperCase()} · {schedule.timing?.timeOfDay || "08:00"}
                </p>
                <small>
                  {schedule.enabled
                    ? `Next run: ${schedule.nextRunAt ? new Date(schedule.nextRunAt).toLocaleString() : "calculating"}`
                    : "Paused"}
                </small>
              </div>
              <div className="report-scheduler__schedule-actions">
                <Button kind="ghost" size="sm" onClick={() => openSchedulePanel(schedule)}>
                  Edit
                </Button>
                <Button kind="ghost" size="sm" onClick={() => void mutateSchedule("run", schedule)}>
                  Run now
                </Button>
                <Button
                  kind="ghost"
                  size="sm"
                  onClick={() =>
                    void mutateSchedule(schedule.enabled ? "pause" : "resume", schedule)
                  }
                >
                  {schedule.enabled ? "Pause" : "Resume"}
                </Button>
                <Button
                  kind="ghost"
                  size="sm"
                  onClick={() => void mutateSchedule("duplicate", schedule)}
                >
                  Duplicate
                </Button>
                <Button
                  kind="danger--ghost"
                  size="sm"
                  onClick={() => void mutateSchedule("delete", schedule)}
                >
                  Delete
                </Button>
              </div>
            </article>
          ))}
          {!schedulesQuery.isLoading && !schedulesQuery.data?.length && (
            <p>No report schedules yet.</p>
          )}
        </div>
      </Tile>

      <Tile>
        <h2>Execution history</h2>
        <div className="report-scheduler__filters">
          <TextInput
            id="report-execution-search"
            labelText="Search executions"
            placeholder="Report name or report ID"
            value={executionSearch}
            onChange={(event) => setExecutionSearch(event.target.value)}
          />
          <Select
            id="report-execution-status"
            labelText="Execution status"
            value={executionStatus}
            onChange={(event) => setExecutionStatus(event.target.value)}
          >
            <SelectItem value="all" text="All statuses" />
            <SelectItem value="queued" text="Queued" />
            <SelectItem value="generating" text="Generating" />
            <SelectItem value="polling" text="Polling" />
            <SelectItem value="delivering" text="Delivering" />
            <SelectItem value="retrying" text="Retrying" />
            <SelectItem value="completed" text="Completed" />
            <SelectItem value="failed" text="Failed" />
            <SelectItem value="cancelled" text="Cancelled" />
          </Select>
        </div>
        <div className="report-scheduler__list">
          {(executionsQuery.data ?? []).map((execution) => (
            <article key={execution.id}>
              <div className="report-scheduler__schedule-copy">
                <strong>
                  {schedulesQuery.data?.find((schedule) => schedule.id === execution.scheduleId)
                    ?.reportName || execution.reportId}
                </strong>
                <p>
                  {execution.status} · {execution.triggerType} ·{" "}
                  {execution.outputFormat.toUpperCase()}
                </p>
                <small>
                  Attempts: {execution.generationAttempts}/{execution.maxGenerationAttempts}
                  {execution.nextRetryAt
                    ? ` · Retry: ${new Date(execution.nextRetryAt).toLocaleString()}`
                    : ""}
                  {execution.scheduledFor
                    ? ` · Scheduled: ${new Date(execution.scheduledFor).toLocaleString()}`
                    : ""}
                </small>
                {execution.errorMessage && (
                  <p className="report-scheduler__error">{execution.errorMessage}</p>
                )}
              </div>
              <div className="report-scheduler__schedule-actions">
                <Button kind="ghost" size="sm" onClick={() => setSelectedExecutionId(execution.id)}>
                  Details
                </Button>
                {execution.status === "failed" && (
                  <Button
                    kind="tertiary"
                    size="sm"
                    disabled={retryExecutionState.isLoading}
                    onClick={async () => {
                      try {
                        const wasSelected = selectedExecutionId === execution.id;
                        await retryExecution(execution.id).unwrap();
                        setSelectedExecutionId(execution.id);
                        setMessage("Execution retry started.");
                        await executionsQuery.refetch();
                        await overviewQuery.refetch();
                        if (wasSelected) await executionDetailQuery.refetch();
                      } catch {
                        setMessage("Unable to retry this execution.");
                      }
                    }}
                  >
                    Retry
                  </Button>
                )}
              </div>
            </article>
          ))}
          {!executionsQuery.isLoading && !executionsQuery.data?.length && (
            <p>No report executions yet.</p>
          )}
        </div>

        {selectedExecutionId && (
          <div className="report-scheduler__execution-detail">
            <div className="report-scheduler__heading-row">
              <h3>Execution details</h3>
              <Button kind="ghost" size="sm" onClick={() => setSelectedExecutionId(undefined)}>
                Close
              </Button>
            </div>
            {executionDetailQuery.isFetching && (
              <InlineLoading description="Loading execution details…" />
            )}
            {executionDetailQuery.data && (
              <>
                <div className="report-scheduler__detail-grid">
                  <span>
                    <strong>Status</strong>
                    <br />
                    {executionDetailQuery.data.execution.status}
                  </span>
                  <span>
                    <strong>Generation attempts</strong>
                    <br />
                    {executionDetailQuery.data.execution.generationAttempts}/
                    {executionDetailQuery.data.execution.maxGenerationAttempts}
                  </span>
                  <span>
                    <strong>Period</strong>
                    <br />
                    {executionDetailQuery.data.execution.resolvedPeriod
                      ? `${executionDetailQuery.data.execution.resolvedPeriod.strategy}: ${new Date(executionDetailQuery.data.execution.resolvedPeriod.start).toLocaleString()} – ${new Date(executionDetailQuery.data.execution.resolvedPeriod.end).toLocaleString()}`
                      : "—"}
                  </span>
                  <span>
                    <strong>Health scope</strong>
                    <br />
                    {executionDetailQuery.data.schedule?.healthContext.facility ||
                      executionDetailQuery.data.schedule?.healthContext.district ||
                      executionDetailQuery.data.schedule?.healthContext.level ||
                      "National"}
                  </span>
                </div>

                {executionDetailQuery.data.artifacts.length > 0 && (
                  <div className="report-scheduler__detail-section">
                    <h3>Artifacts</h3>
                    {executionDetailQuery.data.artifacts.map((artifact) => (
                      <div key={artifact.id} className="report-scheduler__detail-row">
                        <span>{artifact.fileName}</span>
                        {artifact.downloadUrl && (
                          <Button kind="ghost" size="sm" href={artifact.downloadUrl}>
                            Download
                          </Button>
                        )}
                      </div>
                    ))}
                  </div>
                )}

                <div className="report-scheduler__detail-section">
                  <h3>Deliveries</h3>
                  {executionDetailQuery.data.deliveries.map((delivery) => (
                    <div key={delivery.id} className="report-scheduler__detail-row">
                      <div>
                        <strong>{delivery.deliveryChannel}</strong> · {delivery.recipientValue}
                        <p>
                          {delivery.status} · attempts {delivery.attempts}/{delivery.maxAttempts}
                        </p>
                        {delivery.lastError && (
                          <small className="report-scheduler__error">{delivery.lastError}</small>
                        )}
                      </div>
                      {delivery.nextRetryAt && (
                        <small>{new Date(delivery.nextRetryAt).toLocaleString()}</small>
                      )}
                    </div>
                  ))}
                  {!executionDetailQuery.data.deliveries.length && (
                    <p>No recipient deliveries recorded.</p>
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </Tile>

      <Tile>
        <h2>Delivered to me</h2>
        <div className="report-scheduler__list">
          {(portalReportsQuery.data ?? []).map((report) => (
            <article key={report.deliveryId}>
              <div>
                <strong>{report.reportName}</strong>
                <p>{report.artifact.fileName}</p>
              </div>
              {report.artifact.downloadUrl && (
                <Button kind="ghost" size="sm" href={report.artifact.downloadUrl}>
                  Download
                </Button>
              )}
            </article>
          ))}
          {!portalReportsQuery.isLoading && !portalReportsQuery.data?.length && (
            <p>No reports have been delivered to your portal yet.</p>
          )}
        </div>
      </Tile>
    </div>
  );
}
