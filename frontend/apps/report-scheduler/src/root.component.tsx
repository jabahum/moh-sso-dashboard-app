import { useMemo, useState } from "react";
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
  useCreateReportScheduleMutation,
  useDeleteReportScheduleMutation,
  useDuplicateReportScheduleMutation,
  useGetHealthBIReportParametersQuery,
  useGetHealthBIReportsQuery,
  useGetPortalReportsQuery,
  useGetReportExecutionQuery,
  useGetReportExecutionsQuery,
  useGetReportSchedulerModuleQuery,
  useGetReportSchedulerOverviewQuery,
  useGetReportSchedulesQuery,
  usePauseReportScheduleMutation,
  usePreviewReportRecipientsMutation,
  useResumeReportScheduleMutation,
  useRunReportScheduleNowMutation,
  useRetryReportExecutionMutation,
  useUpdateReportScheduleMutation,
} from "./api";
import type { CreateScheduleRequest, ReportSchedule, ScheduleRecipient } from "./types";
import "./report-scheduler.scss";

const splitValues = (value: string) =>
  value.split(",").map((item) => item.trim()).filter(Boolean);

const optionValue = (option: unknown) => {
  if (typeof option === "string" || typeof option === "number") {
    return { value: String(option), label: String(option) };
  }
  if (option && typeof option === "object") {
    const candidate = option as Record<string, unknown>;
    const value = candidate.value ?? candidate.id ?? candidate.code ?? candidate.name;
    const label = candidate.label ?? candidate.name ?? candidate.displayName ?? value;
    if (value !== undefined) return { value: String(value), label: String(label ?? value) };
  }
  return undefined;
};

