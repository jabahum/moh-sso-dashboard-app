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
  useGetHealthBIReportParametersQuery,
  useGetHealthBIReportsQuery,
  useGetPortalReportsQuery,
  useGetReportSchedulerModuleQuery,
  useGetReportSchedulesQuery,
  usePreviewReportRecipientsMutation,
} from "./api";
import type { CreateScheduleRequest, ScheduleRecipient } from "./types";
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

export function ReportSchedulerRoot(props: MicrofrontendRuntimeProps) {
  const currentUserId =
    props.auth?.user && typeof props.auth.user === "object" && "id" in props.auth.user
      ? String((props.auth.user as { id?: unknown }).id ?? "")
      : "";
  const moduleQuery = useGetReportSchedulerModuleQuery();
  const reportsQuery = useGetHealthBIReportsQuery();
  const schedulesQuery = useGetReportSchedulesQuery();
  const portalReportsQuery = useGetPortalReportsQuery();
  const [createSchedule, createState] = useCreateReportScheduleMutation();
  const [previewRecipients, previewState] = usePreviewReportRecipientsMutation();

  const [reportId, setReportId] = useState("");
  const [name, setName] = useState("");
  const [frequency, setFrequency] = useState("monthly");
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
  const selectedReport = reportsQuery.data?.find((report) => report.id === reportId);
  const formats = selectedReport?.supportedFormats?.length
    ? selectedReport.supportedFormats
    : ["pdf", "xlsx", "csv"];

  const recipients = useMemo<ScheduleRecipient[]>(() => [
    ...splitValues(emailRecipients).map((value) => ({ type: "email" as const, value, deliveryChannel: "email" as const })),
    ...splitValues(groupRecipients).map((value) => ({ type: "group" as const, value, deliveryChannel: "email" as const })),
    ...splitValues(portalRecipients).map((value) => ({ type: "user" as const, value, deliveryChannel: "portal" as const })),
  ], [emailRecipients, groupRecipients, portalRecipients]);

  const handleCreate = async () => {
    if (!selectedReport) return;
    const request: CreateScheduleRequest = {
      healthBiReportId: selectedReport.id,
      reportName: name.trim() || selectedReport.name,
      frequency,
      timezone: "Africa/Kampala",
      periodStrategy,
      parameters: parameterValues,
      outputFormat: format,
      outputConfig: { format, deliveryMode: "link", fileNamePrefix: fileNamePrefix.trim() },
      healthContext: {
        ...moduleQuery.data?.healthContext,
        district: healthDistrict || moduleQuery.data?.healthContext.district,
        facility: healthFacility || moduleQuery.data?.healthContext.facility,
      },
      recipients,
      enabled: true,
    };
    try {
      await createSchedule(request).unwrap();
      setMessage("Report schedule created.");
      await schedulesQuery.refetch();
    } catch {
      setMessage("Unable to create the report schedule. Review the report parameters and recipients.");
    }
  };

  const handlePreview = async () => {
    try {
      await previewRecipients(recipients).unwrap();
    } catch {
      setMessage("Unable to resolve one or more recipients.");
    }
  };

  if (moduleQuery.isLoading) {
    return <InlineLoading description="Loading Report Scheduler…" />;
  }

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
      {message && <InlineNotification kind="info" title={message} hideCloseButton />}

      <div className="report-scheduler__summary">
        <Tile>
          <strong>{schedulesQuery.data?.length ?? 0}</strong>
          <span>Schedules</span>
        </Tile>
        <Tile>
          <strong>{reportsQuery.data?.length ?? 0}</strong>
          <span>Health BI reports</span>
        </Tile>
        <Tile>
          <strong>{portalReportsQuery.data?.length ?? 0}</strong>
          <span>Delivered reports</span>
        </Tile>
      </div>

      <Tile className="report-scheduler__form">
        <h2>Create schedule</h2>
        <div className="report-scheduler__grid">
          <Select
            id="report-scheduler-report"
            labelText="Health BI report"
            value={reportId}
            onChange={(event) => {
              const next = event.target.value;
              setReportId(next);
              const report = reportsQuery.data?.find((item) => item.id === next);
              if (report) setName(report.name);
            }}
          >
            <SelectItem value="" text="Select a report" />
            {(reportsQuery.data ?? []).map((report) => (
              <SelectItem key={report.id} value={report.id} text={report.name} />
            ))}
          </Select>
          <TextInput id="report-scheduler-name" labelText="Schedule name" value={name} onChange={(event) => setName(event.target.value)} />
          <Select id="report-scheduler-frequency" labelText="Frequency" value={frequency} onChange={(event) => setFrequency(event.target.value)}>
            <SelectItem value="daily" text="Daily" />
            <SelectItem value="weekly" text="Weekly" />
            <SelectItem value="monthly" text="Monthly" />
            <SelectItem value="quarterly" text="Quarterly" />
          </Select>
          <Select id="report-scheduler-period" labelText="Reporting period" value={periodStrategy} onChange={(event) => setPeriodStrategy(event.target.value)}>
            <SelectItem value="previous_day" text="Previous day" />
            <SelectItem value="previous_week" text="Previous week" />
            <SelectItem value="previous_month" text="Previous month" />
            <SelectItem value="previous_quarter" text="Previous quarter" />
            <SelectItem value="previous_year" text="Previous year" />
          </Select>
          <Select id="report-scheduler-format" labelText="Output format" value={format} onChange={(event) => setFormat(event.target.value)}>
            {formats.map((item) => <SelectItem key={item} value={item.toLowerCase()} text={item.toUpperCase()} />)}
          </Select>
          <TextInput
            id="report-file-name-prefix"
            labelText="File name prefix"
            helperText="Optional prefix for generated report files"
            value={fileNamePrefix}
            onChange={(event) => setFileNamePrefix(event.target.value)}
          />
        </div>

        {parametersQuery.isFetching && <InlineLoading description="Loading report parameters…" />}
        {(parametersQuery.data ?? []).length > 0 && (
          <div className="report-scheduler__parameters">
            <h3>Report parameters</h3>
            <div className="report-scheduler__grid">
              {(parametersQuery.data ?? []).map((parameter) => {
                const options = (parameter.options ?? []).map(optionValue).filter((option): option is { value: string; label: string } => Boolean(option));
                const label = `${parameter.label || parameter.name}${parameter.required ? " *" : ""}`;
                if (options.length > 0) {
                  return (
                    <Select
                      key={parameter.name}
                      id={`report-parameter-${parameter.name}`}
                      labelText={label}
                      value={parameterValues[parameter.name] ?? ""}
                      onChange={(event) => setParameterValues((current) => ({ ...current, [parameter.name]: event.target.value }))}
                    >
                      <SelectItem value="" text="Select an option" />
                      {options.map((option) => <SelectItem key={option.value} value={option.value} text={option.label} />)}
                    </Select>
                  );
                }
                return (
                  <TextInput
                    key={parameter.name}
                    id={`report-parameter-${parameter.name}`}
                    labelText={label}
                    helperText={parameter.description}
                    value={parameterValues[parameter.name] ?? ""}
                    onChange={(event) => setParameterValues((current) => ({ ...current, [parameter.name]: event.target.value }))}
                  />
                );
              })}
            </div>
          </div>
        )}

        <div className="report-scheduler__parameters">
          <h3>Health scope</h3>
          <div className="report-scheduler__grid">
            <TextInput
              id="report-health-district"
              labelText="District"
              helperText={moduleQuery.data?.healthContext.district ? "Restricted by your signed-in health context" : "Optional district UID or name"}
              value={healthDistrict || moduleQuery.data?.healthContext.district || ""}
              disabled={Boolean(moduleQuery.data?.healthContext.district)}
              onChange={(event) => setHealthDistrict(event.target.value)}
            />
            <TextInput
              id="report-health-facility"
              labelText="Facility"
              helperText={moduleQuery.data?.healthContext.facility ? "Restricted by your signed-in health context" : "Optional facility UID or name"}
              value={healthFacility || moduleQuery.data?.healthContext.facility || ""}
              disabled={Boolean(moduleQuery.data?.healthContext.facility)}
              onChange={(event) => setHealthFacility(event.target.value)}
            />
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
            <Button kind="tertiary" size="sm" disabled={!recipients.length || previewState.isLoading} onClick={() => void handlePreview()}>
              Preview recipients
            </Button>
            {currentUserId && (
              <Button
                kind="ghost"
                size="sm"
                onClick={() => {
                  const values = splitValues(portalRecipients);
                  if (!values.includes(currentUserId)) {
                    setPortalRecipients([...values, currentUserId].join(", "));
                  }
                }}
              >
                Deliver to my portal
              </Button>
            )}
            {previewState.data && (
              <span>{previewState.data.emailCount} email · {previewState.data.portalCount} portal</span>
            )}
          </div>
        </div>

        <div className="report-scheduler__context">
          <strong>Health scope:</strong>{" "}
          {moduleQuery.data?.healthContext.facility ||
            moduleQuery.data?.healthContext.district ||
            moduleQuery.data?.healthContext.level ||
            "National"}
        </div>

        <Button disabled={!reportId || createState.isLoading || !moduleQuery.data?.healthBiEnabled} onClick={() => void handleCreate()}>
          {createState.isLoading ? "Creating…" : "Create schedule"}
        </Button>
      </Tile>

      <Tile>
        <h2>Scheduled reports</h2>
        <div className="report-scheduler__list">
          {(schedulesQuery.data ?? []).map((schedule) => (
            <article key={schedule.id}>
              <div>
                <strong>{schedule.reportName}</strong>
                <p>{schedule.frequency} · {schedule.periodStrategy} · {schedule.outputFormat.toUpperCase()}</p>
              </div>
              <span>{schedule.enabled ? "Active" : "Paused"}</span>
            </article>
          ))}
          {!schedulesQuery.isLoading && !(schedulesQuery.data?.length) && <p>No report schedules yet.</p>}
        </div>
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
              {report.artifact.externalUrl && (
                <Button kind="ghost" size="sm" href={report.artifact.externalUrl}>
                  Download
                </Button>
              )}
            </article>
          ))}
          {!portalReportsQuery.isLoading && !(portalReportsQuery.data?.length) && <p>No reports have been delivered to your portal yet.</p>}
        </div>
      </Tile>
    </div>
  );
}
