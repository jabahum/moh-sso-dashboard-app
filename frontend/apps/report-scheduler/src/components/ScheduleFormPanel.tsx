import { useMemo, useState } from "react";
import { Button, InlineLoading, Select, SelectItem, TextInput } from "@carbon/react";
import {
  useCreateReportScheduleMutation,
  useUpdateReportScheduleMutation,
  useGetHealthBIReportsQuery,
  useGetHealthBIReportQuery,
  useGetHealthBIReportParametersQuery,
  useGetReportSchedulerModuleQuery,
  usePreviewReportRecipientsMutation,
} from "../api";
import type {
  CreateScheduleRequest,
  ReportSchedule,
  ScheduleRecipient,
  RecipientOption,
} from "../types";
import { useToast } from "@moh-sso/ui";
import { useSchedulerQueryToasts } from "../hooks/useSchedulerQueryToasts";
import { parameterInputValue, populateReportParameters } from "../utils/report-parameters";
import { RecipientSelect } from "./RecipientSelect";
import { useSchedulerPermissions } from "../hooks/useSchedulerPermissions";
import "../report-scheduler.scss";

const periodOptions = [
  ["current_day", "Current day"],
  ["previous_day", "Previous day"],
  ["current_week", "Current week"],
  ["previous_week", "Previous week"],
  ["current_epi_week", "Current epi week"],
  ["previous_epi_week", "Previous epi week"],
  ["current_month", "Current month"],
  ["previous_month", "Previous month"],
  ["current_quarter", "Current quarter"],
  ["previous_quarter", "Previous quarter"],
  ["current_year", "Current year"],
  ["previous_year", "Previous year"],
];

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

type ScheduleFormPanelProps = {
  initialSchedule?: ReportSchedule;
  currentUserId: string;
  onSuccess: () => void;
  onCancel: () => void;
};

export function ScheduleFormPanel(props: ScheduleFormPanelProps) {
  const access = useSchedulerPermissions();
  if (!(props.initialSchedule ? access.update : access.create))
    return (
      <div>
        <p>
          You do not have permission to {props.initialSchedule ? "edit" : "create"} report
          schedules.
        </p>
        <Button kind="secondary" onClick={props.onCancel}>
          Close
        </Button>
      </div>
    );
  return <ScheduleFormContent {...props} />;
}

