import { useRef, useState } from "react";
import { useHeaderPanel, useToast } from "@moh-sso/ui";
import {
  Button,
  InlineLoading,
  Modal,
  Select,
  SelectItem,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Tag,
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
import {
  DeliveredReportsTable,
  ExecutionsTable,
  SchedulesTable,
  formatReportDate,
  type ScheduleAction,
} from "./components/SchedulerTables";
import { useSchedulerQueryToasts } from "./hooks/useSchedulerQueryToasts";
import { useSchedulerPermissions } from "./hooks/useSchedulerPermissions";
import { canScheduleAction } from "./utils/scheduler-permissions";
import "./report-scheduler.scss";

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
  const access = useSchedulerPermissions();
  if (!access.read) return <p>You do not have permission to access Report Scheduler.</p>;
  return <ReportSchedulerWorkspace {...props} />;
}

type SchedulerTab = "schedules" | "executions" | "inbox";
function ReportSchedulerWorkspace(props: MicrofrontendRuntimeProps) {
  const access = useSchedulerPermissions();
  const currentUserId =
    props.auth?.user && typeof props.auth.user === "object" && "id" in props.auth.user
      ? String((props.auth.user as { id?: unknown }).id ?? "")
      : "";

  const [requestedTab, setSelectedTab] = useState<SchedulerTab>("schedules");
  const selectedTab = requestedTab === "executions" && !access.history ? "schedules" : requestedTab;
  const visibleTabs: SchedulerTab[] = access.history
    ? ["schedules", "executions", "inbox"]
    : ["schedules", "inbox"];
  const [scheduleSearch, setScheduleSearch] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState("all");
  const [executionSearch, setExecutionSearch] = useState("");
  const [executionStatus, setExecutionStatus] = useState("all");

  const moduleQuery = useGetReportSchedulerModuleQuery();
  const overviewQuery = useGetReportSchedulerOverviewQuery(undefined, {
    skip: !access.history,
    pollingInterval: 30_000,
  });
  const reportsQuery = useGetHealthBIReportsQuery(undefined, {
    skip: !(access.create || access.update),
  });
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
    { skip: !access.history, pollingInterval: 30_000 },
  );
  const portalReportsQuery = useGetPortalReportsQuery(100, {
    pollingInterval: selectedTab === "inbox" ? 30_000 : 0,
    refetchOnMountOrArgChange: true,
  });
  const [deleteSchedule] = useDeleteReportScheduleMutation();
  const [pauseSchedule] = usePauseReportScheduleMutation();
  const [resumeSchedule] = useResumeReportScheduleMutation();
  const [duplicateSchedule] = useDuplicateReportScheduleMutation();
  const [runNow] = useRunReportScheduleNowMutation();
  const [retryExecution, retryExecutionState] = useRetryReportExecutionMutation();

  const [selectedExecutionId, setSelectedExecutionId] = useState<string>();
  const [pendingSchedule, setPendingSchedule] = useState<{ id: string; action: ScheduleAction }>();
  const [deleteTarget, setDeleteTarget] = useState<ReportSchedule>();
  const { openPanel, closePanel } = useHeaderPanel();
  const toast = useToast();
  const panelInstance = useRef(0);

  const executionDetailQuery = useGetReportExecutionQuery(selectedExecutionId ?? "", {
    skip: !access.history || !selectedExecutionId || selectedTab !== "executions",
    pollingInterval: 15_000,
  });
  const openSchedulePanel = (schedule?: ReportSchedule) => {
    if (!(schedule ? access.update : access.create)) return;
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
            toast.success(schedule ? "Report schedule updated" : "Report schedule created");
            closePanel();
            void schedulesQuery.refetch();
          }}
        />
      ),
    });
  };

  const mutateSchedule = async (action: ScheduleAction, schedule: ReportSchedule) => {
    if (pendingSchedule || !canScheduleAction(access, action)) return false;
    setPendingSchedule({ id: schedule.id, action });
    try {
      if (action === "pause") await pauseSchedule(schedule.id).unwrap();
      if (action === "resume") await resumeSchedule(schedule.id).unwrap();
      if (action === "duplicate") await duplicateSchedule(schedule.id).unwrap();
      if (action === "run") await runNow(schedule.id).unwrap();
      if (action === "delete") await deleteSchedule(schedule.id).unwrap();
      const outcomes: Record<ScheduleAction, string> = {
        run: "Report execution started.",
        pause: "Schedule paused.",
        resume: "Schedule resumed.",
        duplicate: "Schedule duplicated.",
        delete: "Schedule deleted.",
      };
      toast.success(outcomes[action]);
      await schedulesQuery.refetch();
      await portalReportsQuery.refetch();
      if (access.history) await Promise.all([executionsQuery.refetch(), overviewQuery.refetch()]);
      if (action === "run" && access.history) setSelectedTab("executions");
      return true;
    } catch {
      toast.error(`Unable to ${action} this schedule`, "Please try again.");
      return false;
    } finally {
      setPendingSchedule(undefined);
    }
  };

  const handleRetryExecution = async (id: string) => {
    if (!access.execute || !access.history) return;
    try {
      const wasSelected = selectedExecutionId === id;
      await retryExecution(id).unwrap();
      setSelectedExecutionId(id);
      toast.success("Execution retry started");
      await Promise.all([executionsQuery.refetch(), overviewQuery.refetch()]);
      if (wasSelected) await executionDetailQuery.refetch();
    } catch {
      toast.error("Unable to retry this execution", "Please try again.");
    }
  };

  useSchedulerQueryToasts([
    {
      id: "module",
      active: moduleQuery.isError,
      kind: "error",
      title: "Report Scheduler is unavailable",
      subtitle: "Please try again or contact your portal administrator.",
    },
    {
      id: "datawarehouse-report-browser",
      active: !!moduleQuery.data && !moduleQuery.data.healthBiEnabled,
      kind: "warning",
      title: "Health BI is not configured",
      subtitle: "Contact your portal administrator to connect Health BI.",
    },
    {
      id: "catalogue",
      active:
        (access.create || access.update) &&
        reportsQuery.isError &&
        !!moduleQuery.data?.healthBiEnabled,
      kind: "error",
      title: "Unable to load Health BI reports",
    },
    {
      id: "worker",
      active: access.history && !!overviewQuery.data && !overviewQuery.data.workerHealthy,
      kind: "warning",
      title: "Scheduler worker needs attention",
      subtitle:
        overviewQuery.data?.workerLastCycleError ||
        "The scheduler has not reported a healthy cycle recently.",
    },
    {
      id: "schedules",
      active: selectedTab === "schedules" && schedulesQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load schedules",
      subtitle: "Use Refresh schedules to try again.",
    },
    {
      id: "executions",
      active: selectedTab === "executions" && executionsQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load execution history",
      subtitle: "Use Refresh history to try again.",
    },
    {
      id: "inbox",
      active: selectedTab === "inbox" && portalReportsQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load delivered reports",
      subtitle: "Use Refresh inbox to try again.",
    },
    {
      id: `detail-${selectedExecutionId}`,
      active: selectedTab === "executions" && !!selectedExecutionId && executionDetailQuery.isError,
      kind: "error",
      title: "Unable to load execution details",
    },
    {
      id: "overview",
      active: access.history && overviewQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load scheduler health",
    },
  ]);

  if (moduleQuery.isLoading) return <InlineLoading description="Loading Report Scheduler…" />;

  return (
    <div className="moh-microfrontend-root report-scheduler">
      <header>
        <h1>Report Scheduler</h1>
        <p>Schedule Health BI reports and deliver them by email or through the portal.</p>
      </header>

      {access.history && (
        <>
          <section className="report-scheduler__health" aria-label="Scheduler health">
            <div className="report-scheduler__health-bar">
              <div className="report-scheduler__health-title">
                <h2>Scheduler health</h2>
                <Tag
                  type={
                    !overviewQuery.data
                      ? "gray"
                      : overviewQuery.data.workerHealthy
                        ? "green"
                        : "red"
                  }
                  size="sm"
                >
                  {!overviewQuery.data
                    ? "Unavailable"
                    : overviewQuery.data.workerHealthy
                      ? "Healthy"
                      : "Attention required"}
                </Tag>
              </div>
              <span className="report-scheduler__health-heartbeat">
                Last heartbeat:{" "}
                {overviewQuery.data?.workerLastHeartbeatAt
                  ? formatReportDate(overviewQuery.data.workerLastHeartbeatAt)
                  : "Not recorded"}
              </span>
            </div>
            <details className="report-scheduler__health-details">
              <summary>View health details</summary>
              <div className="report-scheduler__status-grid">
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
            </details>
          </section>

          <div className="report-scheduler__summary">
            <Tile>
              <strong>{overviewQuery.data?.enabledSchedules ?? "—"}</strong>
              <span>Enabled schedules</span>
            </Tile>
            <Tile>
              <strong>{overviewQuery.data?.executions24h ?? "—"}</strong>
              <span>Executions · 24h</span>
              <small>
                {overviewQuery.data?.failed24h ?? "—"} failed ·{" "}
                {overviewQuery.data?.retryingNow ?? "—"} retrying now
              </small>
            </Tile>
            <Tile>
              <strong>
                {overviewQuery.data ? `${Math.round(overviewQuery.data.successRate24h)}%` : "—"}
              </strong>
              <span>Success rate · 24h</span>
              <small>
                {overviewQuery.data?.deliveryFailures24h ?? "—"} delivery failures · 24h
              </small>
            </Tile>
          </div>
        </>
      )}

      <section className="report-scheduler__workspace" aria-label="Report scheduler views">
        <Tabs
          selectedIndex={visibleTabs.indexOf(selectedTab)}
          onChange={({ selectedIndex }) => {
            setSelectedTab(visibleTabs[selectedIndex]);
            if (visibleTabs[selectedIndex] === "inbox") void portalReportsQuery.refetch();
          }}
        >
          <TabList aria-label="Report scheduler views" contained>
            {[
              <Tab key="schedules">Scheduled reports</Tab>,
              ...(access.history ? [<Tab key="executions">Execution history</Tab>] : []),
              <Tab key="inbox">Delivered to me</Tab>,
            ]}
          </TabList>
          <TabPanels>
            {[
              <TabPanel key="schedules" className="report-scheduler__tab-panel">
                <Tile>
                  <div className="report-scheduler__heading-row">
                    <h2>Scheduled reports</h2>
                    {access.create && (
                      <Button
                        size="sm"
                        disabled={!!pendingSchedule || !moduleQuery.data?.healthBiEnabled}
                        onClick={() => openSchedulePanel()}
                      >
                        Create schedule
                      </Button>
                    )}
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
                  <div className="report-scheduler__table-tools">
                    <span>Manage report timing and delivery.</span>
                    <div>
                      {(scheduleSearch.trim() || scheduleEnabled !== "all") && (
                        <Button
                          kind="ghost"
                          size="sm"
                          onClick={() => {
                            setScheduleSearch("");
                            setScheduleEnabled("all");
                          }}
                        >
                          Clear filters
                        </Button>
                      )}
                      <Button
                        kind="ghost"
                        size="sm"
                        disabled={schedulesQuery.isFetching}
                        onClick={() => void schedulesQuery.refetch()}
                      >
                        {schedulesQuery.isFetching ? "Refreshing…" : "Refresh schedules"}
                      </Button>
                    </div>
                  </div>
                  {schedulesQuery.isLoading && <InlineLoading description="Loading schedules…" />}
                  {!schedulesQuery.isLoading &&
                    (!schedulesQuery.isError || !!schedulesQuery.data?.length) && (
                      <SchedulesTable
                        key={`${scheduleSearch}:${scheduleEnabled}`}
                        rows={schedulesQuery.data ?? []}
                        access={access}
                        busy={!!pendingSchedule}
                        pending={pendingSchedule}
                        onEdit={openSchedulePanel}
                        onAction={(action, schedule) => {
                          if (!canScheduleAction(access, action)) return;
                          if (action === "delete") {
                            setDeleteTarget(schedule);
                          } else void mutateSchedule(action, schedule);
                        }}
                        empty={
                          <>
                            <strong>
                              {scheduleSearch.trim() || scheduleEnabled !== "all"
                                ? "No matching schedules"
                                : "No schedules yet"}
                            </strong>
                            <p>
                              {scheduleSearch.trim() || scheduleEnabled !== "all"
                                ? "Try another search or clear your filters."
                                : access.create
                                  ? "Create a schedule to start delivering reports automatically."
                                  : "No report schedules are available for your account."}
                            </p>
                          </>
                        }
                      />
                    )}
                </Tile>
              </TabPanel>,
              ...(access.history
                ? [
                    <TabPanel key="executions" className="report-scheduler__tab-panel">
                      <Tile>
                        <h2>Execution history</h2>
                        <div className="report-scheduler__filters">
                          <TextInput
                            id="report-execution-search"
                            labelText="Search executions"
                            placeholder="Report name or report ID"
                            value={executionSearch}
                            onChange={(event) => {
                              setExecutionSearch(event.target.value);
                              setSelectedExecutionId(undefined);
                            }}
                          />
                          <Select
                            id="report-execution-status"
                            labelText="Execution status"
                            value={executionStatus}
                            onChange={(event) => {
                              setExecutionStatus(event.target.value);
                              setSelectedExecutionId(undefined);
                            }}
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
                        <div className="report-scheduler__table-tools">
                          <span>Updates automatically every 30 seconds.</span>
                          <div>
                            {(executionSearch.trim() || executionStatus !== "all") && (
                              <Button
                                kind="ghost"
                                size="sm"
                                onClick={() => {
                                  setExecutionSearch("");
                                  setExecutionStatus("all");
                                  setSelectedExecutionId(undefined);
                                }}
                              >
                                Clear filters
                              </Button>
                            )}
                            <Button
                              kind="ghost"
                              size="sm"
                              disabled={executionsQuery.isFetching}
                              onClick={() => void executionsQuery.refetch()}
                            >
                              {executionsQuery.isFetching ? "Refreshing…" : "Refresh history"}
                            </Button>
                          </div>
                        </div>
                        {executionsQuery.isLoading && (
                          <InlineLoading description="Loading executions…" />
                        )}
                        {!executionsQuery.isLoading &&
                          (!executionsQuery.isError || !!executionsQuery.data?.length) && (
                            <ExecutionsTable
                              key={`${executionSearch}:${executionStatus}`}
                              rows={executionsQuery.data ?? []}
                              schedules={schedulesQuery.data ?? []}
                              selectedId={selectedExecutionId}
                              onDetails={(id) => setSelectedExecutionId(id)}
                              expandedDetails={
                                selectedExecutionId && (
                                  <div
                                    className="report-scheduler__execution-detail"
                                    role="region"
                                    aria-label="Execution details"
                                  >
                                    <h3>Execution details</h3>
                                    {executionDetailQuery.isFetching && (
                                      <InlineLoading description="Loading execution details…" />
                                    )}
                                    {executionDetailQuery.isError && (
                                      <Button
                                        kind="ghost"
                                        size="sm"
                                        disabled={executionDetailQuery.isFetching}
                                        onClick={() => void executionDetailQuery.refetch()}
                                      >
                                        Reload execution details
                                      </Button>
                                    )}
                                    {executionDetailQuery.currentData && (
                                      <>
                                        <div className="report-scheduler__detail-grid">
                                          <span>
                                            <strong>Status</strong>
                                            <br />
                                            {executionDetailQuery.currentData.execution.status}
                                          </span>
                                          <span>
                                            <strong>Generation attempts</strong>
                                            <br />
                                            {
                                              executionDetailQuery.currentData.execution
                                                .generationAttempts
                                            }
                                            /
                                            {
                                              executionDetailQuery.currentData.execution
                                                .maxGenerationAttempts
                                            }
                                          </span>
                                          <span>
                                            <strong>Period</strong>
                                            <br />
                                            {executionDetailQuery.currentData.execution
                                              .resolvedPeriod
                                              ? `${executionDetailQuery.currentData.execution.resolvedPeriod.strategy}: ${formatReportDate(executionDetailQuery.currentData.execution.resolvedPeriod.start)} – ${formatReportDate(executionDetailQuery.currentData.execution.resolvedPeriod.end)}`
                                              : "—"}
                                          </span>
                                          <span>
                                            <strong>Health scope</strong>
                                            <br />
                                            {executionDetailQuery.currentData.schedule
                                              ?.healthContext.facility ||
                                              executionDetailQuery.currentData.schedule
                                                ?.healthContext.district ||
                                              executionDetailQuery.currentData.schedule
                                                ?.healthContext.level ||
                                              "National"}
                                          </span>
                                        </div>

                                        {executionDetailQuery.currentData.artifacts.length > 0 && (
                                          <div className="report-scheduler__detail-section">
                                            <h3>Artifacts</h3>
                                            {executionDetailQuery.currentData.artifacts.map(
                                              (artifact) => (
                                                <div
                                                  key={artifact.id}
                                                  className="report-scheduler__detail-row"
                                                >
                                                  <span>{artifact.fileName}</span>
                                                  {artifact.downloadUrl && (
                                                    <Button
                                                      kind="ghost"
                                                      size="sm"
                                                      href={artifact.downloadUrl}
                                                    >
                                                      Download
                                                    </Button>
                                                  )}
                                                </div>
                                              ),
                                            )}
                                          </div>
                                        )}

                                        <div className="report-scheduler__detail-section">
                                          <h3>Deliveries</h3>
                                          {executionDetailQuery.currentData.deliveries.map(
                                            (delivery) => (
                                              <div
                                                key={delivery.id}
                                                className="report-scheduler__detail-row"
                                              >
                                                <div>
                                                  <strong>{delivery.deliveryChannel}</strong> ·{" "}
                                                  {delivery.recipientValue}
                                                  <p>
                                                    {delivery.status} · attempts {delivery.attempts}
                                                    /{delivery.maxAttempts}
                                                  </p>
                                                  {delivery.lastError && (
                                                    <small className="report-scheduler__error">
                                                      {delivery.lastError}
                                                    </small>
                                                  )}
                                                </div>
                                                {delivery.nextRetryAt && (
                                                  <small>
                                                    {formatReportDate(delivery.nextRetryAt)}
                                                  </small>
                                                )}
                                              </div>
                                            ),
                                          )}
                                          {!executionDetailQuery.currentData.deliveries.length && (
                                            <p>No recipient deliveries recorded.</p>
                                          )}
                                        </div>
                                      </>
                                    )}
                                  </div>
                                )
                              }
                              onRetry={(id) => void handleRetryExecution(id)}
                              canRetry={access.execute}
                              retrying={retryExecutionState.isLoading}
                              empty={
                                <>
                                  <strong>
                                    {executionSearch.trim() || executionStatus !== "all"
                                      ? "No matching executions"
                                      : "No executions yet"}
                                  </strong>
                                  <p>
                                    {executionSearch.trim() || executionStatus !== "all"
                                      ? "Try another search or clear your filters."
                                      : "Run a schedule to see its progress and delivery results here."}
                                  </p>
                                </>
                              }
                            />
                          )}
                      </Tile>
                    </TabPanel>,
                  ]
                : []),
              <TabPanel key="inbox" className="report-scheduler__tab-panel">
                <Tile>
                  <div className="report-scheduler__heading-row">
                    <h2>Delivered to me</h2>
                    <Button
                      kind="ghost"
                      size="sm"
                      disabled={portalReportsQuery.isFetching}
                      onClick={() => void portalReportsQuery.refetch()}
                    >
                      Refresh inbox
                    </Button>
                  </div>
                  {portalReportsQuery.isLoading && (
                    <InlineLoading description="Loading delivered reports…" />
                  )}
                  {!portalReportsQuery.isLoading &&
                    (!portalReportsQuery.isError || !!portalReportsQuery.data?.length) && (
                      <DeliveredReportsTable
                        rows={portalReportsQuery.data ?? []}
                        empty={
                          <>
                            <strong>No delivered reports yet</strong>
                            <p>
                              Only reports delivered to your signed-in account’s portal inbox appear
                              here.{" "}
                              {access.create || access.update
                                ? "In a schedule’s recipients, select your account under Portal recipients or choose “Deliver to my portal”."
                                : "Ask the schedule owner to include your account as a portal recipient."}{" "}
                              {access.history
                                ? "Email-only deliveries appear in Execution history."
                                : "Email-only deliveries are sent to email inboxes."}
                            </p>
                          </>
                        }
                      />
                    )}
                </Tile>
              </TabPanel>,
            ]}
          </TabPanels>
        </Tabs>
      </section>
      <Modal
        open={access.delete && !!deleteTarget}
        danger
        size="sm"
        modalHeading="Delete schedule?"
        primaryButtonText={pendingSchedule?.action === "delete" ? "Deleting…" : "Delete schedule"}
        secondaryButtonText="Cancel"
        primaryButtonDisabled={!!pendingSchedule}
        onRequestClose={() => {
          if (!pendingSchedule) setDeleteTarget(undefined);
        }}
        onRequestSubmit={async () => {
          if (!access.delete || !deleteTarget) return;
          if (await mutateSchedule("delete", deleteTarget)) setDeleteTarget(undefined);
        }}
      >
        <p>
          Delete <strong>{deleteTarget?.reportName}</strong>? Future runs will stop. Existing
          execution history and delivered reports will remain available.
        </p>
      </Modal>
    </div>
  );
}