const toStringParameters = (parameters: Record<string, unknown>) =>
  Object.fromEntries(Object.entries(parameters ?? {}).map(([key, value]) => [key, value == null ? "" : String(value)]));

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
  const currentUserId =
    props.auth?.user && typeof props.auth.user === "object" && "id" in props.auth.user
      ? String((props.auth.user as { id?: unknown }).id ?? "")
      : "";

  const moduleQuery = useGetReportSchedulerModuleQuery();
  const overviewQuery = useGetReportSchedulerOverviewQuery(undefined, { pollingInterval: 30_000 });
  const reportsQuery = useGetHealthBIReportsQuery();
  const schedulesQuery = useGetReportSchedulesQuery();
  const executionsQuery = useGetReportExecutionsQuery(undefined, { pollingInterval: 30_000 });
  const portalReportsQuery = useGetPortalReportsQuery();
  const [createSchedule, createState] = useCreateReportScheduleMutation();
  const [updateSchedule, updateState] = useUpdateReportScheduleMutation();
  const [deleteSchedule] = useDeleteReportScheduleMutation();
  const [pauseSchedule] = usePauseReportScheduleMutation();
  const [resumeSchedule] = useResumeReportScheduleMutation();
  const [duplicateSchedule] = useDuplicateReportScheduleMutation();
  const [runNow] = useRunReportScheduleNowMutation();
  const [retryExecution, retryExecutionState] = useRetryReportExecutionMutation();
  const [previewRecipients, previewState] = usePreviewReportRecipientsMutation();

  const [editingId, setEditingId] = useState<string>();
  const [selectedExecutionId, setSelectedExecutionId] = useState<string>();
  const [reportId, setReportId] = useState("");
  const [name, setName] = useState("");
  const [frequency, setFrequency] = useState("monthly");
  const [timeOfDay, setTimeOfDay] = useState("08:00");
  const [weekday, setWeekday] = useState(1);
  const [dayOfMonth, setDayOfMonth] = useState(1);
  const [periodStrategy, setPeriodStrategy] = useState("previous_month");
  const [format, setFormat] = useState("pdf");
  const [fileNamePrefix, setFileNamePrefix] = useState("");
  const [emailRecipients, setEmailRecipients] = useState("");
  const [groupRecipients, setGroupRecipients] = useState("");
  const [portalRecipients, setPortalRecipients] = useState("");
  const [healthDistrict, setHealthDistrict] = useState("");
  const [healthFacility, setHealthFacility] = useState("");
  const [parameterValues, setParameterValues] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");

  const parametersQuery = useGetHealthBIReportParametersQuery(reportId, { skip: !reportId });
  const executionDetailQuery = useGetReportExecutionQuery(selectedExecutionId ?? "", { skip: !selectedExecutionId, pollingInterval: 15_000 });
  const selectedReport = reportsQuery.data?.find((report) => report.id === reportId);
  const formats = selectedReport?.supportedFormats?.length
    ? selectedReport.supportedFormats
    : ["pdf", "xlsx", "csv"];

  const recipients = useMemo<ScheduleRecipient[]>(() => [
    ...splitValues(emailRecipients).map((value) => ({ type: "email" as const, value, deliveryChannel: "email" as const })),
    ...splitValues(groupRecipients).map((value) => ({ type: "group" as const, value, deliveryChannel: "email" as const })),
    ...splitValues(portalRecipients).map((value) => ({ type: "user" as const, value, deliveryChannel: "portal" as const })),
  ], [emailRecipients, groupRecipients, portalRecipients]);

  const resetForm = () => {
    setEditingId(undefined);
    setReportId("");
    setName("");
    setFrequency("monthly");
    setTimeOfDay("08:00");
    setWeekday(1);
    setDayOfMonth(1);
    setPeriodStrategy("previous_month");
    setFormat("pdf");
    setFileNamePrefix("");
    setEmailRecipients("");
    setGroupRecipients("");
    setPortalRecipients("");
    setHealthDistrict("");
    setHealthFacility("");
    setParameterValues({});
  };

  const requestBody = (): CreateScheduleRequest | undefined => {
    const report = reportsQuery.data?.find((item) => item.id === reportId);
    if (!report) return undefined;
    return {
      healthBiReportId: report.id,
      reportName: name.trim() || report.name,
      frequency,
      timezone: "Africa/Kampala",
      periodStrategy,
      parameters: parameterValues,
      outputFormat: format,
      outputConfig: { format, deliveryMode: "link", fileNamePrefix: fileNamePrefix.trim() },
      timing: { timeOfDay, weekday, dayOfMonth },
      healthContext: {
        ...moduleQuery.data?.healthContext,
        district: healthDistrict || moduleQuery.data?.healthContext.district,
        facility: healthFacility || moduleQuery.data?.healthContext.facility,
      },
      recipients,
      enabled: editingId
        ? schedulesQuery.data?.find((schedule) => schedule.id === editingId)?.enabled ?? true
        : true,
    };
  };

  const handleSave = async () => {
    const body = requestBody();
    if (!body) return;
    try {
      if (editingId) {
        await updateSchedule({ id: editingId, body }).unwrap();
        setMessage("Report schedule updated.");
      } else {
        await createSchedule(body).unwrap();
        setMessage("Report schedule created.");
      }
      resetForm();
      await schedulesQuery.refetch();
    } catch {
      setMessage("Unable to save the report schedule. Review timing, parameters, health scope, and recipients.");
    }
  };

  const handleEdit = (schedule: ReportSchedule) => {
    setEditingId(schedule.id);
    setReportId(schedule.healthBiReportId);
    setName(schedule.reportName);
    setFrequency(schedule.frequency);
    setTimeOfDay(schedule.timing?.timeOfDay || "08:00");
    setWeekday(schedule.timing?.weekday || 1);
    setDayOfMonth(schedule.timing?.dayOfMonth || 1);
    setPeriodStrategy(schedule.periodStrategy);
    setFormat(schedule.outputFormat);
    setFileNamePrefix(schedule.outputConfig?.fileNamePrefix || "");
    setParameterValues(toStringParameters(schedule.parameters));
    setHealthDistrict(schedule.healthContext?.district || "");
    setHealthFacility(schedule.healthContext?.facility || "");
    setEmailRecipients(schedule.recipients.filter((r) => r.type === "email" && r.deliveryChannel === "email").map((r) => r.value).join(", "));
    setGroupRecipients(schedule.recipients.filter((r) => r.type === "group" && r.deliveryChannel === "email").map((r) => r.value).join(", "));
    setPortalRecipients(schedule.recipients.filter((r) => r.type === "user" && r.deliveryChannel === "portal").map((r) => r.value).join(", "));
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const mutateSchedule = async (action: "pause" | "resume" | "duplicate" | "run" | "delete", schedule: ReportSchedule) => {
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

  const handlePreview = async () => {
    try {
      await previewRecipients(recipients).unwrap();
    } catch {
      setMessage("Unable to resolve one or more recipients.");
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
        <InlineNotification kind="error" title="Report Scheduler is unavailable" subtitle="Check the SSO backend and Health BI integration." hideCloseButton />
      )}
      {moduleQuery.data && !moduleQuery.data.healthBiEnabled && (
        <InlineNotification kind="warning" title="Health BI is not configured" subtitle="Set HEALTH_BI_BASE_URL before creating schedules." hideCloseButton />
      )}
      {overviewQuery.data && !overviewQuery.data.workerHealthy && (
        <InlineNotification
          kind="warning"
          title="Scheduler worker heartbeat is stale"
          subtitle={overviewQuery.data.workerLastCycleError || "The background scheduler has not reported a healthy cycle recently."}
          hideCloseButton
        />
      )}
      {message && <InlineNotification kind="info" title={message} hideCloseButton />}

      <div className="report-scheduler__summary">
        <Tile><strong>{overviewQuery.data?.enabledSchedules ?? 0}</strong><span>Enabled schedules</span></Tile>
        <Tile><strong>{overviewQuery.data?.executions24h ?? 0}</strong><span>Executions · 24h</span></Tile>
        <Tile><strong>{Math.round(overviewQuery.data?.successRate24h ?? 0)}%</strong><span>Success rate · 24h</span></Tile>
        <Tile><strong>{overviewQuery.data?.failed24h ?? 0}</strong><span>Failed · 24h</span></Tile>
        <Tile><strong>{overviewQuery.data?.retryingNow ?? 0}</strong><span>Retrying now</span></Tile>
        <Tile><strong>{overviewQuery.data?.deliveryFailures24h ?? 0}</strong><span>Delivery failures · 24h</span></Tile>
      </div>

      <Tile>
        <div className="report-scheduler__heading-row">
          <h2>Scheduler health</h2>
          <span>{overviewQuery.data?.workerHealthy ? "Healthy" : "Attention required"}</span>
        </div>
        <div className="report-scheduler__status-grid">
          <div>
            <strong>Worker heartbeat</strong>
            <p>{overviewQuery.data?.workerLastHeartbeatAt ? new Date(overviewQuery.data.workerLastHeartbeatAt).toLocaleString() : "No heartbeat recorded"}</p>
          </div>
          <div>
            <strong>Execution states</strong>
            <p>{(overviewQuery.data?.executionStatuses ?? []).map((item) => `${item.status}: ${item.count}`).join(" · ") || "No executions"}</p>
          </div>
          <div>
            <strong>Delivery states</strong>
            <p>{(overviewQuery.data?.deliveryStatuses ?? []).map((item) => `${item.status}: ${item.count}`).join(" · ") || "No deliveries"}</p>
          </div>
        </div>
      </Tile>

      <Tile className="report-scheduler__form">
        <div className="report-scheduler__heading-row">
          <h2>{editingId ? "Edit schedule" : "Create schedule"}</h2>
          {editingId && <Button kind="ghost" size="sm" onClick={resetForm}>Cancel edit</Button>}
        </div>
        <div className="report-scheduler__grid">
          <Select id="report-scheduler-report" labelText="Health BI report" value={reportId} onChange={(event) => {
            const next = event.target.value;
            setReportId(next);
            const report = reportsQuery.data?.find((item) => item.id === next);
            if (report && !editingId) setName(report.name);
          }}>
            <SelectItem value="" text="Select a report" />
            {(reportsQuery.data ?? []).map((report) => <SelectItem key={report.id} value={report.id} text={report.name} />)}
          </Select>
          <TextInput id="report-scheduler-name" labelText="Schedule name" value={name} onChange={(event) => setName(event.target.value)} />
          <Select id="report-scheduler-frequency" labelText="Frequency" value={frequency} onChange={(event) => setFrequency(event.target.value)}>
            <SelectItem value="daily" text="Daily" />
            <SelectItem value="weekly" text="Weekly" />
            <SelectItem value="monthly" text="Monthly" />
            <SelectItem value="quarterly" text="Quarterly" />
            <SelectItem value="annual" text="Annually" />
          </Select>
          <TextInput id="report-scheduler-time" type="time" labelText="Run time" value={timeOfDay} onChange={(event) => setTimeOfDay(event.target.value)} />
          {frequency === "weekly" && (
            <Select id="report-scheduler-weekday" labelText="Day of week" value={String(weekday)} onChange={(event) => setWeekday(Number(event.target.value))}>
              <SelectItem value="1" text="Monday" /><SelectItem value="2" text="Tuesday" /><SelectItem value="3" text="Wednesday" />
              <SelectItem value="4" text="Thursday" /><SelectItem value="5" text="Friday" /><SelectItem value="6" text="Saturday" /><SelectItem value="7" text="Sunday" />
            </Select>
          )}
          {(frequency === "monthly" || frequency === "quarterly" || frequency === "annual") && (
            <TextInput id="report-scheduler-day" type="number" min={1} max={31} labelText="Day of month" value={String(dayOfMonth)} onChange={(event) => setDayOfMonth(Number(event.target.value))} />
          )}
          <Select id="report-scheduler-period" labelText="Reporting period" value={periodStrategy} onChange={(event) => setPeriodStrategy(event.target.value)}>
            <SelectItem value="current_day" text="Current day" /><SelectItem value="previous_day" text="Previous day" />
            <SelectItem value="current_week" text="Current week" /><SelectItem value="previous_week" text="Previous week" />
            <SelectItem value="current_epi_week" text="Current epi week" /><SelectItem value="previous_epi_week" text="Previous epi week" />
            <SelectItem value="current_month" text="Current month" /><SelectItem value="previous_month" text="Previous month" />
            <SelectItem value="current_quarter" text="Current quarter" /><SelectItem value="previous_quarter" text="Previous quarter" />
            <SelectItem value="current_year" text="Current year" /><SelectItem value="previous_year" text="Previous year" />
          </Select>
          <Select id="report-scheduler-format" labelText="Output format" value={format} onChange={(event) => setFormat(event.target.value)}>
            {formats.map((item) => <SelectItem key={item} value={item.toLowerCase()} text={item.toUpperCase()} />)}
          </Select>
          <TextInput id="report-file-name-prefix" labelText="File name prefix" helperText="Optional prefix for generated report files" value={fileNamePrefix} onChange={(event) => setFileNamePrefix(event.target.value)} />
        </div>

        {parametersQuery.isFetching && <InlineLoading description="Loading report parameters…" />}
        {(parametersQuery.data ?? []).length > 0 && (
          <div className="report-scheduler__parameters">
            <h3>Report parameters</h3>
            <div className="report-scheduler__grid">
              {(parametersQuery.data ?? []).map((parameter) => {
                const options = (parameter.options ?? []).map(optionValue).filter((option): option is { value: string; label: string } => Boolean(option));
                const label = `${parameter.label || parameter.name}${parameter.required ? " *" : ""}`;
                if (options.length > 0) return (
                  <Select key={parameter.name} id={`report-parameter-${parameter.name}`} labelText={label} value={parameterValues[parameter.name] ?? ""} onChange={(event) => setParameterValues((current) => ({ ...current, [parameter.name]: event.target.value }))}>
                    <SelectItem value="" text="Select an option" />
                    {options.map((option) => <SelectItem key={option.value} value={option.value} text={option.label} />)}
                  </Select>
                );
                return <TextInput key={parameter.name} id={`report-parameter-${parameter.name}`} labelText={label} helperText={parameter.description} value={parameterValues[parameter.name] ?? ""} onChange={(event) => setParameterValues((current) => ({ ...current, [parameter.name]: event.target.value }))} />;
              })}
            </div>
          </div>
        )}

        <div className="report-scheduler__parameters">
          <h3>Health scope</h3>
          <div className="report-scheduler__grid">
            <TextInput id="report-health-district" labelText="District" helperText={moduleQuery.data?.healthContext.district ? "Restricted by your signed-in health context" : "Optional district UID or name"} value={healthDistrict || moduleQuery.data?.healthContext.district || ""} disabled={Boolean(moduleQuery.data?.healthContext.district)} onChange={(event) => setHealthDistrict(event.target.value)} />
            <TextInput id="report-health-facility" labelText="Facility" helperText={moduleQuery.data?.healthContext.facility ? "Restricted by your signed-in health context" : "Optional facility UID or name"} value={healthFacility || moduleQuery.data?.healthContext.facility || ""} disabled={Boolean(moduleQuery.data?.healthContext.facility)} onChange={(event) => setHealthFacility(event.target.value)} />
          </div>
        </div>

        <div className="report-scheduler__parameters">
          <h3>Recipients</h3>
          <div className="report-scheduler__grid">
            <TextInput id="report-email-recipients" labelText="Email addresses" helperText="Comma-separated email addresses" value={emailRecipients} onChange={(event) => setEmailRecipients(event.target.value)} />
            <TextInput id="report-group-recipients" labelText="Keycloak groups" helperText="Comma-separated group IDs or /group/paths" value={groupRecipients} onChange={(event) => setGroupRecipients(event.target.value)} />
            <TextInput id="report-portal-recipients" labelText="Portal user IDs" helperText="Comma-separated Keycloak user UUIDs" value={portalRecipients} onChange={(event) => setPortalRecipients(event.target.value)} />
          </div>
          <div className="report-scheduler__actions">
            <Button kind="tertiary" size="sm" disabled={!recipients.length || previewState.isLoading} onClick={() => void handlePreview()}>Preview recipients</Button>
            {currentUserId && <Button kind="ghost" size="sm" onClick={() => {
              const values = splitValues(portalRecipients);
              if (!values.includes(currentUserId)) setPortalRecipients([...values, currentUserId].join(", "));
            }}>Deliver to my portal</Button>}
            {previewState.data && <span>{previewState.data.emailCount} email · {previewState.data.portalCount} portal</span>}
          </div>
        </div>

        <Button disabled={!reportId || createState.isLoading || updateState.isLoading || !moduleQuery.data?.healthBiEnabled} onClick={() => void handleSave()}>
          {editingId ? "Save changes" : "Create schedule"}
        </Button>
      </Tile>

      <Tile>
        <h2>Scheduled reports</h2>
        <div className="report-scheduler__list">
          {(schedulesQuery.data ?? []).map((schedule) => (
            <article key={schedule.id}>
              <div className="report-scheduler__schedule-copy">
                <strong>{schedule.reportName}</strong>
                <p>{schedule.frequency} · {schedule.periodStrategy} · {schedule.outputFormat.toUpperCase()} · {schedule.timing?.timeOfDay || "08:00"}</p>
                <small>{schedule.enabled ? `Next run: ${schedule.nextRunAt ? new Date(schedule.nextRunAt).toLocaleString() : "calculating"}` : "Paused"}</small>
              </div>
              <div className="report-scheduler__schedule-actions">
                <Button kind="ghost" size="sm" onClick={() => handleEdit(schedule)}>Edit</Button>
                <Button kind="ghost" size="sm" onClick={() => void mutateSchedule("run", schedule)}>Run now</Button>
                <Button kind="ghost" size="sm" onClick={() => void mutateSchedule(schedule.enabled ? "pause" : "resume", schedule)}>{schedule.enabled ? "Pause" : "Resume"}</Button>
                <Button kind="ghost" size="sm" onClick={() => void mutateSchedule("duplicate", schedule)}>Duplicate</Button>
                <Button kind="danger--ghost" size="sm" onClick={() => void mutateSchedule("delete", schedule)}>Delete</Button>
              </div>
            </article>
          ))}
          {!schedulesQuery.isLoading && !(schedulesQuery.data?.length) && <p>No report schedules yet.</p>}
        </div>
      </Tile>

      <Tile>
        <h2>Execution history</h2>
        <div className="report-scheduler__list">
          {(executionsQuery.data ?? []).map((execution) => (
            <article key={execution.id}>
              <div className="report-scheduler__schedule-copy">
                <strong>{schedulesQuery.data?.find((schedule) => schedule.id === execution.scheduleId)?.reportName || execution.reportId}</strong>
                <p>{execution.status} · {execution.triggerType} · {execution.outputFormat.toUpperCase()}</p>
                <small>
                  Attempts: {execution.generationAttempts}/{execution.maxGenerationAttempts}
                  {execution.nextRetryAt ? ` · Retry: ${new Date(execution.nextRetryAt).toLocaleString()}` : ""}
                  {execution.scheduledFor ? ` · Scheduled: ${new Date(execution.scheduledFor).toLocaleString()}` : ""}
                </small>
                {execution.errorMessage && <p className="report-scheduler__error">{execution.errorMessage}</p>}
              </div>
              <div className="report-scheduler__schedule-actions">
                <Button kind="ghost" size="sm" onClick={() => setSelectedExecutionId(execution.id)}>Details</Button>
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
          {!executionsQuery.isLoading && !(executionsQuery.data?.length) && <p>No report executions yet.</p>}
        </div>

        {selectedExecutionId && (
          <div className="report-scheduler__execution-detail">
            <div className="report-scheduler__heading-row">
              <h3>Execution details</h3>
              <Button kind="ghost" size="sm" onClick={() => setSelectedExecutionId(undefined)}>Close</Button>
            </div>
            {executionDetailQuery.isFetching && <InlineLoading description="Loading execution details…" />}
            {executionDetailQuery.data && (
              <>
                <div className="report-scheduler__detail-grid">
                  <span><strong>Status</strong><br />{executionDetailQuery.data.execution.status}</span>
                  <span><strong>Generation attempts</strong><br />{executionDetailQuery.data.execution.generationAttempts}/{executionDetailQuery.data.execution.maxGenerationAttempts}</span>
                  <span>
                    <strong>Period</strong><br />
                    {executionDetailQuery.data.execution.resolvedPeriod
                      ? `${executionDetailQuery.data.execution.resolvedPeriod.strategy}: ${new Date(executionDetailQuery.data.execution.resolvedPeriod.start).toLocaleString()} – ${new Date(executionDetailQuery.data.execution.resolvedPeriod.end).toLocaleString()}`
                      : "—"}
                  </span>
                  <span><strong>Health scope</strong><br />{executionDetailQuery.data.schedule?.healthContext.facility || executionDetailQuery.data.schedule?.healthContext.district || executionDetailQuery.data.schedule?.healthContext.level || "National"}</span>
                </div>

                {executionDetailQuery.data.artifacts.length > 0 && (
                  <div className="report-scheduler__detail-section">
                    <h3>Artifacts</h3>
                    {executionDetailQuery.data.artifacts.map((artifact) => (
                      <div key={artifact.id} className="report-scheduler__detail-row">
                        <span>{artifact.fileName}</span>
                        {artifact.downloadUrl && <Button kind="ghost" size="sm" href={artifact.downloadUrl}>Download</Button>}
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
                        <p>{delivery.status} · attempts {delivery.attempts}/{delivery.maxAttempts}</p>
                        {delivery.lastError && <small className="report-scheduler__error">{delivery.lastError}</small>}
                      </div>
                      {delivery.nextRetryAt && <small>{new Date(delivery.nextRetryAt).toLocaleString()}</small>}
                    </div>
                  ))}
                  {!executionDetailQuery.data.deliveries.length && <p>No recipient deliveries recorded.</p>}
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
              <div><strong>{report.reportName}</strong><p>{report.artifact.fileName}</p></div>
              {report.artifact.downloadUrl && <Button kind="ghost" size="sm" href={report.artifact.downloadUrl}>Download</Button>}
            </article>
          ))}
          {!portalReportsQuery.isLoading && !(portalReportsQuery.data?.length) && <p>No reports have been delivered to your portal yet.</p>}
        </div>
      </Tile>
    </div>
  );
}
