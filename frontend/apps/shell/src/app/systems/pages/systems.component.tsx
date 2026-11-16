import {
  Button,
  ContentSwitcher,
  DataTable,
  Form,
  FormGroup,
  InlineLoading,
  Search,
  Select,
  SelectItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tag,
  TextArea,
  TextInput,
  Toggle,
  Switch,
} from "@carbon/react";
import { Add, Launch, Save } from "@carbon/react/icons";
import { useMemo, useState } from "react";
import { useSelector } from "react-redux";
import { selectIsAdmin, useAuthorization } from "@moh-sso/auth";
import { ApplicationManifestPanel } from "../components/application-manifest-panel";

import {
  useGetRbacSystemQuery,
  useListRbacSystemsQuery,
  useUpdateRbacSystemMutation,
} from "@moh-sso/rbac/api";
import type { RbacSystem, UpsertRbacSystemPayload } from "@moh-sso/rbac/types";
import {
  DataTableShell,
  ErrorState,
  RowActionsCell,
  TableStatusTag,
  useHeaderPanel,
  useToast,
} from "@moh-sso/ui";

import "./systems.scss";

const headers = [
  { key: "displayName", header: "System" },
  { key: "clientId", header: "Client ID" },
  { key: "category", header: "Category" },
  { key: "systemType", header: "Type" },
  { key: "launchMode", header: "Launch mode" },
  { key: "environment", header: "Environment" },
  { key: "criticality", header: "Criticality" },
  { key: "enabled", header: "Status" },
  { key: "launchUrl", header: "Launch URL" },
  { key: "actions", header: "" },
  { key: "raw", header: "" },
];

type SystemDraft = UpsertRbacSystemPayload & {
  enabled: boolean;
};

function toDraft(system: RbacSystem): SystemDraft {
  return {
    displayName: system.displayName || system.clientId,
    description: system.description || "",
    icon: system.icon || "",
    launchUrl: system.launchUrl || "",
    authenticatedLaunchUrl: system.authenticatedLaunchUrl || "",
    category: system.category || "",
    ownerTeam: system.ownerTeam || "",
    ownerName: system.ownerName || "",
    ownerEmail: system.ownerEmail || "",
    supportUrl: system.supportUrl || "",
    documentationUrl: system.documentationUrl || "",
    environment: system.environment || "",
    criticality: system.criticality || "",
    navigation: system.navigation || "",
    systemType: system.systemType || "platform",
    displayInLauncher: system.displayInLauncher ?? true,
    displayInSideNav: system.displayInSideNav ?? false,
    launchMode: system.launchMode || "internal",
    enabled: system.enabled,
    sortOrder: system.sortOrder ?? 0,
  };
}