function ScheduleFormContent({
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
  const asOption = (recipient: ScheduleRecipient): RecipientOption => ({
    id: `${recipient.type}:${recipient.value}`,
    type: recipient.type,
    value: recipient.value,
    label: recipient.value,
  });
  const [emailRecipients, setEmailRecipients] = useState<RecipientOption[]>(() =>
    (initialSchedule?.recipients ?? [])
      .filter((recipient) => recipient.deliveryChannel === "email" && recipient.type !== "group")
      .map(asOption),
  );
  const [groupRecipients, setGroupRecipients] = useState<RecipientOption[]>(() =>
    (initialSchedule?.recipients ?? [])
      .filter((recipient) => recipient.deliveryChannel === "email" && recipient.type === "group")
      .map(asOption),
  );
  const [portalRecipients, setPortalRecipients] = useState<RecipientOption[]>(() =>
    (initialSchedule?.recipients ?? [])
      .filter((recipient) => recipient.deliveryChannel === "portal")
      .map(asOption),
  );
  const [externalEmails, setExternalEmails] = useState("");
  const [healthDistrict, setHealthDistrict] = useState(
    initialSchedule?.healthContext?.district ?? "",
  );
  const [healthFacility, setHealthFacility] = useState(
    initialSchedule?.healthContext?.facility ?? "",
  );
  const [parameterOverrides, setParameterOverrides] = useState<Record<string, unknown>>(
    initialSchedule?.parameters ?? {},
  );
  const toast = useToast();

  const parametersQuery = useGetHealthBIReportParametersQuery(reportId, { skip: !reportId });
  const reportQuery = useGetHealthBIReportQuery(reportId, { skip: !reportId });
  const selectedReport = reportQuery.currentData;
  const parameterSchema = parametersQuery.currentData;
  const parameterValues = useMemo(
    () => populateReportParameters(parameterSchema ?? [], parameterOverrides),
    [parameterSchema, parameterOverrides],
  );
  const formats = selectedReport?.supportedFormats ?? [];
  const supportedPeriods = selectedReport?.metadata?.periodStrategies;
  const availablePeriods = periodOptions.filter(
    ([value]) => supportedPeriods === undefined || supportedPeriods.includes(value),
  );
  const keepingSavedReport = initialSchedule?.healthBiReportId === reportId;
  const selectedPeriod = availablePeriods.some(([value]) => value === periodStrategy)
    ? periodStrategy
    : keepingSavedReport
      ? ""
      : (availablePeriods.find(([value]) => value.startsWith("previous_"))?.[0] ??
        availablePeriods[0]?.[0] ??
        "");
  const selectedFormat = formats.includes(format)
    ? format
    : keepingSavedReport
      ? ""
      : (formats[0] ?? "");
  const periodSupported = availablePeriods.some(([value]) => value === selectedPeriod);
  const formatSupported = formats.includes(selectedFormat);

  useSchedulerQueryToasts([
    {
      id: `configuration-${reportId}`,
      active: reportQuery.isError || parametersQuery.isError,
      kind: "error",
      title: "Unable to load report configuration",
      subtitle: "Select the report again to retry.",
    },
    {
      id: `periods-${reportId}`,
      active: !!selectedReport && availablePeriods.length === 0,
      kind: "warning",
      title: "This report does not support scheduled reporting periods",
      subtitle: "Choose another Health BI report.",
    },
  ]);

  const recipients = useMemo<ScheduleRecipient[]>(() => {
    const values: ScheduleRecipient[] = [
      ...emailRecipients.map((item) => ({
        type: item.type,
        value: item.value,
        deliveryChannel: "email" as const,
      })),
      ...splitValues(externalEmails).map((value) => ({
        type: "email" as const,
        value,
        deliveryChannel: "email" as const,
      })),
      ...groupRecipients.map((item) => ({
        type: "group" as const,
        value: item.value,
        deliveryChannel: "email" as const,
      })),
      ...portalRecipients.map((item) => ({
        type: "user" as const,
        value: item.value,
        deliveryChannel: "portal" as const,
      })),
    ];
    return [
      ...new Map(
        values.map((item) => [`${item.type}:${item.value}:${item.deliveryChannel}`, item]),
      ).values(),
    ];
  }, [emailRecipients, externalEmails, groupRecipients, portalRecipients]);

  const requestBody = (): CreateScheduleRequest | undefined => {
    const report = reportsQuery.data?.find((item) => item.id === reportId);
    if (!report || !selectedReport || !periodSupported || !formatSupported) return undefined;
    return {
      healthBiReportId: report.id,
      reportName: name.trim() || report.name,
      frequency,
      timezone: "Africa/Kampala",
      periodStrategy: selectedPeriod,
      parameters: parameterValues,
      outputFormat: selectedFormat,
      outputConfig: {
        format: selectedFormat,
        deliveryMode: "attachment",
        fileNamePrefix: fileNamePrefix.trim(),
      },
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
    try {
      if (editingId) {
        await updateSchedule({ id: editingId, body }).unwrap();
      } else {
        await createSchedule(body).unwrap();
      }
      onSuccess();
    } catch {
      toast.error(
        "Unable to save the report schedule",
        "Review timing, parameters, health scope, and recipients.",
      );
    }
  };

  const handlePreview = async () => {
    try {
      await previewRecipients(recipients).unwrap();
    } catch {
      toast.error("Unable to resolve recipients", "Check the recipient details and try again.");
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
      <div className="report-scheduler__grid">
        <Select
          id="report-scheduler-report"
          labelText="Health BI report"
          value={reportId}
          onChange={(event) => {
            const next = event.target.value;
            setReportId(next);
            setParameterOverrides({});
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
          value={selectedPeriod}
          onChange={(event) => setPeriodStrategy(event.target.value)}
        >
          <SelectItem value="" text="Select a supported reporting period" />
          {availablePeriods.map(([value, label]) => (
            <SelectItem key={value} value={value} text={label} />
          ))}
        </Select>
        <Select
          id="report-scheduler-format"
          labelText="Output format"
          value={selectedFormat}
          onChange={(event) => setFormat(event.target.value)}
        >
          <SelectItem value="" text="Select an output format" />
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

      {selectedReport && (
        <section className="report-scheduler__context" aria-label="Selected report details">
          <h3>{selectedReport.name}</h3>
          {selectedReport.category && <p>{selectedReport.category}</p>}
          {selectedReport.description && <p>{selectedReport.description}</p>}
        </section>
      )}
      {reportQuery.isFetching && <InlineLoading description="Loading report capabilities…" />}
      {parametersQuery.isFetching && <InlineLoading description="Loading report parameters…" />}
      {(parameterSchema ?? []).length > 0 && (
        <div className="report-scheduler__parameters">
          <h3>Report parameters</h3>
          <p>
            Defaults from the selected report are filled in below. You can adjust them before
            saving.
          </p>
          <div className="report-scheduler__grid">
            {(parameterSchema ?? []).map((parameter) => {
              const options = (parameter.options ?? [])
                .map(optionValue)
                .filter((option): option is { value: string; label: string } => Boolean(option));
              const currentValue = parameterInputValue(parameterValues[parameter.name]);
              if (
                currentValue &&
                options.length > 0 &&
                !options.some((option) => option.value === currentValue)
              ) {
                options.push({ value: currentValue, label: currentValue });
              }
              const label = `${parameter.label || parameter.name}${parameter.required ? " *" : ""}`;
              if (options.length > 0)
                return (
                  <Select
                    key={parameter.name}
                    id={`report-parameter-${parameter.name}`}
                    labelText={label}
                    required={parameter.required}
                    value={currentValue}
                    onChange={(event) =>
                      setParameterOverrides((current) => ({
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
                  required={parameter.required}
                  helperText={parameter.description}
                  value={parameterInputValue(parameterValues[parameter.name])}
                  onChange={(event) =>
                    setParameterOverrides((current) => ({
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

      {reportId && parameterSchema?.length === 0 && !parametersQuery.isFetching && (
        <p>
          This report has no additional parameters. Reporting period and health scope are applied
          when it runs.
        </p>
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
          <RecipientSelect
            kind="email"
            label="Email recipients"
            selected={emailRecipients}
            onChange={(items) => {
              previewState.reset();
              setEmailRecipients(items);
            }}
            disabled={createState.isLoading || updateState.isLoading}
          />
          <RecipientSelect
            kind="group"
            label="Recipient groups"
            selected={groupRecipients}
            onChange={(items) => {
              previewState.reset();
              setGroupRecipients(items);
            }}
            disabled={createState.isLoading || updateState.isLoading}
          />
          <RecipientSelect
            kind="portal"
            label="Portal recipients"
            selected={portalRecipients}
            onChange={(items) => {
              previewState.reset();
              setPortalRecipients(items);
            }}
            disabled={createState.isLoading || updateState.isLoading}
          />
          <TextInput
            id="report-external-email-recipients"
            type="email"
            multiple
            labelText="Additional email addresses"
            helperText="Optional addresses outside the portal, separated by commas"
            value={externalEmails}
            onChange={(event) => {
              previewState.reset();
              setExternalEmails(event.target.value);
            }}
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
                previewState.reset();
                if (!portalRecipients.some((item) => item.value === currentUserId))
                  setPortalRecipients([
                    ...portalRecipients,
                    {
                      id: `user:${currentUserId}`,
                      type: "user",
                      value: currentUserId,
                      label: "Me",
                    },
                  ]);
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
            !selectedReport ||
            reportQuery.isFetching ||
            reportQuery.isError ||
            parametersQuery.isFetching ||
            parametersQuery.isError ||
            !periodSupported ||
            !formatSupported ||
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
