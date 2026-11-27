import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  DatePicker,
  DatePickerInput,
  Form,
  FormGroup,
  InlineLoading,
  MultiSelect,
  Select,
  SelectItem,
  Stack,
  TextArea,
  TextInput,
} from "@carbon/react";
import { Add, Save, Send } from "@carbon/react/icons";

import type {
  Announcement,
  AnnouncementAudiencePreviewRequest,
  AnnouncementAudienceType,
  AnnouncementLevel,
  AnnouncementStatus,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
} from "../types";
import { useAvailableHealthContexts } from "@moh-sso/auth";
import { useToast } from "@moh-sso/ui";
import {
  useListRbacGroupsQuery,
  useListRbacSystemsQuery,
  useListRealmRolePermissionsQuery,
} from "@moh-sso/rbac";
import { useListUsersQuery } from "@moh-sso/users";
import { usePreviewAnnouncementAudienceMutation } from "../api";
import "./announcements.components.scss";

export interface AnnouncementFormValues {
  title: string;
  message: string;
  summary: string;
  level: AnnouncementLevel;
  tag: string;
  link_url: string;
  link_label: string;
  priority: number;
  is_pinned: boolean;
  status: AnnouncementStatus;
  audience_type: AnnouncementAudienceType;
  client_ids: string[];
  role_names: string[];
  user_ids: string[];
  group_ids: string[];
  health_context_ids: string[];
  publish_at: string;
  expires_at: string;

  /**
   * If true, email notifications will be queued when this announcement is published.
   */
  notify_by_email: boolean;
  notify_by_sms: boolean;
  sms_message: string;
}

interface AnnouncementFormProps {
  mode: "create" | "edit";
  initialValues?: Partial<AnnouncementFormValues> | Announcement | null;
  isSubmitting?: boolean;
  onSubmit: (
    payload: CreateAnnouncementRequest | UpdateAnnouncementRequest,
  ) => Promise<void> | void;
  onCancel?: () => void;
  submitLabel?: string;
}

const initialForm: AnnouncementFormValues = {
  title: "",
  message: "",
  summary: "",
  level: "INFO",
  tag: "",
  link_url: "",
  link_label: "",
  priority: 0,
  is_pinned: false,
  status: "DRAFT",
  audience_type: "ALL_USERS",
  client_ids: [],
  role_names: [],
  user_ids: [],
  group_ids: [],
  health_context_ids: [],
  publish_at: "",
  expires_at: "",
  notify_by_email: false,
  notify_by_sms: false,
  sms_message: "",
};

