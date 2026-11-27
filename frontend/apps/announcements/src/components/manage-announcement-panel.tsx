import { Link, Stack, Tag, Tile } from "@carbon/react";
import { Attachment, Link as LinkIcon } from "@carbon/react/icons";

import type {
  Announcement,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
} from "../types";

import { useCreateAnnouncementMutation, useUpdateAnnouncementMutation } from "../api";

import { AnnouncementForm } from "./announcements-form.component";
import { AnnouncementAttachments } from "./announcement-attachments.component";
import { useToast } from "@moh-sso/ui";
import "./announcements.components.scss";

function apiErrorMessage(error: unknown): string | null {
  const data = (error as { data?: { message?: unknown; error?: { message?: unknown } } })?.data;
  if (typeof data?.error?.message === "string") return data.error.message;
  if (typeof data?.message === "string") return data.message;
  return null;
}

type ManageAnnouncementsPanelProps = {
  mode: "create" | "edit";
  announcement?: Announcement | null;
  onSuccess?: () => void;
  onCancel?: () => void;
};

export function ManageAnnouncementsPanel({
  mode,
  announcement,
  onSuccess,
  onCancel,
}: ManageAnnouncementsPanelProps) {
  const toast = useToast();

  const [createAnnouncement, createState] = useCreateAnnouncementMutation();
  const [updateAnnouncement, updateState] = useUpdateAnnouncementMutation();

  const isSubmitting = createState.isLoading || updateState.isLoading;
  const attachmentCount = announcement?.attachment_count ?? announcement?.attachments?.length ?? 0;
  const linkUrl = announcement?.link_url?.trim();
  const linkLabel = announcement?.link_label?.trim() || "Open related link";

  const handleSubmit = async (payload: CreateAnnouncementRequest | UpdateAnnouncementRequest) => {
    try {
      if (mode === "create") {
        await createAnnouncement(payload as CreateAnnouncementRequest).unwrap();

        toast.success({
          title: "Announcement created",
          subtitle: "The announcement was created successfully.",
        });
      } else {
        if (!announcement) return;

        await updateAnnouncement({
          id: announcement.id,
          body: payload as UpdateAnnouncementRequest,
        }).unwrap();

        toast.success({
          title: "Announcement updated",
          subtitle: "The announcement was updated successfully.",
        });
      }

      onSuccess?.();
    } catch (error) {
      toast.error({
        title: mode === "create" ? "Create failed" : "Update failed",
        subtitle:
          apiErrorMessage(error) ??
          (mode === "create" ? "Failed to create announcement." : "Failed to update announcement."),
      });
      // Let the form know the save failed so it keeps the entered values.
      throw error;
    }
  };

  return (
    <Tile className="manage-announcement-panel">
      <Stack gap={6}>
        <div>
          <h3 className="manage-announcement-panel__title">
            {mode === "create" ? "Create Announcement" : "Edit Announcement"}
          </h3>

          <p className="manage-announcement-panel__subtitle">
            {mode === "create"
              ? "Publish announcements, alerts, and important updates for dashboard users."
              : "Update the announcement details, audience, status, schedule, and visibility."}
          </p>
        </div>

        <AnnouncementForm
          mode={mode}
          initialValues={announcement}
          isSubmitting={isSubmitting}
          onSubmit={handleSubmit}
          onCancel={onCancel}
        />

        {mode === "edit" && (linkUrl || attachmentCount > 0) ? (
          <section
            aria-label="Announcement publishing assets"
            className="manage-announcement-panel__assets"
          >
            <h4 className="manage-announcement-panel__asset-title">Publishing assets</h4>

            <div className="manage-announcement-panel__asset-tags">
              {linkUrl && (
                <Tag type="blue" size="sm">
                  <span className="announcements-page__tag-content">
                    <LinkIcon size={12} />
                    Related link
                  </span>
                </Tag>
              )}

              {attachmentCount > 0 && (
                <Tag type="cyan" size="sm">
                  <span className="announcements-page__tag-content">
                    <Attachment size={12} />
                    {attachmentCount === 1 ? "1 attachment" : `${attachmentCount} attachments`}
                  </span>
                </Tag>
              )}
            </div>

            {linkUrl && (
              <Link href={linkUrl} target="_blank" rel="noopener noreferrer">
                {linkLabel}
              </Link>
            )}
          </section>
        ) : null}

        {mode === "edit" && announcement?.id ? (
          <AnnouncementAttachments announcementId={announcement.id} />
        ) : null}
      </Stack>
    </Tile>
  );
}
