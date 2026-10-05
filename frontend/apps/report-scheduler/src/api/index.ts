import { baseApi } from "@moh-sso/api";
import type {
  CreateScheduleRequest,
  ExecutionDetail,
  HealthBIParameter,
  HealthBIReport,
  PortalReport,
  RecipientPreview,
  ReportExecution,
  ReportSchedule,
  ReportSchedulerModule,
  SchedulerOverview,
  ScheduleRecipient,
} from "../types";

type Envelope<T> = { data: T };

export const reportSchedulerApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
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
      query: (reportId) => `/report-scheduler/reports/${encodeURIComponent(reportId)}/parameters`,
      transformResponse: (response: Envelope<HealthBIParameter[]>) => response.data,
    }),
    getReportSchedules: builder.query<ReportSchedule[], void>({
      query: () => "/report-scheduler/schedules",
      transformResponse: (response: Envelope<ReportSchedule[]>) => response.data,
    }),
    getReportExecutions: builder.query<ReportExecution[], void>({
      query: () => "/report-scheduler/executions",
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
    updateReportSchedule: builder.mutation<ReportSchedule, { id: string; body: CreateScheduleRequest }>({
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
    getPortalReports: builder.query<PortalReport[], void>({
      query: () => "/report-scheduler/portal-reports",
      transformResponse: (response: Envelope<PortalReport[]>) => response.data,
    }),
  }),
});

export const {
  useGetReportSchedulerModuleQuery,
  useGetReportSchedulerOverviewQuery,
  useGetHealthBIReportsQuery,
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
