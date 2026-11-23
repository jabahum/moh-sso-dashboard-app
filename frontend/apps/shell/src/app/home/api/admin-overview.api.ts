import { API } from "@moh-sso/config";
import { baseApi } from "@moh-sso/api";

/**
 * Lightweight summaries for the Admin Overview signal tiles.
 *
 * These reuse the same endpoints the users, clients and audit microfrontends
 * already call, but live in the shell so the overview does not have to pull
 * those microfrontend bundles in eagerly. They share cache tags with the
 * microfrontend queries, so creating or toggling a user/client refreshes them.
 */

type ApiEnvelope<T> = {
  success: boolean;
  data: T;
};

type EnabledRecord = {
  enabled?: boolean;
  isActive?: boolean;
};

export type EnabledSummary = {
  total: number;
  disabled: number;
};

type AuditOverviewSummary = {
  failed_logins: number;
};

const summarise = (records: EnabledRecord[] | null | undefined): EnabledSummary => {
  const list = Array.isArray(records) ? records : [];
  return {
    total: list.length,
    disabled: list.filter((record) => (record.enabled ?? record.isActive ?? true) === false).length,
  };
};

export const adminOverviewApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    getAdminOverviewUsers: builder.query<EnabledSummary, void>({
      query: () => ({ url: API.admin.users.list(), credentials: "include" }),
      transformResponse: (res: ApiEnvelope<EnabledRecord[]>) => summarise(res?.data),
      providesTags: [{ type: "User", id: "LIST" }],
    }),
    getAdminOverviewClients: builder.query<EnabledSummary, void>({
      query: () => ({ url: API.clients.list(), credentials: "include" }),
      transformResponse: (res: ApiEnvelope<EnabledRecord[]>) => summarise(res?.data),
      providesTags: [{ type: "Client", id: "LIST" }],
    }),
    getAdminOverviewFailedLogins: builder.query<number, { from: string; to: string }>({
      query: ({ from, to }) => ({
        url: API.admin.audit.metrics.overview(),
        params: { from, to },
        credentials: "include",
      }),
      transformResponse: (res: ApiEnvelope<AuditOverviewSummary>) =>
        Number(res?.data?.failed_logins ?? 0),
      providesTags: ["Audit"],
    }),
  }),
});

export const {
  useGetAdminOverviewUsersQuery,
  useGetAdminOverviewClientsQuery,
  useGetAdminOverviewFailedLoginsQuery,
} = adminOverviewApi;