export default function SystemsPage() {
  const isAdmin = useSelector(selectIsAdmin);
  const { canAll } = useAuthorization();
  const { data: systems = [], isLoading, isError, error, refetch } = useListRbacSystemsQuery();
  const [search, setSearch] = useState("");
  const { openPanel, closePanel } = useHeaderPanel();

  const rows = useMemo(
    () =>
      systems
        .filter((system) => {
          const query = search.trim().toLowerCase();
          if (!query) return true;

          return [
            system.displayName,
            system.clientId,
            system.category,
            system.environment,
            system.criticality,
            system.ownerTeam,
          ]
            .filter(Boolean)
            .some((value) => String(value).toLowerCase().includes(query));
        })
        .map((system) => ({
          id: system.clientId,
          displayName: system.displayName || system.clientId,
          clientId: system.clientId,
          category: system.category || "-",
          systemType: system.systemType || "platform",
          launchMode: system.launchMode || "internal",
          environment: system.environment || "-",
          criticality: system.criticality || "-",
          enabled: system.enabled ? "Enabled" : "Disabled",
          launchUrl: system.launchUrl || "-",
          actions: "",
          raw: system,
        })),
    [search, systems],
  );

  const enabledCount = systems.filter((system) => system.enabled).length;

  return (
    <DataTableShell
      title="Systems"
      description="Connected Keycloak clients registered as portal systems."
      rows={rows}
      headers={headers}
      getRowId={(row) => row.id}
      isLoading={isLoading}
      loadingDescription="Loading systems..."
      emptyTitle="No systems found"
      emptyDescription="No connected systems match the selected filters."
      tableState={
        isError ? (
          <ErrorState
            title="Failed to load systems"
            description={getApiErrorMessage(error, "Unable to fetch system registry.")}
            primaryAction={{ label: "Retry", onClick: refetch }}
          />
        ) : undefined
      }
      toolbarActions={
        <>
          {isAdmin && canAll(["rbac:read", "rbac:write", "rbac:roles:write"]) && (
            <Button
              kind="secondary"
              onClick={() => openPanel({
                title: "Import application manifest",
                size: "lg",
                content: <ApplicationManifestPanel />,
              })}
            >
              Import manifest
            </Button>
          )}
          <Button
            renderIcon={Add}
            onClick={() =>
              openPanel({
                title: "Add system",
                size: "lg",
                content: <CreateSystemPanel onClose={closePanel} />,
              })
            }
          >
            Add system
          </Button>
        </>
      }
      topContent={
        <div className="systems-summary">
          <SummaryTile label="Registered" value={systems.length} />
          <SummaryTile label="Enabled" value={enabledCount} />
          <SummaryTile label="Disabled" value={systems.length - enabledCount} />
        </div>
      }
      filters={
        <Search
          id="systems-search"
          labelText="Search systems"
          placeholder="Search by system, client ID, category..."
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      }
    >
      {({ rows, headers }) => (
        <DataTable rows={rows} headers={headers}>
          {({ rows, headers, getHeaderProps, getRowProps }) => (
            <Table size="lg">
              <TableHead>
                <TableRow>
                  {headers
                    .filter((header) => header.key !== "raw")
                    .map((header) => (
                      <TableHeader {...getHeaderProps({ header })}>{header.header}</TableHeader>
                    ))}
                </TableRow>
              </TableHead>

              <TableBody>
                {rows.map((row) => {
                  const system = row.cells.find((cell) => cell.info.header === "raw")
                    ?.value as RbacSystem;

                  return (
                    <TableRow {...getRowProps({ row })}>
                      {row.cells.map((cell) => {
                        if (cell.info.header === "raw") return null;

                        if (cell.info.header === "enabled") {
                          return (
                            <TableCell key={cell.id}>
                              <TableStatusTag
                                status={String(cell.value)}
                                kind={system.enabled ? "green" : "red"}
                              />
                            </TableCell>
                          );
                        }

                        if (cell.info.header === "criticality") {
                          return (
                            <TableCell key={cell.id}>
                              <Tag size="sm" type={getCriticalityTag(system.criticality)}>
                                {system.criticality || "Unclassified"}
                              </Tag>
                            </TableCell>
                          );
                        }

                        if (cell.info.header === "actions") {
                          return (
                            <RowActionsCell key={cell.id}>
                              <Button
                                size="sm"
                                kind="ghost"
                                renderIcon={Launch}
                                onClick={() => {
                                  openPanel({
                                    title: `System: ${system.displayName || system.clientId}`,
                                    size: "lg",
                                    content: (
                                      <SystemPanel
                                        clientId={system.clientId}
                                        onClose={closePanel}
                                      />
                                    ),
                                  });
                                }}
                              >
                                Manage
                              </Button>
                            </RowActionsCell>
                          );
                        }

                        return <TableCell key={cell.id}>{cell.value || "-"}</TableCell>;
                      })}
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </DataTable>
      )}
    </DataTableShell>
  );
}

function SummaryTile({ label, value }: { label: string; value: number }) {
  return (
    <div className="systems-summary__item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function SystemPanel({ clientId, onClose }: { clientId: string; onClose: () => void }) {
  const toast = useToast();
  const { data: system, isLoading, isError, error } = useGetRbacSystemQuery(clientId);
  const [updateSystem, { isLoading: saving }] = useUpdateRbacSystemMutation();
  const [draft, setDraft] = useState<SystemDraft | null>(null);
  const [validationError, setValidationError] = useState("");

  const currentDraft = draft ?? (system ? toDraft(system) : null);

  if (isLoading) {
    return <InlineLoading description="Loading system..." />;
  }

  if (isError || !system || !currentDraft) {
    return (
      <ErrorState
        title="Unable to load system"
        description={getApiErrorMessage(error, "Check RBAC sync and try again.")}
      />
    );
  }

  const updateField = <K extends keyof SystemDraft>(key: K, value: SystemDraft[K]) => {
    setDraft({ ...currentDraft, [key]: value });
  };

  const handleSave = async () => {
    const validation = validateSystemDraft(currentDraft);
    if (validation) {
      setValidationError(validation);
      return;
    }
    setValidationError("");
    try {
      await updateSystem({ clientId, data: currentDraft }).unwrap();
      toast.success("System updated", `${currentDraft.displayName} was saved.`);
      onClose();
    } catch (err) {
      toast.error("Save failed", getApiErrorMessage(err, "Unable to update system."));
    }
  };

  return (
    <Form className="system-panel">
      <Stack gap={6}>
        <FormGroup legendText="System metadata">
          <Stack gap={4}>
            <TextInput
              id="system-client-id"
              labelText="Client ID"
              value={system.clientId}
              disabled
            />
            <TextInput
              id="system-display-name"
              labelText="Display name"
              value={currentDraft.displayName}
              onChange={(event) => updateField("displayName", event.target.value)}
            />
            <ContentSwitcher
              selectedIndex={currentDraft.systemType === "external" ? 1 : 0}
              onChange={({ index }) => {
                const external = index === 1;
                setDraft({
                  ...currentDraft,
                  systemType: external ? "external" : "platform",
                  launchMode: external ? "new_tab" : "internal",
                  displayInSideNav: external ? false : currentDraft.displayInSideNav,
                  navigation: external ? "" : currentDraft.navigation,
                });
              }}
            >
              <Switch name="platform" text="Platform" />
              <Switch name="external" text="External" />
            </ContentSwitcher>
            {currentDraft.systemType === "external" && (
              <Select
                id="system-launch-mode"
                labelText="Launch mode"
                value={currentDraft.launchMode}
                onChange={(event) =>
                  updateField("launchMode", event.target.value as "new_tab" | "same_tab")
                }
              >
                <SelectItem value="new_tab" text="Open in a new tab" />
                <SelectItem value="same_tab" text="Open in the current tab" />
              </Select>
            )}
            <TextArea
              id="system-description"
              labelText="Description"
              value={currentDraft.description}
              onChange={(event) => updateField("description", event.target.value)}
            />
            <TextInput
              id="system-launch-url"
              labelText="Launch URL"
              value={currentDraft.launchUrl}
              onChange={(event) => updateField("launchUrl", event.target.value)}
            />
            <TextInput
              id="system-authenticated-launch-url"
              labelText="Authenticated launch URL"
              helperText="Optional backend login URL used when another application launches this system."
              value={currentDraft.authenticatedLaunchUrl ?? ""}
              onChange={(event) => updateField("authenticatedLaunchUrl", event.target.value)}
            />
            <Stack orientation="horizontal" gap={4}>
              <TextInput
                id="system-category"
                labelText="Category"
                value={currentDraft.category}
                onChange={(event) => updateField("category", event.target.value)}
              />
              <TextInput
                id="system-icon"
                labelText="Icon"
                value={currentDraft.icon}
                onChange={(event) => updateField("icon", event.target.value)}
              />
            </Stack>
            <Toggle
              id="system-display-launcher"
              labelText="Display in application launcher"
              toggled={currentDraft.displayInLauncher}
              onToggle={(value) => updateField("displayInLauncher", value)}
            />
            {currentDraft.systemType === "platform" && (
              <>
                <Toggle
                  id="system-display-sidenav"
                  labelText="Display in side navigation"
                  toggled={currentDraft.displayInSideNav}
                  onToggle={(value) => updateField("displayInSideNav", value)}
                />
                {currentDraft.displayInSideNav && (
                  <TextArea
                    id="system-navigation"
                    labelText="Navigation JSON"
                    value={currentDraft.navigation}
                    onChange={(event) => updateField("navigation", event.target.value)}
                  />
                )}
              </>
            )}
            <Stack orientation="horizontal" gap={4}>
              <TextInput
                id="system-environment"
                labelText="Environment"
                value={currentDraft.environment}
                onChange={(event) => updateField("environment", event.target.value)}
              />
              <TextInput
                id="system-criticality"
                labelText="Criticality"
                value={currentDraft.criticality}
                onChange={(event) => updateField("criticality", event.target.value)}
              />
            </Stack>
            <Toggle
              id="system-enabled"
              labelText="Enabled"
              toggled={currentDraft.enabled}
              onToggle={(enabled) => updateField("enabled", enabled)}
            />
          </Stack>
        </FormGroup>

        <FormGroup legendText="Ownership">
          <Stack gap={4}>
            <TextInput
              id="system-owner-team"
              labelText="Owner team"
              value={currentDraft.ownerTeam}
              onChange={(event) => updateField("ownerTeam", event.target.value)}
            />
            <Stack orientation="horizontal" gap={4}>
              <TextInput
                id="system-owner-name"
                labelText="Owner name"
                value={currentDraft.ownerName}
                onChange={(event) => updateField("ownerName", event.target.value)}
              />
              <TextInput
                id="system-owner-email"
                labelText="Owner email"
                value={currentDraft.ownerEmail}
                onChange={(event) => updateField("ownerEmail", event.target.value)}
              />
            </Stack>
            <TextInput
              id="system-support-url"
              labelText="Support URL"
              value={currentDraft.supportUrl}
              onChange={(event) => updateField("supportUrl", event.target.value)}
            />
            <TextInput
              id="system-documentation-url"
              labelText="Documentation URL"
              value={currentDraft.documentationUrl}
              onChange={(event) => updateField("documentationUrl", event.target.value)}
            />
          </Stack>
        </FormGroup>

        <FormGroup legendText="RBAC mapping">
          <div className="system-panel__mapping">
            <SummaryTile label="System roles" value={system.roles.length} />
            <SummaryTile label="Access roles" value={system.accessRoles.length} />
          </div>

          <div className="system-panel__tags">
            {system.accessRoles.map((role) => (
              <Tag key={role} type="cyan">
                {role}
              </Tag>
            ))}
            {system.accessRoles.length === 0 && <span>No access roles configured.</span>}
          </div>
        </FormGroup>

        {validationError && <p className="system-panel__validation-error">{validationError}</p>}
        <Button renderIcon={Save} disabled={saving} onClick={handleSave}>
          {saving ? "Saving..." : "Save system"}
        </Button>
      </Stack>
    </Form>
  );
}

function CreateSystemPanel({ onClose }: { onClose: () => void }) {
  const toast = useToast();
  const [updateSystem, { isLoading: saving }] = useUpdateRbacSystemMutation();
  const [clientId, setClientId] = useState("");
  const [draft, setDraft] = useState<SystemDraft>({
    displayName: "",
    description: "",
    icon: "application",
    launchUrl: "/portal",
    authenticatedLaunchUrl: "",
    category: "platform",
    ownerTeam: "",
    ownerName: "",
    ownerEmail: "",
    supportUrl: "",
    documentationUrl: "",
    environment: "",
    criticality: "",
    navigation: "",
    systemType: "platform",
    displayInLauncher: true,
    displayInSideNav: false,
    launchMode: "internal",
    enabled: true,
    sortOrder: 0,
  });
  const [validationError, setValidationError] = useState("");

  const setSystemType = (external: boolean) => {
    setDraft((current) => ({
      ...current,
      systemType: external ? "external" : "platform",
      launchMode: external ? "new_tab" : "internal",
      displayInSideNav: false,
      navigation: "",
      launchUrl: external ? "https://" : "/portal",
    }));
  };

  const handleCreate = async () => {
    const normalizedClientID = clientId.trim();
    if (!normalizedClientID || !/^[a-z0-9][a-z0-9-]*$/.test(normalizedClientID)) {
      setValidationError("Client ID must use lowercase letters, numbers, and hyphens.");
      return;
    }
    if (!draft.displayName.trim()) {
      setValidationError("Display name is required.");
      return;
    }
    const validation = validateSystemDraft(draft);
    if (validation) {
      setValidationError(validation);
      return;
    }
    try {
      await updateSystem({ clientId: normalizedClientID, data: draft }).unwrap();
      toast.success("System added", `${draft.displayName} was added to the registry.`);
      onClose();
    } catch (error) {
      toast.error("Create failed", getApiErrorMessage(error, "Unable to add system."));
    }
  };

  return (
    <Form className="system-panel">
      <Stack gap={5}>
        <TextInput
          id="new-system-client-id"
          labelText="Client ID"
          value={clientId}
          onChange={(event) => setClientId(event.target.value)}
        />
        <TextInput
          id="new-system-display-name"
          labelText="Display name"
          value={draft.displayName}
          onChange={(event) => setDraft({ ...draft, displayName: event.target.value })}
        />
        <ContentSwitcher
          selectedIndex={draft.systemType === "external" ? 1 : 0}
          onChange={({ index }) => setSystemType(index === 1)}
        >
          <Switch name="platform" text="Platform" />
          <Switch name="external" text="External" />
        </ContentSwitcher>
        <TextInput
          id="new-system-launch-url"
          labelText="Launch URL"
          value={draft.launchUrl}
          onChange={(event) => setDraft({ ...draft, launchUrl: event.target.value })}
        />
        <TextInput
          id="new-system-authenticated-launch-url"
          labelText="Authenticated launch URL"
          helperText="Optional backend login URL used for cross-application SSO."
          value={draft.authenticatedLaunchUrl ?? ""}
          onChange={(event) => setDraft({ ...draft, authenticatedLaunchUrl: event.target.value })}
        />
        {draft.systemType === "external" && (
          <Select
            id="new-system-launch-mode"
            labelText="Launch mode"
            value={draft.launchMode}
            onChange={(event) =>
              setDraft({ ...draft, launchMode: event.target.value as "new_tab" | "same_tab" })
            }
          >
            <SelectItem value="new_tab" text="Open in a new tab" />
            <SelectItem value="same_tab" text="Open in the current tab" />
          </Select>
        )}
        <Toggle
          id="new-system-display-launcher"
          labelText="Display in application launcher"
          toggled={draft.displayInLauncher}
          onToggle={(value) => setDraft({ ...draft, displayInLauncher: value })}
        />
        {draft.systemType === "platform" && (
          <>
            <Toggle
              id="new-system-display-sidenav"
              labelText="Display in side navigation"
              toggled={draft.displayInSideNav}
              onToggle={(value) => setDraft({ ...draft, displayInSideNav: value })}
            />
            {draft.displayInSideNav && (
              <TextArea
                id="new-system-navigation"
                labelText="Navigation JSON"
                value={draft.navigation}
                onChange={(event) => setDraft({ ...draft, navigation: event.target.value })}
              />
            )}
          </>
        )}
        {validationError && <p className="system-panel__validation-error">{validationError}</p>}
        <Button renderIcon={Save} disabled={saving} onClick={handleCreate}>
          {saving ? "Adding..." : "Add system"}
        </Button>
      </Stack>
    </Form>
  );
}

function validateSystemDraft(draft: SystemDraft): string {
  const authenticatedLaunchUrl = draft.authenticatedLaunchUrl?.trim();
  if (
    authenticatedLaunchUrl &&
    !authenticatedLaunchUrl.startsWith("/api/v1/auth/launch") &&
    !isSafeAbsoluteURL(authenticatedLaunchUrl)
  ) {
    return "Authenticated launch URL must use /api/v1/auth/launch or a safe absolute HTTP or HTTPS URL.";
  }

  if (draft.systemType === "platform") {
    if (draft.launchMode !== "internal") return "Platform systems must use internal launch mode.";
    if (draft.launchUrl && !/^\/(portal|apps)(\/|$)/.test(draft.launchUrl)) {
      return "Platform launch URLs must begin with /portal or /apps.";
    }
    if (draft.displayInSideNav) {
      try {
        const items = JSON.parse(draft.navigation || "[]");
        if (!Array.isArray(items) || items.length === 0) throw new Error("empty");
      } catch {
        return "Side navigation requires a non-empty navigation JSON array.";
      }
    }
    return "";
  }

  if (draft.displayInSideNav || (draft.navigation ?? "").trim()) {
    return "External systems cannot define portal side navigation.";
  }
  if (!isSafeAbsoluteURL(draft.launchUrl ?? "")) {
    return "External launch URLs must be safe absolute HTTP or HTTPS URLs.";
  }
  return "";
}

function isSafeAbsoluteURL(value: string): boolean {
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password;
  } catch {
    return false;
  }
}

function getCriticalityTag(criticality?: string) {
  const value = criticality?.toLowerCase();
  if (value === "critical" || value === "high") return "red";
  if (value === "medium") return "purple";
  if (value === "low") return "green";
  return "gray";
}

function getApiErrorMessage(error: unknown, fallback: string) {
  if (!error || typeof error !== "object") {
    return fallback;
  }

  const maybeError = error as {
    data?: {
      message?: unknown;
      error?: {
        message?: unknown;
      };
    };
    error?: unknown;
  };

  if (typeof maybeError.data?.message === "string") return maybeError.data.message;
  if (typeof maybeError.data?.error?.message === "string") return maybeError.data.error.message;
  if (typeof maybeError.error === "string") return maybeError.error;
  return fallback;
}
