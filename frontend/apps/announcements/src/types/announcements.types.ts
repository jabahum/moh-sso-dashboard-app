export type AnnouncementLevel = "INFO" | "SUCCESS" | "WARNING" | "CRITICAL";

export type AnnouncementStatus = "DRAFT" | "SCHEDULED" | "PUBLISHED" | "ARCHIVED";

export type AnnouncementAudienceType =
  | "ALL_USERS"
  | "ADMINS_ONLY"
  | "SPECIFIC_CLIENTS"
  | "SPECIFIC_ROLES"
  | "SPECIFIC_USERS"
  | "SPECIFIC_GROUPS"
  | "SPECIFIC_HEALTH_CONTEXTS"
  | "HEALTH_CONTEXT_AND_DESCENDANTS";

export interface Announcement {
  id: string;
  title: string;
  message: string;
  summary: string | null;
  level: AnnouncementLevel;
  tag: string | null;
  link_url: string | null;
  link_label: string | null;
  priority: number;
  is_pinned: boolean;
  status: AnnouncementStatus;
  publish_at: string | null;
  expires_at: string | null;
  audience_type: AnnouncementAudienceType;
  client_ids?: string[];
  role_names?: string[];
  user_ids?: string[];
  group_ids?: string[];
  health_context_ids?: string[];
  include_context_descendants?: boolean;
  notify_by_email: boolean;
  email_notification_sent_at: string | null;
  notify_by_sms: boolean;
  sms_message: string | null;
  sms_notification_queued_at: string | null;
  created_by: string | null;
  updated_by: string | null;
  published_by: string | null;
  archived_by: string | null;
  created_at: string;
  updated_at: string;
  published_at: string | null;
  archived_at: string | null;
  deleted_at: string | null;
  deleted_by: string | null;
  version: number;
  attachments?: AnnouncementAttachment[];
  attachment_count?: number;
}

export interface ListAnnouncementsParams {
  limit?: number;
  offset?: number;
  role?: string;
  clientId?: string;
}

export interface CreateAnnouncementRequest {
  title: string;
  message: string;
  summary?: string | null;
  level: AnnouncementLevel;
  tag?: string | null;
  link_url?: string | null;
  link_label?: string | null;
  priority?: number;
  is_pinned?: boolean;
  status?: AnnouncementStatus;
  publish_at?: string | null;
  expires_at?: string | null;
  audience_type?: AnnouncementAudienceType;
  client_ids?: string[];
  role_names?: string[];
  user_ids?: string[];
  group_ids?: string[];
  health_context_ids?: string[];
  notify_by_email?: boolean;
  notify_by_sms?: boolean;
  sms_message?: string | null;
}

export interface AnnouncementEmailAttachment {
  file_name: string;
  content_type?: string;
  data_base64?: string;
  content_id?: string;
  inline?: boolean;
}

export interface AnnouncementAttachment {
  id: string;
  announcement_id: string;
  file_name: string;
  original_file_name: string;
  content_type?: string | null;
  file_size: number;
  storage_provider: string;
  checksum?: string | null;
  uploaded_by?: string | null;
  include_in_email: boolean;
  inline: boolean;
  content_id?: string | null;
  sort_order: number;
  created_at: string;
  deleted_at?: string | null;
  deleted_by?: string | null;
  download_url?: string;
}

export interface UploadAnnouncementAttachmentRequest {
  announcementId: string;
  file: File;
  include_in_email?: boolean;
  inline?: boolean;
  content_id?: string;
  sort_order?: number;
}

export interface UpdateAnnouncementAttachmentRequest {
  announcementId: string;
  attachmentId: string;
  include_in_email?: boolean;
  inline?: boolean;
  content_id?: string | null;
  sort_order?: number;
}

export interface PublishAnnouncementRequest {
  attachments?: AnnouncementEmailAttachment[];
  include_attachments_in_email?: boolean;
}

export interface UpdateAnnouncementRequest {
  title: string;
  message: string;
  summary?: string | null;
  level: AnnouncementLevel;
  tag?: string | null;
  link_url?: string | null;
  link_label?: string | null;
  priority?: number;
  is_pinned?: boolean;
  status?: AnnouncementStatus;
  publish_at?: string | null;
  expires_at?: string | null;
  audience_type?: AnnouncementAudienceType;
  client_ids?: string[];
  role_names?: string[];
  user_ids?: string[];
  group_ids?: string[];
  health_context_ids?: string[];

  /**
   * User-controlled email notification preference.
   * Email is only sent/queued on publish if this is true.
   */
  notify_by_email?: boolean;
  notify_by_sms?: boolean;
  sms_message?: string | null;
}

export interface ScheduleAnnouncementRequest {
  publish_at: string;
  attachments?: AnnouncementEmailAttachment[];
  include_attachments_in_email?: boolean;
}

export interface SetAnnouncementPinnedRequest {
  is_pinned: boolean;
}

export interface SetAnnouncementPriorityRequest {
  priority: number;
}

export interface AnnouncementAudiencePreviewRequest {
  audience_type: Extract<
    AnnouncementAudienceType,
    "SPECIFIC_HEALTH_CONTEXTS" | "HEALTH_CONTEXT_AND_DESCENDANTS"
  >;
  health_context_ids: string[];
}

export interface AnnouncementAudiencePreview {
  recipient_count: number;
}

export interface AnnouncementStats {
  total: number;
  draft_count: number;
  scheduled_count: number;
  published_count: number;
  archived_count: number;
  active_count: number;
}
