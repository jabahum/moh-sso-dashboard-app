import { baseApi } from "@moh-sso/api";

import type {
  IdentitySyncDescriptor,
  IdentitySyncIntegration,
  IdentitySyncIntegrationPayload,
  IdentitySyncJob,
  IdentitySyncRevocation,
  IdentitySyncRun,
  IdentitySyncRunDetail,
  IdentitySyncRow,
  ServiceIdentity,
} from "../types";

type ApiEnvelope<T> = {
  success: boolean;
  data: T;
};

const identitiesBase = (clientId: string) =>
  `/admin/platform/applications/${encodeURIComponent(clientId)}/identities`;

const base = (clientId: string) =>
  `/admin/platform/applications/${encodeURIComponent(clientId)}/identity-sync`;

export const identitySyncApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    getIdentitySyncIntegration: builder.query<IdentitySyncIntegration, string>({
      query: (clientId) => base(clientId),
      transformResponse: (res: ApiEnvelope<IdentitySyncIntegration>) => res.data,
      providesTags: (_r, _e, clientId) => [{ type: "IdentitySync", id: `integration:${clientId}` }],
    }),
    saveIdentitySyncIntegration: builder.mutation<
      IdentitySyncIntegration,
      { clientId: string; data: IdentitySyncIntegrationPayload }
    >({
      query: ({ clientId, data }) => ({ url: base(clientId), method: "PUT", body: data }),
      transformResponse: (res: ApiEnvelope<IdentitySyncIntegration>) => res.data,
      invalidatesTags: (_r, _e, { clientId }) => [
        { type: "IdentitySync", id: `integration:${clientId}` },
      ],
    }),
    listServiceIdentities: builder.query<ServiceIdentity[], string>({
      query: (clientId) => identitiesBase(clientId),
      transformResponse: (res: ApiEnvelope<{ identities: ServiceIdentity[] }>) =>
        res.data.identities,
      providesTags: (_r, _e, clientId) => [{ type: "IdentitySync", id: `identities:${clientId}` }],
    }),
    resolveServiceAccount: builder.mutation<
      { clientId: string; subject: string },
      { clientId: string; oauthClientId: string }
    >({
      query: ({ clientId, oauthClientId }) => ({
        url: `${identitiesBase(clientId)}/${encodeURIComponent(oauthClientId)}/service-account`,
        method: "GET",
      }),
      transformResponse: (res: ApiEnvelope<{ clientId: string; subject: string }>) => res.data,
    }),
    saveServiceIdentity: builder.mutation<
      ServiceIdentity,
      {
        clientId: string;
        oauthClientId: string;
        data: { subject: string; scopes: string[]; enabled: boolean; reason: string };
      }
    >({
      query: ({ clientId, oauthClientId, data }) => ({
        url: `${identitiesBase(clientId)}/${encodeURIComponent(oauthClientId)}`,
        method: "PUT",
        body: data,
      }),
      transformResponse: (res: ApiEnvelope<ServiceIdentity>) => res.data,
      invalidatesTags: (_r, _e, { clientId }) => [
        { type: "IdentitySync", id: `identities:${clientId}` },
      ],
    }),
    describeIdentitySync: builder.mutation<IdentitySyncDescriptor, string>({
      query: (clientId) => ({ url: `${base(clientId)}/describe`, method: "GET" }),
      transformResponse: (res: ApiEnvelope<IdentitySyncDescriptor>) => res.data,
    }),
    listIdentitySyncRuns: builder.query<
      { runs: IdentitySyncRun[]; jobs: IdentitySyncJob[] },
      string
    >({
      query: (clientId) => `${base(clientId)}/runs`,
      transformResponse: (res: ApiEnvelope<{ runs: IdentitySyncRun[]; jobs: IdentitySyncJob[] }>) =>
        res.data,
      providesTags: (_r, _e, clientId) => [{ type: "IdentitySync", id: `runs:${clientId}` }],
    }),
    startIdentitySyncRun: builder.mutation<IdentitySyncJob, string>({
      query: (clientId) => ({ url: `${base(clientId)}/runs`, method: "POST" }),
      transformResponse: (res: ApiEnvelope<IdentitySyncJob>) => res.data,
      invalidatesTags: (_r, _e, clientId) => [{ type: "IdentitySync", id: `runs:${clientId}` }],
    }),
    getIdentitySyncRun: builder.query<IdentitySyncRunDetail, { clientId: string; runId: string }>({
      query: ({ clientId, runId }) => `${base(clientId)}/runs/${encodeURIComponent(runId)}`,
      transformResponse: (res: ApiEnvelope<IdentitySyncRunDetail>) => res.data,
      providesTags: (_r, _e, { runId }) => [{ type: "IdentitySync", id: `run:${runId}` }],
    }),
    decideIdentitySyncRows: builder.mutation<
      { rows: IdentitySyncRow[] },
      {
        clientId: string;
        runId: string;
        decision: "approve" | "reject";
        accountIds: string[];
        note: string;
      }
    >({
      query: ({ clientId, runId, decision, accountIds, note }) => ({
        url: `${base(clientId)}/runs/${encodeURIComponent(runId)}/decisions`,
        method: "POST",
        body: { decision, accountIds, note },
      }),
      transformResponse: (res: ApiEnvelope<{ rows: IdentitySyncRow[] }>) => res.data,
      invalidatesTags: (_r, _e, { runId }) => [{ type: "IdentitySync", id: `run:${runId}` }],
    }),
    applyIdentitySyncRun: builder.mutation<
      IdentitySyncJob,
      { clientId: string; runId: string; resume: boolean }
    >({
      query: ({ clientId, runId, resume }) => ({
        url: `${base(clientId)}/runs/${encodeURIComponent(runId)}/apply`,
        method: "POST",
        body: { resume },
      }),
      transformResponse: (res: ApiEnvelope<IdentitySyncJob>) => res.data,
      invalidatesTags: (_r, _e, { clientId, runId }) => [
        { type: "IdentitySync", id: `runs:${clientId}` },
        { type: "IdentitySync", id: `run:${runId}` },
      ],
    }),
    listIdentitySyncRevocations: builder.query<
      { revocations: IdentitySyncRevocation[]; jobs: IdentitySyncJob[] },
      { clientId: string; all: boolean }
    >({
      query: ({ clientId, all }) => `${base(clientId)}/revocations?all=${all ? "true" : "false"}`,
      transformResponse: (
        res: ApiEnvelope<{ revocations: IdentitySyncRevocation[]; jobs: IdentitySyncJob[] }>,
      ) => res.data,
      providesTags: (_r, _e, { clientId }) => [
        { type: "IdentitySync", id: `revocations:${clientId}` },
      ],
    }),
    resumeIdentitySyncRevocations: builder.mutation<IdentitySyncJob, string>({
      query: (clientId) => ({ url: `${base(clientId)}/revocations/resume`, method: "POST" }),
      transformResponse: (res: ApiEnvelope<IdentitySyncJob>) => res.data,
      invalidatesTags: (_r, _e, clientId) => [
        { type: "IdentitySync", id: `revocations:${clientId}` },
      ],
    }),
    restoreIdentitySyncRevocation: builder.mutation<
      IdentitySyncRevocation,
      { clientId: string; subject: string; reason: string }
    >({
      query: ({ clientId, subject, reason }) => ({
        url: `${base(clientId)}/revocations/restore`,
        method: "POST",
        body: { subject, reason },
      }),
      transformResponse: (res: ApiEnvelope<IdentitySyncRevocation>) => res.data,
      invalidatesTags: (_r, _e, { clientId }) => [
        { type: "IdentitySync", id: `revocations:${clientId}` },
      ],
    }),
  }),
});

export const {
  useGetIdentitySyncIntegrationQuery,
  useSaveIdentitySyncIntegrationMutation,
  useDescribeIdentitySyncMutation,
  useListServiceIdentitiesQuery,
  useResolveServiceAccountMutation,
  useSaveServiceIdentityMutation,
  useListIdentitySyncRunsQuery,
  useStartIdentitySyncRunMutation,
  useGetIdentitySyncRunQuery,
  useDecideIdentitySyncRowsMutation,
  useApplyIdentitySyncRunMutation,
  useListIdentitySyncRevocationsQuery,
  useResumeIdentitySyncRevocationsMutation,
  useRestoreIdentitySyncRevocationMutation,
} = identitySyncApi;
