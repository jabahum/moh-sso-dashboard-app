import type {
  Announcement,
  AnnouncementAudiencePreview,
  AnnouncementAudiencePreviewRequest,
  AnnouncementAttachment,
  AnnouncementStats,
  CreateAnnouncementRequest,
  ListAnnouncementsParams,
  PublishAnnouncementRequest,
  UpdateAnnouncementRequest,
  UpdateAnnouncementAttachmentRequest,
  UploadAnnouncementAttachmentRequest,
  ScheduleAnnouncementRequest,
  SetAnnouncementPinnedRequest,
  SetAnnouncementPriorityRequest,
} from "../types";
import { API } from "@moh-sso/config";
import { baseApi } from "@moh-sso/api";

const ADMIN_PAGE_SIZE = 200;

type ApiEnvelope<T> = {
  success: boolean;
  data: T;
};

export const announcementApi = baseApi.injectEndpoints({
  endpoints: (builder) => ({
    listAnnouncementsAdmin: builder.query<Announcement[], ListAnnouncementsParams | void>({
      query: (params) => ({
        url: "/admin/announcements",
        params: {
          limit: params?.limit ?? 20,
          offset: params?.offset ?? 0,
        },
      }),
      transformResponse: (response: ApiEnvelope<Announcement[]>) => response.data,
      providesTags: (result) =>
        result
          ? [
              ...result.map((item) => ({
                type: "Announcements" as const,
                id: item.id,
              })),
              { type: "Announcements" as const, id: "LIST" },
            ]
          : [{ type: "Announcements" as const, id: "LIST" }],
    }),

    // Loads every announcement page by page, so the admin table's client-side
    // filters and pagination cover all announcements rather than a fixed first page.
    listAllAnnouncementsAdmin: builder.query<Announcement[], void>({
      async queryFn(_arg, _api, _extraOptions, fetchWithBQ) {
        const all: Announcement[] = [];
        for (let offset = 0; ; offset += ADMIN_PAGE_SIZE) {
          const result = await fetchWithBQ({
            url: "/admin/announcements",
            params: { limit: ADMIN_PAGE_SIZE, offset },
          });
          if (result.error) return { error: result.error };
          const page = (result.data as ApiEnvelope<Announcement[]>).data ?? [];
          all.push(...page);
          if (page.length < ADMIN_PAGE_SIZE) return { data: all };
        }
      },
      providesTags: (result) =>
        result
          ? [
              ...result.map((item) => ({
                type: "Announcements" as const,
                id: item.id,
              })),
              { type: "Announcements" as const, id: "LIST" },
            ]
          : [{ type: "Announcements" as const, id: "LIST" }],
    }),

    getAnnouncementById: builder.query<Announcement, string>({
      query: (id) => ({
        url: `/admin/announcements/${id}`,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      providesTags: (_result, _error, id) => [{ type: "Announcements" as const, id }],
    }),

    getAnnouncementStats: builder.query<AnnouncementStats, void>({
      query: () => ({
        url: "/admin/announcements/stats",
      }),
      transformResponse: (response: ApiEnvelope<AnnouncementStats>) => response.data,
      providesTags: [{ type: "Announcements", id: "STATS" }],
    }),

    previewAnnouncementAudience: builder.mutation<
      AnnouncementAudiencePreview,
      AnnouncementAudiencePreviewRequest
    >({
      query: (body) => ({
        url: "/admin/announcements/audience-preview",
        method: "POST",
        body,
      }),
      transformResponse: (response: ApiEnvelope<AnnouncementAudiencePreview>) =>
        response.data,
    }),

    createAnnouncement: builder.mutation<Announcement, CreateAnnouncementRequest>({
      query: (body) => ({
        url: "/admin/announcements",
        method: "POST",
        body,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: [
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    updateAnnouncement: builder.mutation<
      Announcement,
      { id: string; body: UpdateAnnouncementRequest }
    >({
      query: ({ id, body }) => ({
        url: `/admin/announcements/${id}`,
        method: "PUT",
        body,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, { id }) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    deleteAnnouncement: builder.mutation<{ message: string }, string>({
      query: (id) => ({
        url: `/admin/announcements/${id}`,
        method: "DELETE",
      }),
      transformResponse: (response: ApiEnvelope<{ message: string }>) => response.data,
      invalidatesTags: (_result, _error, id) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    restoreAnnouncement: builder.mutation<Announcement, string>({
      query: (id) => ({
        url: `/admin/announcements/${id}/restore`,
        method: "POST",
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, id) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
      ],
    }),

    publishAnnouncement: builder.mutation<
      Announcement,
      string | { id: string; body?: PublishAnnouncementRequest }
    >({
      query: (arg) => {
        const id = typeof arg === "string" ? arg : arg.id;
        const body = typeof arg === "string" ? undefined : arg.body;

        return {
          url: `/admin/announcements/${id}/publish`,
          method: "POST",
          ...(body ? { body } : {}),
        };
      },
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, arg) => {
        const id = typeof arg === "string" ? arg : arg.id;
        return [
          { type: "Announcements", id },
          { type: "Announcements", id: "LIST" },
          { type: "Announcements", id: "STATS" },
          { type: "Announcements", id: "MY_LIST" },
        ];
      },
    }),

    draftAnnouncement: builder.mutation<Announcement, string>({
      query: (id) => ({
        url: `/admin/announcements/${id}/draft`,
        method: "POST",
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, id) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
      ],
    }),

    scheduleAnnouncement: builder.mutation<
      Announcement,
      { id: string; body: ScheduleAnnouncementRequest }
    >({
      query: ({ id, body }) => ({
        url: `/admin/announcements/${id}/schedule`,
        method: "POST",
        body,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, { id }) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
      ],
    }),

    archiveAnnouncement: builder.mutation<Announcement, string>({
      query: (id) => ({
        url: `/admin/announcements/${id}/archive`,
        method: "POST",
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, id) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "STATS" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    setAnnouncementPinned: builder.mutation<
      Announcement,
      { id: string; body: SetAnnouncementPinnedRequest }
    >({
      query: ({ id, body }) => ({
        url: `/admin/announcements/${id}/pin`,
        method: "PATCH",
        body,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, { id }) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    setAnnouncementPriority: builder.mutation<
      Announcement,
      { id: string; body: SetAnnouncementPriorityRequest }
    >({
      query: ({ id, body }) => ({
        url: `/admin/announcements/${id}/priority`,
        method: "PATCH",
        body,
      }),
      transformResponse: (response: ApiEnvelope<Announcement>) => response.data,
      invalidatesTags: (_result, _error, { id }) => [
        { type: "Announcements", id },
        { type: "Announcements", id: "LIST" },
        { type: "Announcements", id: "MY_LIST" },
      ],
    }),

    listAnnouncementAttachments: builder.query<AnnouncementAttachment[], string>({
      query: (announcementId) => ({
        url: `/admin/announcements/${announcementId}/attachments`,
      }),
      transformResponse: (response: ApiEnvelope<AnnouncementAttachment[]>) => response.data,
      providesTags: (_result, _error, announcementId) => [
        { type: "Announcements" as const, id: announcementId },
      ],
    }),

    uploadAnnouncementAttachment: builder.mutation<
      AnnouncementAttachment,
      UploadAnnouncementAttachmentRequest
    >({
      query: ({ announcementId, file, include_in_email = true, inline = false, content_id, sort_order }) => {
        const body = new FormData();
        body.append("file", file);
        body.append("include_in_email", String(include_in_email));
        body.append("inline", String(inline));
        if (content_id) {
          body.append("content_id", content_id);
        }
        if (typeof sort_order === "number") {
          body.append("sort_order", String(sort_order));
        }

        return {
          url: `/admin/announcements/${announcementId}/attachments`,
          method: "POST",
          body,
        };
      },
      transformResponse: (response: ApiEnvelope<AnnouncementAttachment>) => response.data,
      invalidatesTags: (_result, _error, { announcementId }) => [
        { type: "Announcements", id: announcementId },
        { type: "Announcements", id: "LIST" },
      ],
    }),

    updateAnnouncementAttachment: builder.mutation<
      AnnouncementAttachment,
      UpdateAnnouncementAttachmentRequest
    >({
      query: ({ announcementId, attachmentId, ...body }) => ({
        url: `/admin/announcements/${announcementId}/attachments/${attachmentId}`,
        method: "PATCH",
        body,
      }),
      transformResponse: (response: ApiEnvelope<AnnouncementAttachment>) => response.data,
      invalidatesTags: (_result, _error, { announcementId }) => [
        { type: "Announcements", id: announcementId },
      ],
    }),

    deleteAnnouncementAttachment: builder.mutation<
      { message: string },
      { announcementId: string; attachmentId: string }
    >({
      query: ({ announcementId, attachmentId }) => ({
        url: `/admin/announcements/${announcementId}/attachments/${attachmentId}`,
        method: "DELETE",
      }),
      transformResponse: (response: ApiEnvelope<{ message: string }>) => response.data,
      invalidatesTags: (_result, _error, { announcementId }) => [
        { type: "Announcements", id: announcementId },
        { type: "Announcements", id: "LIST" },
      ],
    }),

    listMyAnnouncements: builder.query<Announcement[], ListAnnouncementsParams | void>({
      query: (params) => ({
        url: "/announcements/me",
        params: {
          limit: params?.limit ?? 20,
          offset: params?.offset ?? 0,
          role: params?.role,
          client_id: params?.clientId,
        },
      }),
      transformResponse: (response: ApiEnvelope<Announcement[]>) => response.data,
      providesTags: (result) =>
        result
          ? [
              ...result.map((item) => ({
                type: "Announcements" as const,
                id: item.id,
              })),
              { type: "Announcements" as const, id: "MY_LIST" },
            ]
          : [{ type: "Announcements" as const, id: "MY_LIST" }],
    }),

    listPublicAnnouncements: builder.query<Announcement[], ListAnnouncementsParams>({
      query: ({ limit = 20, offset = 0 }) => ({
        url: "/announcements/public",
        params: { limit, offset },
      }),
      transformResponse: (response: ApiEnvelope<Announcement[]>) => response.data,
      providesTags: (result) =>
        result
          ? [
              ...result.map((item) => ({
                type: "Announcements" as const,
                id: item.id,
              })),
              { type: "Announcements" as const, id: "MY_LIST" },
            ]
          : [{ type: "Announcements" as const, id: "MY_LIST" }],
    }),
  }),
  overrideExisting: false,
});

export function buildAnnouncementAttachmentDownloadUrl(
  announcementId: string,
  attachmentId: string,
) {
  return `${API.base}/admin/announcements/${announcementId}/attachments/${attachmentId}/download`;
}

export function buildUserAnnouncementAttachmentDownloadUrl(
  announcementId: string,
  attachmentId: string,
) {
  return `${API.base}/announcements/${announcementId}/attachments/${attachmentId}/download`;
}

export const {
  useListAnnouncementsAdminQuery,
  useListAllAnnouncementsAdminQuery,
  useGetAnnouncementByIdQuery,
  useGetAnnouncementStatsQuery,
  usePreviewAnnouncementAudienceMutation,
  useCreateAnnouncementMutation,
  useUpdateAnnouncementMutation,
  useDeleteAnnouncementMutation,
  useRestoreAnnouncementMutation,
  usePublishAnnouncementMutation,
  useScheduleAnnouncementMutation,
  useDraftAnnouncementMutation,
  useArchiveAnnouncementMutation,
  useSetAnnouncementPinnedMutation,
  useSetAnnouncementPriorityMutation,
  useListAnnouncementAttachmentsQuery,
  useUploadAnnouncementAttachmentMutation,
  useUpdateAnnouncementAttachmentMutation,
  useDeleteAnnouncementAttachmentMutation,
  useListMyAnnouncementsQuery,
  useListPublicAnnouncementsQuery,
} = announcementApi;
