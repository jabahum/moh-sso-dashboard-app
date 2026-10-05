import { baseApi } from "@moh-sso/api";
import type { ReportSchedulerModule } from "../types";

export const reportSchedulerApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    getReportSchedulerModule: builder.query<ReportSchedulerModule, void>({
      query: () => "/report-scheduler",
      transformResponse: (response: { data: ReportSchedulerModule }) => response.data,
    }),
  }),
});

export const { useGetReportSchedulerModuleQuery } = reportSchedulerApi;