function toIsoString(value?: string | null) {
  if (!value) return "";

  const date = new Date(value);

  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function normalizeInitialValues(
  values?: Partial<AnnouncementFormValues> | Announcement | null,
): AnnouncementFormValues {
  if (!values) return initialForm;

  return {
    title: values.title ?? "",
    message: values.message ?? "",
    summary: values.summary ?? "",
    level: (values.level as AnnouncementLevel) ?? "INFO",
    tag: values.tag ?? "",
    link_url: values.link_url ?? "",
    link_label: values.link_label ?? "",
    priority: typeof values.priority === "number" ? values.priority : 0,
    is_pinned: Boolean(values.is_pinned),
    status: (values.status as AnnouncementStatus) ?? "DRAFT",
    audience_type: (values.audience_type as AnnouncementAudienceType) ?? "ALL_USERS",
    client_ids: "client_ids" in values && Array.isArray(values.client_ids) ? values.client_ids : [],
    role_names: "role_names" in values && Array.isArray(values.role_names) ? values.role_names : [],
    user_ids: "user_ids" in values && Array.isArray(values.user_ids) ? values.user_ids : [],
    group_ids: "group_ids" in values && Array.isArray(values.group_ids) ? values.group_ids : [],
    health_context_ids:
      "health_context_ids" in values && Array.isArray(values.health_context_ids)
        ? values.health_context_ids
        : [],
    publish_at: toIsoString(values.publish_at),
    expires_at: toIsoString(values.expires_at),
    notify_by_email: Boolean(values.notify_by_email),
    notify_by_sms: Boolean("notify_by_sms" in values ? values.notify_by_sms : false),
    sms_message: "sms_message" in values && values.sms_message ? values.sms_message : "",
  };
}

export function AnnouncementForm({
  mode,
  initialValues,
  isSubmitting = false,
  onSubmit,
  onCancel,
  submitLabel,
}: AnnouncementFormProps) {
  const toast = useToast();
  const healthContexts = useAvailableHealthContexts();
  const [form, setForm] = useState<AnnouncementFormValues>(normalizeInitialValues(initialValues));
  const { data: systems = [], isLoading: systemsLoading, isError: systemsError } = useListRbacSystemsQuery();
  const {
    data: realmRoles = [],
    isLoading: rolesLoading,
    isError: rolesError,
  } = useListRealmRolePermissionsQuery();
  const { data: users = [], isLoading: usersLoading, isError: usersError } = useListUsersQuery();
  const {
    data: groups = [],
    isLoading: groupsLoading,
    isError: groupsError,
  } = useListRbacGroupsQuery();
  const [previewAudience, audiencePreview] = usePreviewAnnouncementAudienceMutation();

  useEffect(() => {
    setForm(normalizeInitialValues(initialValues));
  }, [initialValues]);

  const systemItems = useMemo(
    () =>
      systems.map((system) => ({
        id: system.id,
        text: system.displayName || system.clientId,
      })),
    [systems],
  );

  const roleItems = useMemo(
    () =>
      realmRoles.map((role) => ({
        id: role.realmRole,
        text: role.realmRole,
      })),
    [realmRoles],
  );

  const userItems = useMemo(
    () =>
      users.map((user) => ({
        id: user.id,
        text: user.email
          ? `${user.username} (${user.email})`
          : user.username,
      })),
    [users],
  );

  const groupItems = useMemo(
    () =>
      groups
        .filter((group) => group.enabled)
        .map((group) => ({
          id: group.id,
          text: group.displayName || group.path || group.name,
        })),
    [groups],
  );

  const healthContextItems = useMemo(
    () =>
      healthContexts
        .filter((context) => context.enabled)
        .map((context) => ({
          id: context.id,
          text: `${context.name} (${context.contextType.toLowerCase()})`,
        })),
    [healthContexts],
  );

  const isValid = useMemo(() => {
    if (form.title.trim().length === 0 || form.message.trim().length === 0) {
      return false;
    }

    if (form.audience_type === "SPECIFIC_CLIENTS") {
      return form.client_ids.length > 0;
    }

    if (form.audience_type === "SPECIFIC_ROLES") {
      return form.role_names.length > 0;
    }

    if (form.audience_type === "SPECIFIC_USERS") {
      return form.user_ids.length > 0;
    }

    if (form.audience_type === "SPECIFIC_GROUPS") {
      return form.group_ids.length > 0;
    }

    if (
      form.audience_type === "SPECIFIC_HEALTH_CONTEXTS" ||
      form.audience_type === "HEALTH_CONTEXT_AND_DESCENDANTS"
    ) {
      return form.health_context_ids.length > 0;
    }

    return true;
  }, [form]);

  const updateForm = <K extends keyof AnnouncementFormValues>(
    key: K,
    value: AnnouncementFormValues[K],
  ) => {
    setForm((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const resetForm = () => {
    setForm(normalizeInitialValues(initialValues));
  };

  const buildPayload = (): CreateAnnouncementRequest | UpdateAnnouncementRequest => {
    const payload: CreateAnnouncementRequest | UpdateAnnouncementRequest = {
      title: form.title.trim(),
      message: form.message.trim(),
      level: form.level,
      priority: Number.isNaN(Number(form.priority)) ? 0 : Number(form.priority),
      is_pinned: form.is_pinned,
      status: form.status,
      audience_type: form.audience_type,
      notify_by_email: form.notify_by_email,
      notify_by_sms: form.notify_by_sms,
    };

    if (form.audience_type === "SPECIFIC_CLIENTS") {
      payload.client_ids = form.client_ids;
    }

    if (form.audience_type === "SPECIFIC_ROLES") {
      payload.role_names = form.role_names;
    }

    if (form.audience_type === "SPECIFIC_USERS") {
      payload.user_ids = form.user_ids;
    }

    if (form.audience_type === "SPECIFIC_GROUPS") {
      payload.group_ids = form.group_ids;
    }

    if (
      form.audience_type === "SPECIFIC_HEALTH_CONTEXTS" ||
      form.audience_type === "HEALTH_CONTEXT_AND_DESCENDANTS"
    ) {
      payload.health_context_ids = form.health_context_ids;
    }

    if (form.summary.trim()) {
      payload.summary = form.summary.trim();
    }

    if (form.tag.trim()) {
      payload.tag = form.tag.trim();
    }

    if (form.link_url.trim()) {
      payload.link_url = form.link_url.trim();
    }

    if (form.link_label.trim()) {
      payload.link_label = form.link_label.trim();
    }

    if (form.publish_at.trim()) {
      payload.publish_at = form.publish_at;
    }

    if (form.expires_at.trim()) {
      payload.expires_at = form.expires_at;
    }

    if (form.sms_message.trim()) {
      payload.sms_message = form.sms_message.trim();
    }

    return payload;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!isValid) {
      toast.error(
        "Invalid",
        "Please provide a title, message, and the required audience selection.",
      );
      return;
    }

    // onSubmit owns the API call and its success/error toasts; it rejects when
    // the save fails, in which case the entered values are kept.
    try {
      await onSubmit(buildPayload());
    } catch {
      return;
    }

    if (mode === "create") {
      setForm(initialForm);
    }
  };

  return (
    <Form onSubmit={handleSubmit}>
      <Stack gap={6}>
        <TextInput
          id="announcement-title"
          labelText="Title"
          placeholder="Enter announcement title"
          value={form.title}
          onChange={(e) => updateForm("title", e.target.value)}
          maxLength={255}
        />

        <TextInput
          id="announcement-summary"
          labelText="Summary"
          placeholder="Optional short summary"
          value={form.summary}
          onChange={(e) => updateForm("summary", e.target.value)}
          maxLength={500}
        />

        <TextArea
          id="announcement-message"
          labelText="Message"
          placeholder="Write the announcement message"
          rows={8}
          value={form.message}
          onChange={(e) => updateForm("message", e.target.value)}
          maxCount={2000}
          enableCounter
        />

        <div className="announcement-form__grid">
          <Select
            id="announcement-level"
            labelText="Level"
            value={form.level}
            onChange={(e) => updateForm("level", e.target.value as AnnouncementLevel)}
          >
            <SelectItem value="INFO" text="INFO" />
            <SelectItem value="SUCCESS" text="SUCCESS" />
            <SelectItem value="WARNING" text="WARNING" />
            <SelectItem value="CRITICAL" text="CRITICAL" />
          </Select>

          <Select
            id="announcement-status"
            labelText="Status"
            value={form.status}
            onChange={(e) => updateForm("status", e.target.value as AnnouncementStatus)}
          >
            <SelectItem value="DRAFT" text="DRAFT" />
            <SelectItem value="SCHEDULED" text="SCHEDULED" />
            <SelectItem value="PUBLISHED" text="PUBLISHED" />
            <SelectItem value="ARCHIVED" text="ARCHIVED" />
          </Select>

          <Select
            id="announcement-audience-type"
            labelText="Audience"
            value={form.audience_type}
            onChange={(e) =>
              updateForm("audience_type", e.target.value as AnnouncementAudienceType)
            }
          >
            <SelectItem value="ALL_USERS" text="ALL_USERS" />
            <SelectItem value="ADMINS_ONLY" text="ADMINS_ONLY" />
            <SelectItem value="SPECIFIC_CLIENTS" text="SPECIFIC_CLIENTS" />
            <SelectItem value="SPECIFIC_ROLES" text="SPECIFIC_ROLES" />
            <SelectItem value="SPECIFIC_USERS" text="SPECIFIC_USERS" />
            <SelectItem value="SPECIFIC_GROUPS" text="SPECIFIC_GROUPS" />
            <SelectItem value="SPECIFIC_HEALTH_CONTEXTS" text="SPECIFIC_HEALTH_CONTEXTS" />
            <SelectItem
              value="HEALTH_CONTEXT_AND_DESCENDANTS"
              text="HEALTH_CONTEXT_AND_DESCENDANTS"
            />
          </Select>

          <TextInput
            id="announcement-tag"
            labelText="Tag"
            placeholder="Optional tag"
            value={form.tag}
            onChange={(e) => updateForm("tag", e.target.value)}
          />

          <TextInput
            id="announcement-priority"
            labelText="Priority"
            type="number"
            min={0}
            value={String(form.priority)}
            onChange={(e) => updateForm("priority", Number(e.target.value || 0))}
          />

          <TextInput
            id="announcement-link-url"
            labelText="Link URL"
            placeholder="https://example.com"
            value={form.link_url}
            onChange={(e) => updateForm("link_url", e.target.value)}
          />

          <TextInput
            id="announcement-link-label"
            labelText="Link label"
            placeholder="Optional link label"
            value={form.link_label}
            onChange={(e) => updateForm("link_label", e.target.value)}
          />

          <div>
            <DatePicker
              datePickerType="single"
              value={form.publish_at ? [form.publish_at] : []}
              onChange={(dates) => {
                const selectedDate = dates?.[0];

                updateForm(
                  "publish_at",
                  selectedDate instanceof Date ? selectedDate.toISOString() : "",
                );
              }}
            >
              <DatePickerInput
                id="announcement-publish-date"
                labelText="Publish date"
                placeholder="yyyy-mm-dd"
              />
            </DatePicker>
          </div>

          <div>
            <DatePicker
              datePickerType="single"
              value={form.expires_at ? [form.expires_at] : []}
              onChange={(dates) => {
                const selectedDate = dates?.[0];

                updateForm(
                  "expires_at",
                  selectedDate instanceof Date ? selectedDate.toISOString() : "",
                );
              }}
            >
              <DatePickerInput
                id="announcement-expiry-date"
                labelText="Expiry date"
                placeholder="yyyy-mm-dd"
              />
            </DatePicker>
          </div>
        </div>

        <FormGroup legendText="Audience targeting">
          <Stack gap={4}>
            {form.audience_type === "ALL_USERS" && (
              <p className="announcement-form__hint">
                This announcement will be visible to all portal users.
              </p>
            )}

            {form.audience_type === "ADMINS_ONLY" && (
              <p className="announcement-form__hint">
                This announcement will be visible to users with the admin realm role.
              </p>
            )}

            {form.audience_type === "SPECIFIC_CLIENTS" && (
              <>
                {systemsLoading && <InlineLoading description="Loading systems..." />}
                <MultiSelect
                  id="announcement-client-audience"
                  titleText="Systems"
                  label={systemsError ? "Unable to load systems" : "Select systems"}
                  items={systemItems}
                  itemToString={(item) => item?.text ?? ""}
                  selectedItems={systemItems.filter((item) => form.client_ids.includes(item.id))}
                  disabled={systemsLoading || systemsError}
                  invalid={form.client_ids.length === 0}
                  invalidText="Select at least one system."
                  onChange={({ selectedItems }) =>
                    updateForm(
                      "client_ids",
                      (selectedItems ?? []).map((item) => item.id),
                    )
                  }
                />
              </>
            )}

            {form.audience_type === "SPECIFIC_ROLES" && (
              <>
                {rolesLoading && <InlineLoading description="Loading roles..." />}
                <MultiSelect
                  id="announcement-role-audience"
                  titleText="Realm roles"
                  label={rolesError ? "Unable to load roles" : "Select realm roles"}
                  items={roleItems}
                  itemToString={(item) => item?.text ?? ""}
                  selectedItems={roleItems.filter((item) => form.role_names.includes(item.id))}
                  disabled={rolesLoading || rolesError}
                  invalid={form.role_names.length === 0}
                  invalidText="Select at least one role."
                  onChange={({ selectedItems }) =>
                    updateForm(
                      "role_names",
                      (selectedItems ?? []).map((item) => item.id),
                    )
                  }
                />
              </>
            )}

            {form.audience_type === "SPECIFIC_USERS" && (
              <>
                {usersLoading && <InlineLoading description="Loading users..." />}
                <MultiSelect
                  id="announcement-user-audience"
                  titleText="Users"
                  label={usersError ? "Unable to load users" : "Select users"}
                  items={userItems}
                  itemToString={(item) => item?.text ?? ""}
                  selectedItems={userItems.filter((item) => form.user_ids.includes(item.id))}
                  disabled={usersLoading || usersError}
                  invalid={form.user_ids.length === 0}
                  invalidText="Select at least one user."
                  onChange={({ selectedItems }) =>
                    updateForm(
                      "user_ids",
                      (selectedItems ?? []).map((item) => item.id),
                    )
                  }
                />
              </>
            )}

            {form.audience_type === "SPECIFIC_GROUPS" && (
              <>
                {groupsLoading && <InlineLoading description="Loading groups..." />}
                <MultiSelect
                  id="announcement-group-audience"
                  titleText="Groups"
                  label={groupsError ? "Unable to load groups" : "Select groups"}
                  items={groupItems}
                  itemToString={(item) => item?.text ?? ""}
                  selectedItems={groupItems.filter((item) => form.group_ids.includes(item.id))}
                  disabled={groupsLoading || groupsError}
                  invalid={form.group_ids.length === 0}
                  invalidText="Select at least one group."
                  onChange={({ selectedItems }) =>
                    updateForm(
                      "group_ids",
                      (selectedItems ?? []).map((item) => item.id),
                    )
                  }
                />
              </>
            )}

            {(form.audience_type === "SPECIFIC_HEALTH_CONTEXTS" ||
              form.audience_type === "HEALTH_CONTEXT_AND_DESCENDANTS") && (
              <>
                <MultiSelect
                  id="announcement-health-context-audience"
                  titleText="Health contexts"
                  label="Select health contexts"
                  items={healthContextItems}
                  itemToString={(item) => item?.text ?? ""}
                  selectedItems={healthContextItems.filter((item) =>
                    form.health_context_ids.includes(item.id),
                  )}
                  invalid={form.health_context_ids.length === 0}
                  invalidText="Select at least one health context."
                  onChange={({ selectedItems }) => {
                    updateForm(
                      "health_context_ids",
                      (selectedItems ?? []).map((item) => item.id),
                    );
                    audiencePreview.reset();
                  }}
                />
                <div className="announcement-form__audience-preview">
                  <Button
                    type="button"
                    kind="tertiary"
                    size="sm"
                    disabled={
                      form.health_context_ids.length === 0 ||
                      audiencePreview.isLoading
                    }
                    onClick={async () => {
                      try {
                        const request: AnnouncementAudiencePreviewRequest = {
                          audience_type:
                            form.audience_type as AnnouncementAudiencePreviewRequest["audience_type"],
                          health_context_ids: form.health_context_ids,
                        };
                        await previewAudience(request).unwrap();
                      } catch (error) {
                        toast.error(
                          "Unable to preview audience",
                          "Confirm that you can access every selected health context.",
                        );
                      }
                    }}
                  >
                    Preview recipients
                  </Button>
                  {audiencePreview.isLoading ? (
                    <InlineLoading description="Resolving recipients" />
                  ) : null}
                  {audiencePreview.data ? (
                    <p className="announcement-form__hint" aria-live="polite">
                      {audiencePreview.data.recipient_count} eligible recipient
                      {audiencePreview.data.recipient_count === 1 ? "" : "s"}.
                    </p>
                  ) : null}
                </div>
              </>
            )}
          </Stack>
        </FormGroup>

        <div className="announcement-form__toggles">
          <Checkbox
            id="announcement-pinned"
            labelText="Pin this announcement"
            checked={form.is_pinned}
            onChange={(_, { checked }) => updateForm("is_pinned", Boolean(checked))}
          />

          <Checkbox
            id="announcement-notify-by-email"
            labelText="Send email notification when this announcement is published"
            checked={form.notify_by_email}
            onChange={(_, { checked }) => updateForm("notify_by_email", Boolean(checked))}
          />

          <Checkbox
            id="announcement-notify-by-sms"
            labelText="Send SMS notification when this announcement is published"
            checked={form.notify_by_sms}
            onChange={(_, { checked }) => updateForm("notify_by_sms", Boolean(checked))}
          />

          {form.notify_by_sms ? (
            <TextArea
              id="announcement-sms-message"
              labelText="SMS message"
              helperText="Optional. Leave blank to use the announcement summary or message."
              placeholder="Short SMS message"
              rows={3}
              value={form.sms_message}
              onChange={(e) => updateForm("sms_message", e.target.value)}
              maxCount={160}
              enableCounter
            />
          ) : null}

          <p className="announcement-form__notification-note">
            Email and SMS notifications are only queued when the announcement is published and the
            matching option is enabled. Drafts and scheduled announcements will not notify users
            until they are published.
          </p>
        </div>

        <div className="announcement-form__actions">
          <Button
            type="submit"
            renderIcon={mode === "create" ? Send : Save}
            disabled={!isValid || isSubmitting}
          >
            {isSubmitting
              ? mode === "create"
                ? "Saving..."
                : "Updating..."
              : (submitLabel ?? (mode === "create" ? "Create announcement" : "Save changes"))}
          </Button>

          <Button
            type="button"
            kind="secondary"
            renderIcon={Add}
            onClick={resetForm}
            disabled={isSubmitting}
          >
            Reset form
          </Button>

          {onCancel ? (
            <Button type="button" kind="ghost" onClick={onCancel} disabled={isSubmitting}>
              Cancel
            </Button>
          ) : null}
        </div>
      </Stack>
    </Form>
  );
}
