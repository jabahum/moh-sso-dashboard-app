import { baseApi } from "@moh-sso/api";
import type {
  CreateScheduleRequest,
  HealthBIParameter,
  HealthBIReport,
  PortalReport,
  RecipientPreview,
  ReportSchedule,
  ReportSchedulerModule,
  ScheduleRecipient,
} from "../types";

type Envelope<T> = { data: T };

export const reportSchedulerApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    getReportSchedulerModule: builder.query<ReportSchedulerModule, void>({
      query: () => "/report-scheduler",
      transformResponse: (response: Envelope<ReportSchedulerModule>) => response.data,
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
    createReportSchedule: builder.mutation<ReportSchedule, CreateScheduleRequest>({
      query: (body) => ({ url: "/report-scheduler/schedules", method: "POST", body }),
      transformResponse: (response: Envelope<ReportSchedule>) => response.data,
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
  useGetHealthBIReportsQuery,
  useGetHealthBIReportParametersQuery,
  useGetReportSchedulesQuery,
  useCreateReportScheduleMutation,
  usePreviewReportRecipientsMutation,
  useGetPortalReportsQuery,
} = reportSchedulerApi;
