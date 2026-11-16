import { baseApi } from "@moh-sso/api";
import type { UpsertRbacSystemPayload, RbacSystemDetail } from "@moh-sso/rbac/types";

export type ManifestLaunch = {
  mode: "external" | "internal" | "microfrontend";
  url?: string;
  route?: string;
  module?: string;
  appName?: string;
  entryUrl?: string;
};

export type ApplicationManifest = {
  apiVersion: "portal.moh.go.ug/v1alpha1";
  kind: "Application";
  metadata: { id: string; name: string; version: string; description?: string };
  spec: {
    environment: string;
    owner: { team: string; name: string; email: string };
    supportUrl: string;
    icon?: string;
    category?: string;
    launch: ManifestLaunch;
    access: { roles: string[] };
    navigation?: {
      id: string;
      label: string;
      path: string;
      icon?: string;
      requiredPermissions?: string[];
    }[];
  };
};

export type ApplicationManifestPreview = {
  manifest: ApplicationManifest;
  operation: "create" | "update" | "noop";
  before?: RbacSystemDetail;
  after: UpsertRbacSystemPayload & { clientId: string };
  rolesToCreate: string[];
  accessRoles: string[];
  warnings: string[];
  reviewToken: string;
};

export type ApplicationManifestRuntime = {
  id: string;
  name: string;
  version: string;
  launch: ManifestLaunch;
};

type Envelope<T> = { data: T };
const base = "/admin/rbac/application-manifests";
const api = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    previewApplicationManifest: builder.mutation<ApplicationManifestPreview, string>({
      query: (document) => ({ url: `${base}/preview`, method: "POST", body: { document } }),
      transformResponse: (response: Envelope<ApplicationManifestPreview>) => response.data,
    }),
    importApplicationManifest: builder.mutation<
      ApplicationManifestPreview,
      { document: string; reviewToken: string }
    >({
      query: (body) => ({ url: `${base}/import`, method: "POST", body }),
      transformResponse: (response: Envelope<ApplicationManifestPreview>) => response.data,
      invalidatesTags: (result) =>
        result
          ? [
              { type: "RbacSystem", id: "LIST" },
              { type: "RbacSystem", id: result.manifest.metadata.id },
            ]
          : [],
    }),
    getApplicationManifestRuntime: builder.query<ApplicationManifestRuntime, string>({
      query: (id) => ({ url: `/applications/${encodeURIComponent(id)}/manifest` }),
      transformResponse: (response: Envelope<ApplicationManifestRuntime>) => response.data,
      providesTags: (_result, _error, id) => [{ type: "RbacSystem", id }],
    }),
  }),
});

export const {
  usePreviewApplicationManifestMutation,
  useImportApplicationManifestMutation,
  useGetApplicationManifestRuntimeQuery,
} = api;
