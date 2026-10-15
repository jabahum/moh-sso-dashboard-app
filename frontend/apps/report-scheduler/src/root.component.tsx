import { useEffect, useRef, useState } from "react";
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
import "./report-scheduler.scss";

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
  const currentUserId =
    props.auth?.user && typeof props.auth.user === "object" && "id" in props.auth.user
      ? String((props.auth.user as { id?: unknown }).id ?? "")
      : "";

  const [selectedTab, setSelectedTab] = useState(0);
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
  const [pendingSchedule, setPendingSchedule] = useState<{ id: string; action: ScheduleAction }>();
  const [deleteTarget, setDeleteTarget] = useState<ReportSchedule>();
  const executionDetailsRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (selectedExecutionId && selectedTab === 1) executionDetailsRef.current?.focus();
  }, [selectedExecutionId, selectedTab]);
  const { openPanel, closePanel } = useHeaderPanel();
  const toast = useToast();
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
            toast.success(schedule ? "Report schedule updated" : "Report schedule created");
            closePanel();
            void schedulesQuery.refetch();
          }}
        />
      ),
    });
  };

  const mutateSchedule = async (action: ScheduleAction, schedule: ReportSchedule) => {
    if (pendingSchedule) return false;
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
      await executionsQuery.refetch();
      await overviewQuery.refetch();
      if (action === "run") setSelectedTab(1);
      return true;
    } catch {
      toast.error(`Unable to ${action} this schedule`, "Please try again.");
      return false;
    } finally {
      setPendingSchedule(undefined);
    }
  };

  const handleRetryExecution = async (id: string) => {
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
      id: "health-bi",
      active: !!moduleQuery.data && !moduleQuery.data.healthBiEnabled,
      kind: "warning",
      title: "Health BI is not configured",
      subtitle: "Contact your portal administrator to connect Health BI.",
    },
    {
      id: "catalogue",
      active: reportsQuery.isError && !!moduleQuery.data?.healthBiEnabled,
      kind: "error",
      title: "Unable to load Health BI reports",
    },
    {
      id: "worker",
      active: !!overviewQuery.data && !overviewQuery.data.workerHealthy,
      kind: "warning",
      title: "Scheduler worker needs attention",
      subtitle:
        overviewQuery.data?.workerLastCycleError ||
        "The scheduler has not reported a healthy cycle recently.",
    },
    {
      id: "schedules",
      active: selectedTab === 0 && schedulesQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load schedules",
      subtitle: "Use Refresh schedules to try again.",
    },
    {
      id: "executions",
      active: selectedTab === 1 && executionsQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load execution history",
      subtitle: "Use Refresh history to try again.",
    },
    {
      id: "inbox",
      active: selectedTab === 2 && portalReportsQuery.isError && !moduleQuery.isError,
      kind: "error",
      title: "Unable to load delivered reports",
      subtitle: "Use Refresh inbox to try again.",
    },
    {
      id: `detail-${selectedExecutionId}`,
      active: selectedTab === 1 && !!selectedExecutionId && executionDetailQuery.isError,
      kind: "error",
      title: "Unable to load execution details",
    },
    {
      id: "overview",
      active: selectedTab === 3 && overviewQuery.isError && !moduleQuery.isError,
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

      <section className="report-scheduler__workspace" aria-label="Report scheduler views">
        <Tabs
          selectedIndex={selectedTab}
          onChange={({ selectedIndex }) => setSelectedTab(selectedIndex)}
        >
          <TabList aria-label="Report scheduler views" contained>
            <Tab>Scheduled reports</Tab>
            <Tab>Execution history</Tab>
            <Tab>Delivered to me</Tab>
            <Tab>Scheduler health</Tab>
          </TabList>
          <TabPanels>
            <TabPanel className="report-scheduler__tab-panel">
              <Tile>
                <div className="report-scheduler__heading-row">
                  <h2>Scheduled reports</h2>
                  <Button
                    size="sm"
                    disabled={!!pendingSchedule || !moduleQuery.data?.healthBiEnabled}
                    onClick={() => openSchedulePanel()}
                  >
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
                      busy={!!pendingSchedule}
                      pending={pendingSchedule}
                      onEdit={openSchedulePanel}
                      onAction={(action, schedule) => {
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
                              : "Create a schedule to start delivering reports automatically."}
                          </p>
                        </>
                      }
                    />
                  )}
              </Tile>
            </TabPanel>
            <TabPanel className="report-scheduler__tab-panel">
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
                {executionsQuery.isLoading && <InlineLoading description="Loading executions…" />}
                {!executionsQuery.isLoading &&
                  (!executionsQuery.isError || !!executionsQuery.data?.length) && (
                    <ExecutionsTable
                      key={`${executionSearch}:${executionStatus}`}
                      rows={executionsQuery.data ?? []}
                      schedules={schedulesQuery.data ?? []}
                      selectedId={selectedExecutionId}
                      onDetails={(id) => {
                        setSelectedExecutionId(id);
                        if (id === selectedExecutionId) executionDetailsRef.current?.focus();
                      }}
                      onRetry={(id) => void handleRetryExecution(id)}
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

                {selectedExecutionId && (
                  <div
                    className="report-scheduler__execution-detail"
                    ref={executionDetailsRef}
                    tabIndex={-1}
                    role="region"
                    aria-label="Execution details"
                  >
                    <div className="report-scheduler__heading-row">
                      <h3>Execution details</h3>
                      <Button
                        kind="ghost"
                        size="sm"
                        onClick={() => setSelectedExecutionId(undefined)}
                      >
                        Close
                      </Button>
                    </div>
                    {executionDetailQuery.isFetching && (
                      <InlineLoading description="Loading execution details…" />
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
                            {executionDetailQuery.currentData.execution.generationAttempts}/
                            {executionDetailQuery.currentData.execution.maxGenerationAttempts}
                          </span>
                          <span>
                            <strong>Period</strong>
                            <br />
                            {executionDetailQuery.currentData.execution.resolvedPeriod
                              ? `${executionDetailQuery.currentData.execution.resolvedPeriod.strategy}: ${formatReportDate(executionDetailQuery.currentData.execution.resolvedPeriod.start)} – ${formatReportDate(executionDetailQuery.currentData.execution.resolvedPeriod.end)}`
                              : "—"}
                          </span>
                          <span>
                            <strong>Health scope</strong>
                            <br />
                            {executionDetailQuery.currentData.schedule?.healthContext.facility ||
                              executionDetailQuery.currentData.schedule?.healthContext.district ||
                              executionDetailQuery.currentData.schedule?.healthContext.level ||
                              "National"}
                          </span>
                        </div>

                        {executionDetailQuery.currentData.artifacts.length > 0 && (
                          <div className="report-scheduler__detail-section">
                            <h3>Artifacts</h3>
                            {executionDetailQuery.currentData.artifacts.map((artifact) => (
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
                          {executionDetailQuery.currentData.deliveries.map((delivery) => (
                            <div key={delivery.id} className="report-scheduler__detail-row">
                              <div>
                                <strong>{delivery.deliveryChannel}</strong> ·{" "}
                                {delivery.recipientValue}
                                <p>
                                  {delivery.status} · attempts {delivery.attempts}/
                                  {delivery.maxAttempts}
                                </p>
                                {delivery.lastError && (
                                  <small className="report-scheduler__error">
                                    {delivery.lastError}
                                  </small>
                                )}
                              </div>
                              {delivery.nextRetryAt && (
                                <small>{formatReportDate(delivery.nextRetryAt)}</small>
                              )}
                            </div>
                          ))}
                          {!executionDetailQuery.currentData.deliveries.length && (
                            <p>No recipient deliveries recorded.</p>
                          )}
                        </div>
                      </>
                    )}
                  </div>
                )}
              </Tile>
            </TabPanel>
            <TabPanel className="report-scheduler__tab-panel">
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
                            Reports sent to your portal inbox will appear here, ready to download.
                          </p>
                        </>
                      }
                    />
                  )}
              </Tile>
            </TabPanel>
            <TabPanel className="report-scheduler__tab-panel">
              <Tile>
                <div className="report-scheduler__heading-row">
                  <h2>Scheduler health</h2>
                  <span>
                    {overviewQuery.data?.workerHealthy ? "Healthy" : "Attention required"}
                  </span>
                </div>
                <div className="report-scheduler__status-grid">
                  <div>
                    <strong>Worker heartbeat</strong>
                    <p>
                      {overviewQuery.data?.workerLastHeartbeatAt
                        ? formatReportDate(overviewQuery.data.workerLastHeartbeatAt)
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
            </TabPanel>
          </TabPanels>
        </Tabs>
      </section>
      <Modal
        open={!!deleteTarget}
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
          if (!deleteTarget) return;
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
