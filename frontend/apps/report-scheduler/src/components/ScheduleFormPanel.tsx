import { useMemo, useState } from "react";
import {
  Button,
  InlineLoading,
  InlineNotification,
  Select,
  SelectItem,
  TextInput,
} from "@carbon/react";
import {
  useCreateReportScheduleMutation,
  useUpdateReportScheduleMutation,
  useGetHealthBIReportsQuery,
  useGetHealthBIReportParametersQuery,
  useGetReportSchedulerModuleQuery,
  usePreviewReportRecipientsMutation,
} from "../api";
import type { CreateScheduleRequest, ReportSchedule, ScheduleRecipient } from "../types";
import "../report-scheduler.scss";

const splitValues = (value: string) =>
  value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

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
  Object.fromEntries(
    Object.entries(parameters ?? {}).map(([key, value]) => [
      key,
      value == null ? "" : String(value),
    ]),
  );

type ScheduleFormPanelProps = {
  initialSchedule?: ReportSchedule;
  currentUserId: string;
  onSuccess: () => void;
  onCancel: () => void;
};

export function ScheduleFormPanel({
  initialSchedule,
  currentUserId,
  onSuccess,
  onCancel,
}: ScheduleFormPanelProps) {
  const editingId = initialSchedule?.id;
  const moduleQuery = useGetReportSchedulerModuleQuery();
  const reportsQuery = useGetHealthBIReportsQuery();
  const [createSchedule, createState] = useCreateReportScheduleMutation();
  const [updateSchedule, updateState] = useUpdateReportScheduleMutation();
  const [previewRecipients, previewState] = usePreviewReportRecipientsMutation();
  const [reportId, setReportId] = useState(initialSchedule?.healthBiReportId ?? "");
  const [name, setName] = useState(initialSchedule?.reportName ?? "");
  const [frequency, setFrequency] = useState(initialSchedule?.frequency ?? "monthly");
  const [timeOfDay, setTimeOfDay] = useState(initialSchedule?.timing?.timeOfDay ?? "08:00");
  const [weekday, setWeekday] = useState(initialSchedule?.timing?.weekday ?? 1);
  const [dayOfMonth, setDayOfMonth] = useState(initialSchedule?.timing?.dayOfMonth ?? 1);
  const [periodStrategy, setPeriodStrategy] = useState(
    initialSchedule?.periodStrategy ?? "previous_month",
  );
  const [format, setFormat] = useState(initialSchedule?.outputFormat ?? "pdf");
  const [fileNamePrefix, setFileNamePrefix] = useState(
    initialSchedule?.outputConfig?.fileNamePrefix ?? "",
  );
  const [emailRecipients, setEmailRecipients] = useState(
    initialSchedule?.recipients
      .filter((r) => r.type === "email" && r.deliveryChannel === "email")
      .map((r) => r.value)
      .join(", ") ?? "",
  );
  const [groupRecipients, setGroupRecipients] = useState(
    initialSchedule?.recipients
      .filter((r) => r.type === "group" && r.deliveryChannel === "email")
      .map((r) => r.value)
      .join(", ") ?? "",
  );
  const [portalRecipients, setPortalRecipients] = useState(
    initialSchedule?.recipients
      .filter((r) => r.type === "user" && r.deliveryChannel === "portal")
      .map((r) => r.value)
      .join(", ") ?? "",
  );
  const [healthDistrict, setHealthDistrict] = useState(
    initialSchedule?.healthContext?.district ?? "",
  );
  const [healthFacility, setHealthFacility] = useState(
    initialSchedule?.healthContext?.facility ?? "",
  );
  const [parameterValues, setParameterValues] = useState<Record<string, string>>(() =>
    toStringParameters(initialSchedule?.parameters ?? {}),
  );
  const [message, setMessage] = useState("");

  const parametersQuery = useGetHealthBIReportParametersQuery(reportId, { skip: !reportId });
  const selectedReport = reportsQuery.data?.find((report) => report.id === reportId);
  const formats = selectedReport?.supportedFormats?.length
    ? selectedReport.supportedFormats
    : ["pdf", "xlsx", "csv"];

  const recipients = useMemo<ScheduleRecipient[]>(
    () => [
      ...splitValues(emailRecipients).map((value) => ({
        type: "email" as const,
        value,
        deliveryChannel: "email" as const,
      })),
      ...splitValues(groupRecipients).map((value) => ({
        type: "group" as const,
        value,
        deliveryChannel: "email" as const,
      })),
      ...splitValues(portalRecipients).map((value) => ({
        type: "user" as const,
        value,
        deliveryChannel: "portal" as const,
      })),
    ],
    [emailRecipients, groupRecipients, portalRecipients],
  );

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
      enabled: editingId ? (initialSchedule?.enabled ?? true) : true,
    };
  };

  const handleSave = async () => {
    const body = requestBody();
    if (!body) return;
    setMessage("");
    try {
      if (editingId) {
        await updateSchedule({ id: editingId, body }).unwrap();
      } else {
        await createSchedule(body).unwrap();
      }
      onSuccess();
    } catch {
      setMessage(
        "Unable to save the report schedule. Review timing, parameters, health scope, and recipients.",
      );
    }
  };

  const handlePreview = async () => {
    setMessage("");
    try {
      await previewRecipients(recipients).unwrap();
    } catch {
      setMessage("Unable to resolve one or more recipients.");
    }
  };

  return (
    <form
      className="report-scheduler__form"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSave();
      }}
    >
      {message && <InlineNotification kind="error" title={message} hideCloseButton />}
      <div className="report-scheduler__grid">
        <Select
          id="report-scheduler-report"
          labelText="Health BI report"
          value={reportId}
          onChange={(event) => {
            const next = event.target.value;
            setReportId(next);
            const report = reportsQuery.data?.find((item) => item.id === next);
            if (report && !editingId) setName(report.name);
          }}
        >
          <SelectItem value="" text="Select a report" />
          {(reportsQuery.data ?? []).map((report) => (
            <SelectItem key={report.id} value={report.id} text={report.name} />
          ))}
        </Select>
        <TextInput
          id="report-scheduler-name"
          labelText="Schedule name"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        <Select
          id="report-scheduler-frequency"
          labelText="Frequency"
          value={frequency}
          onChange={(event) => setFrequency(event.target.value)}
        >
          <SelectItem value="daily" text="Daily" />
          <SelectItem value="weekly" text="Weekly" />
          <SelectItem value="monthly" text="Monthly" />
          <SelectItem value="quarterly" text="Quarterly" />
          <SelectItem value="annual" text="Annually" />
        </Select>
        <TextInput
          id="report-scheduler-time"
          type="time"
          labelText="Run time"
          value={timeOfDay}
          onChange={(event) => setTimeOfDay(event.target.value)}
        />
        {frequency === "weekly" && (
          <Select
            id="report-scheduler-weekday"
            labelText="Day of week"
            value={String(weekday)}
            onChange={(event) => setWeekday(Number(event.target.value))}
          >
            <SelectItem value="1" text="Monday" />
            <SelectItem value="2" text="Tuesday" />
            <SelectItem value="3" text="Wednesday" />
            <SelectItem value="4" text="Thursday" />
            <SelectItem value="5" text="Friday" />
            <SelectItem value="6" text="Saturday" />
            <SelectItem value="7" text="Sunday" />
          </Select>
        )}
        {(frequency === "monthly" || frequency === "quarterly" || frequency === "annual") && (
          <TextInput
            id="report-scheduler-day"
            type="number"
            min={1}
            max={31}
            labelText="Day of month"
            value={String(dayOfMonth)}
            onChange={(event) => setDayOfMonth(Number(event.target.value))}
          />
        )}
        <Select
          id="report-scheduler-period"
          labelText="Reporting period"
          value={periodStrategy}
          onChange={(event) => setPeriodStrategy(event.target.value)}
        >
          <SelectItem value="current_day" text="Current day" />
          <SelectItem value="previous_day" text="Previous day" />
          <SelectItem value="current_week" text="Current week" />
          <SelectItem value="previous_week" text="Previous week" />
          <SelectItem value="current_epi_week" text="Current epi week" />
          <SelectItem value="previous_epi_week" text="Previous epi week" />
          <SelectItem value="current_month" text="Current month" />
          <SelectItem value="previous_month" text="Previous month" />
          <SelectItem value="current_quarter" text="Current quarter" />
          <SelectItem value="previous_quarter" text="Previous quarter" />
          <SelectItem value="current_year" text="Current year" />
          <SelectItem value="previous_year" text="Previous year" />
        </Select>
        <Select
          id="report-scheduler-format"
          labelText="Output format"
          value={format}
          onChange={(event) => setFormat(event.target.value)}
        >
          {formats.map((item) => (
            <SelectItem key={item} value={item.toLowerCase()} text={item.toUpperCase()} />
          ))}
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
              const options = (parameter.options ?? [])
                .map(optionValue)
                .filter((option): option is { value: string; label: string } => Boolean(option));
              const label = `${parameter.label || parameter.name}${parameter.required ? " *" : ""}`;
              if (options.length > 0)
                return (
                  <Select
                    key={parameter.name}
                    id={`report-parameter-${parameter.name}`}
                    labelText={label}
                    value={parameterValues[parameter.name] ?? ""}
                    onChange={(event) =>
                      setParameterValues((current) => ({
                        ...current,
                        [parameter.name]: event.target.value,
                      }))
                    }
                  >
                    <SelectItem value="" text="Select an option" />
                    {options.map((option) => (
                      <SelectItem key={option.value} value={option.value} text={option.label} />
                    ))}
                  </Select>
                );
              return (
                <TextInput
                  key={parameter.name}
                  id={`report-parameter-${parameter.name}`}
                  labelText={label}
                  helperText={parameter.description}
                  value={parameterValues[parameter.name] ?? ""}
                  onChange={(event) =>
                    setParameterValues((current) => ({
                      ...current,
                      [parameter.name]: event.target.value,
                    }))
                  }
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
            helperText={
              moduleQuery.data?.healthContext.district
                ? "Restricted by your signed-in health context"
                : "Optional district UID or name"
            }
            value={healthDistrict || moduleQuery.data?.healthContext.district || ""}
            disabled={Boolean(moduleQuery.data?.healthContext.district)}
            onChange={(event) => setHealthDistrict(event.target.value)}
          />
          <TextInput
            id="report-health-facility"
            labelText="Facility"
            helperText={
              moduleQuery.data?.healthContext.facility
                ? "Restricted by your signed-in health context"
                : "Optional facility UID or name"
            }
            value={healthFacility || moduleQuery.data?.healthContext.facility || ""}
            disabled={Boolean(moduleQuery.data?.healthContext.facility)}
            onChange={(event) => setHealthFacility(event.target.value)}
          />
        </div>
      </div>

      <div className="report-scheduler__parameters">
        <h3>Recipients</h3>
        <div className="report-scheduler__grid">
          <TextInput
            id="report-email-recipients"
            labelText="Email addresses"
            helperText="Comma-separated email addresses"
            value={emailRecipients}
            onChange={(event) => setEmailRecipients(event.target.value)}
          />
          <TextInput
            id="report-group-recipients"
            labelText="Keycloak groups"
            helperText="Comma-separated group IDs or /group/paths"
            value={groupRecipients}
            onChange={(event) => setGroupRecipients(event.target.value)}
          />
          <TextInput
            id="report-portal-recipients"
            labelText="Portal user IDs"
            helperText="Comma-separated Keycloak user UUIDs"
            value={portalRecipients}
            onChange={(event) => setPortalRecipients(event.target.value)}
          />
        </div>
        <div className="report-scheduler__actions">
          <Button
            kind="tertiary"
            size="sm"
            disabled={!recipients.length || previewState.isLoading}
            type="button"
            onClick={() => void handlePreview()}
          >
            Preview recipients
          </Button>
          {currentUserId && (
            <Button
              kind="ghost"
              size="sm"
              type="button"
              onClick={() => {
                const values = splitValues(portalRecipients);
                if (!values.includes(currentUserId))
                  setPortalRecipients([...values, currentUserId].join(", "));
              }}
            >
              Deliver to my portal
            </Button>
          )}
          {previewState.data && (
            <span>
              {previewState.data.emailCount} email · {previewState.data.portalCount} portal
            </span>
          )}
        </div>
      </div>

      <div className="report-scheduler__actions">
        <Button kind="secondary" type="button" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          type="submit"
          disabled={
            !reportId ||
            createState.isLoading ||
            updateState.isLoading ||
            !moduleQuery.data?.healthBiEnabled
          }
        >
          {createState.isLoading || updateState.isLoading
            ? "Saving…"
            : editingId
              ? "Save changes"
              : "Create schedule"}
        </Button>
      </div>
    </form>
  );
}
