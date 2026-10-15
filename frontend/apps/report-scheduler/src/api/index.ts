import { baseApi } from "@moh-sso/api";
import type {
  CreateScheduleRequest,
  ExecutionDetail,
  HealthBIParameter,
  HealthBIReport,
  PortalReport,
  RecipientPreview,
  RecipientOption,
  ReportExecution,
  ReportSchedule,
  ReportSchedulerModule,
  SchedulerOverview,
  ScheduleRecipient,
} from "../types";

type Envelope<T> = { data: T };
type ScheduleListArgs = { limit?: number; search?: string; enabled?: boolean };
type ExecutionListArgs = { limit?: number; search?: string; status?: string };

export const reportSchedulerApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    getSchedulerRecipientOptions: builder.query<
      RecipientOption[],
      { kind: "email" | "group" | "portal"; search?: string; selected?: string }
    >({
      query: (params) => ({ url: "/report-scheduler/recipients/options", params }),
      transformResponse: (response: Envelope<RecipientOption[]>) => response.data,
    }),
    getReportSchedulerModule: builder.query<ReportSchedulerModule, void>({
      query: () => "/report-scheduler",
      transformResponse: (response: Envelope<ReportSchedulerModule>) => response.data,
    }),
    getReportSchedulerOverview: builder.query<SchedulerOverview, void>({
      query: () => "/report-scheduler/overview",
      transformResponse: (response: Envelope<SchedulerOverview>) => response.data,
    }),
    getHealthBIReports: builder.query<HealthBIReport[], void>({
      query: () => "/report-scheduler/reports",
      transformResponse: (response: Envelope<HealthBIReport[]>) => response.data,
    }),
    getHealthBIReportParameters: builder.query<HealthBIParameter[], string>({
      query: (reportId) => ({ url: "/report-scheduler/report-parameters", params: { reportId } }),
      transformResponse: (response: Envelope<HealthBIParameter[]>) => response.data,
    }),
    getHealthBIReport: builder.query<HealthBIReport, string>({
      query: (reportId) => ({ url: "/report-scheduler/report", params: { reportId } }),
      transformResponse: (response: Envelope<HealthBIReport>) => response.data,
    }),
    getReportSchedules: builder.query<ReportSchedule[], ScheduleListArgs | void>({
      query: (args) => ({ url: "/report-scheduler/schedules", params: args ?? undefined }),
      transformResponse: (response: Envelope<ReportSchedule[]>) => response.data,
    }),
    getReportExecutions: builder.query<ReportExecution[], ExecutionListArgs | void>({
      query: (args) => ({ url: "/report-scheduler/executions", params: args ?? undefined }),
      transformResponse: (response: Envelope<ReportExecution[]>) => response.data,
    }),
    getReportExecution: builder.query<ExecutionDetail, string>({
      query: (id) => `/report-scheduler/executions/${id}`,
      transformResponse: (response: Envelope<ExecutionDetail>) => response.data,
    }),
    retryReportExecution: builder.mutation<ReportExecution, string>({
      query: (id) => ({ url: `/report-scheduler/executions/${id}/retry`, method: "POST" }),
      transformResponse: (response: Envelope<ReportExecution>) => response.data,
    }),
    createReportSchedule: builder.mutation<ReportSchedule, CreateScheduleRequest>({
      query: (body) => ({ url: "/report-scheduler/schedules", method: "POST", body }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
    }),
    updateReportSchedule: builder.mutation<
      ReportSchedule,
      { id: string; body: CreateScheduleRequest }
    >({
      query: ({ id, body }) => ({ url: `/report-scheduler/schedules/${id}`, method: "PUT", body }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
    }),
    deleteReportSchedule: builder.mutation<{ deleted: boolean }, string>({
      query: (id) => ({ url: `/report-scheduler/schedules/${id}`, method: "DELETE" }),
      transformResponse: (response: Envelope<{ deleted: boolean }>) => response.data,
    }),
    pauseReportSchedule: builder.mutation<ReportSchedule, string>({
      query: (id) => ({ url: `/report-scheduler/schedules/${id}/pause`, method: "POST" }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
    }),
    resumeReportSchedule: builder.mutation<ReportSchedule, string>({
      query: (id) => ({ url: `/report-scheduler/schedules/${id}/resume`, method: "POST" }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
    }),
    duplicateReportSchedule: builder.mutation<ReportSchedule, string>({
      query: (id) => ({ url: `/report-scheduler/schedules/${id}/duplicate`, method: "POST" }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
    }),
    runReportScheduleNow: builder.mutation<ReportExecution, string>({
      query: (id) => ({ url: `/report-scheduler/schedules/${id}/run`, method: "POST" }),
      transformResponse: (response: Envelope<ReportExecution>) => response.data,
    }),
    previewReportRecipients: builder.mutation<RecipientPreview, ScheduleRecipient[]>({
      query: (recipients) => ({
        url: "/report-scheduler/recipients/preview",
        method: "POST",
        body: { recipients },
      }),
      transformResponse: (response: Envelope<RecipientPreview>) => response.data,
    }),
    getPortalReports: builder.query<PortalReport[], number | void>({
      query: (limit) => ({
        url: "/report-scheduler/portal-reports",
        params: limit ? { limit } : undefined,
      }),
      transformResponse: (response: Envelope<PortalReport[]>) => response.data,
    }),
  }),
});

export const {
  useGetSchedulerRecipientOptionsQuery,
  useGetReportSchedulerModuleQuery,
  useGetReportSchedulerOverviewQuery,
  useGetHealthBIReportsQuery,
  useGetHealthBIReportQuery,
  useGetHealthBIReportParametersQuery,
  useGetReportSchedulesQuery,
  useGetReportExecutionsQuery,
  useGetReportExecutionQuery,
  useRetryReportExecutionMutation,
  useCreateReportScheduleMutation,
  useUpdateReportScheduleMutation,
  useDeleteReportScheduleMutation,
  usePauseReportScheduleMutation,
  useResumeReportScheduleMutation,
  useDuplicateReportScheduleMutation,
  useRunReportScheduleNowMutation,
  usePreviewReportRecipientsMutation,
  useGetPortalReportsQuery,
} = reportSchedulerApi;
