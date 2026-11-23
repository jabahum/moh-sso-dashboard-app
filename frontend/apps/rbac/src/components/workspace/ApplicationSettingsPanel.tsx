import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Button,
  Dropdown,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Tag,
  TextArea,
  TextInput,
  Toggle,
} from "@carbon/react";
import { Reset, Save } from "@carbon/react/icons";
import { PERMISSIONS, PermissionGuard } from "@moh-sso/auth";
import { useToast } from "@moh-sso/ui";

import { useRegisterRbacSystemMutation, useUpdateRbacSystemMutation } from "../../api";
import type { RbacSystemDetail } from "../../types";
import { ActionError, useUnsavedChanges } from "./feedback";
import {
  getApiErrorMessage,
  parseNavigationPreview,
  toDraft,
  type SystemDraft,
} from "./workspace-helpers";

const settingsTabs = [
  { id: "basics", label: "Basics" },
  { id: "ownership", label: "Ownership & support" },
  { id: "integration", label: "Integration & environment" },
  { id: "launch", label: "Launch & visibility" },
  { id: "navigation", label: "Navigation" },
] as const;
type SettingsTab = (typeof settingsTabs)[number]["id"];

const integrationTypeLabels = { external: "External", hosted_module: "Hosted module" } as const;
const systemTypeLabels = { platform: "Platform", external: "External" } as const;
const launchModeLabels = {
  internal: "Inside the portal",
  new_tab: "New browser tab",
  same_tab: "Same browser tab",
} as const;

type ApplicationSettingsPanelProps = {
  /** The selected application; undefined while registering a new one. */
  system?: RbacSystemDetail;
  registering: boolean;
  canWriteSystems: boolean;
  onRegistered: (clientId: string) => void;
};

export function ApplicationSettingsPanel({
  system,
  registering,
  canWriteSystems,
  onRegistered,
}: ApplicationSettingsPanelProps) {
  const toast = useToast();
  const [registerSystem, { isLoading: registeringSystem }] = useRegisterRbacSystemMutation();
  const [updateSystem, { isLoading: updatingSystem }] = useUpdateRbacSystemMutation();
  const saving = registeringSystem || updatingSystem;

  const [tab, setTab] = useState<SettingsTab>("basics");
  const [edits, setEdits] = useState<Partial<SystemDraft>>({});
  const [newClientId, setNewClientId] = useState("");
  // The saved result, used until the refetched application arrives.
  const [savedSnapshot, setSavedSnapshot] = useState<SystemDraft | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setSavedSnapshot(null);
  }, [system]);

  const baseline = useMemo(
    () => (registering ? { ...toDraft(), enabled: false } : (savedSnapshot ?? toDraft(system))),
    [registering, savedSnapshot, system],
  );
  const draft = useMemo(() => ({ ...baseline, ...edits }), [baseline, edits]);
  const changedFields = (Object.keys(edits) as (keyof SystemDraft)[]).filter(
    (field) => edits[field] !== baseline[field],
  );
  const dirty = changedFields.length > 0 || (registering && newClientId.trim() !== "");
  const navigationPreview = useMemo(
    () => parseNavigationPreview(draft.navigation),
    [draft.navigation],
  );

  const discard = useCallback(() => {
    setEdits({});
    setNewClientId("");
    setError(null);
  }, []);

  useUnsavedChanges(
    dirty,
    registering
      ? "New application registration"
      : `Settings for ${system?.displayName || system?.clientId || "this application"}`,
    discard,
  );

  const update = <K extends keyof SystemDraft>(field: K, value: SystemDraft[K]) =>
    setEdits((current) => ({ ...current, [field]: value }));

  const problems: { tab: SettingsTab; message: string }[] = [];
  if (registering && !newClientId.trim())
    problems.push({ tab: "basics", message: "Client ID is required." });
  if (!draft.displayName.trim())
    problems.push({ tab: "basics", message: "Display name is required." });
  if (navigationPreview.error)
    problems.push({ tab: "navigation", message: `Navigation JSON: ${navigationPreview.error}` });

  const clientId = registering ? newClientId.trim() : (system?.clientId ?? "");
  const canSave = canWriteSystems && Boolean(clientId) && dirty && !problems.length && !saving;

  const handleSave = async () => {
    if (!canSave) return;
    try {
      setError(null);
      const result = registering
        ? await registerSystem({ clientId, ...draft }).unwrap()
        : await updateSystem({ clientId, data: draft }).unwrap();
      setEdits({});
      setNewClientId("");
      if (registering) {
        toast.success("Application registered", `${result.displayName} (${result.clientId}).`);
        onRegistered(result.clientId);
      } else {
        setSavedSnapshot(toDraft(result));
        toast.success("Settings saved", `${result.displayName} was updated.`);
      }
    } catch (failure) {
      setError(
        getApiErrorMessage(
          failure,
          registering ? "Unable to register the application." : "Unable to save the settings.",
        ),
      );
    }
  };

  const disabled = !canWriteSystems;
  const tabIndex = settingsTabs.findIndex((item) => item.id === tab);
  const tabHasProblem = (id: SettingsTab) => problems.some((problem) => problem.tab === id);

  return (
    <section className="rbac-card" aria-labelledby="rbac-settings-title">
      <div className="rbac-card__header">
        <div>
          <h2 id="rbac-settings-title">
            {registering ? "Register a new application" : "Application settings"}
          </h2>
          <p>Changes in every tab below are saved together.</p>
        </div>
        <div className="rbac-card__actions">
          {dirty && (
            <Tag type="warm-gray" className="rbac-unsaved-tag">
              Unsaved changes
            </Tag>
          )}
          <PermissionGuard permission={PERMISSIONS.rbacWrite}>
            {dirty && (
              <Button kind="ghost" size="md" renderIcon={Reset} disabled={saving} onClick={discard}>
                Discard
              </Button>
            )}
            <Button
              renderIcon={Save}
              size="md"
              disabled={!canSave}
              onClick={() => void handleSave()}
            >
              {registering ? "Register application" : "Save changes"}
            </Button>
          </PermissionGuard>
        </div>
      </div>
      {(dirty || registering) && problems.length > 0 && (
        <ul className="rbac-form-problems" aria-label="Fix before saving">
          {problems.map((problem) => (
            <li key={problem.message}>
              <button
                type="button"
                className="rbac-link-button"
                onClick={() => setTab(problem.tab)}
              >
                {settingsTabs.find((item) => item.id === problem.tab)?.label}
              </button>
              : {problem.message}
            </li>
          ))}
        </ul>
      )}
      <ActionError
        title={registering ? "Application not registered" : "Settings not saved"}
        message={error}
        onClose={() => setError(null)}
      />

      <Tabs
        selectedIndex={tabIndex}
        onChange={({ selectedIndex }) => setTab(settingsTabs[selectedIndex].id)}
      >
        <TabList aria-label="Settings sections" className="rbac-subtabs">
          {settingsTabs.map((item) => (
            <Tab key={item.id}>
              {item.label}
              {tabHasProblem(item.id) && (dirty || registering) ? " (needs attention)" : ""}
            </Tab>
          ))}
        </TabList>
        <TabPanels>
          <TabPanel className="rbac-subtab-panel">
            <div className="rbac-form-grid">
              <TextInput
                id="rbac-client-id"
                labelText="Client ID"
                helperText={
                  registering
                    ? "Must match the application's Keycloak client ID exactly."
                    : "Identifies the Keycloak client. It cannot be changed after registration."
                }
                value={registering ? newClientId : (system?.clientId ?? "")}
                readOnly={!registering}
                disabled={disabled || !registering}
                invalid={registering && dirty && !newClientId.trim()}
                invalidText="Client ID is required."
                onChange={(event) => setNewClientId(event.target.value)}
              />
              <TextInput
                id="rbac-display-name"
                labelText="Display name"
                helperText="Shown on the launcher tile and in menus."
                value={draft.displayName}
                disabled={disabled}
                invalid={(dirty || registering) && !draft.displayName.trim()}
                invalidText="Display name is required."
                onChange={(event) => update("displayName", event.target.value)}
              />
              <TextInput
                id="rbac-category"
                labelText="Category (optional)"
                helperText="Groups related applications, for example “laboratory”."
                value={draft.category}
                disabled={disabled}
                onChange={(event) => update("category", event.target.value)}
              />
              <TextInput
                id="rbac-icon"
                labelText="Icon (optional)"
                helperText="Icon name used on the launcher tile."
                value={draft.icon}
                disabled={disabled}
                onChange={(event) => update("icon", event.target.value)}
              />
              <div className="rbac-form-grid__wide">
                <TextArea
                  id="rbac-description"
                  labelText="Description (optional)"
                  rows={2}
                  value={draft.description}
                  disabled={disabled}
                  onChange={(event) => update("description", event.target.value)}
                />
              </div>
            </div>
          </TabPanel>
          <TabPanel className="rbac-subtab-panel">
            <div className="rbac-form-grid">
              <TextInput
                id="rbac-owner-team"
                labelText="Owner team"
                value={draft.ownerTeam}
                disabled={disabled}
                onChange={(event) => update("ownerTeam", event.target.value)}
              />
              <TextInput
                id="rbac-owner-name"
                labelText="Owner name"
                value={draft.ownerName}
                disabled={disabled}
                onChange={(event) => update("ownerName", event.target.value)}
              />
              <TextInput
                id="rbac-owner-email"
                labelText="Owner email"
                type="email"
                value={draft.ownerEmail}
                disabled={disabled}
                onChange={(event) => update("ownerEmail", event.target.value)}
              />
              <TextInput
                id="rbac-support-url"
                labelText="Support URL"
                value={draft.supportUrl}
                disabled={disabled}
                onChange={(event) => update("supportUrl", event.target.value)}
              />
              <TextInput
                id="rbac-documentation-url"
                labelText="Documentation URL"
                value={draft.documentationUrl}
                disabled={disabled}
                onChange={(event) => update("documentationUrl", event.target.value)}
              />
            </div>
            <p className="rbac-hint">Validation requires ownership and support details.</p>
          </TabPanel>
          <TabPanel className="rbac-subtab-panel">
            <div className="rbac-form-grid">
              <Dropdown
                id="application-integration-type"
                titleText="Integration type"
                label="Select integration type"
                items={["external", "hosted_module"] as const}
                itemToString={(item) => (item ? integrationTypeLabels[item] : "")}
                selectedItem={draft.integrationType}
                disabled={disabled}
                onChange={({ selectedItem }) =>
                  selectedItem && update("integrationType", selectedItem)
                }
              />
              <TextInput
                id="application-contract-version"
                labelText="Contract version"
                value={draft.contractVersion}
                helperText="Supported version: 1"
                disabled={disabled}
                onChange={(event) => update("contractVersion", event.target.value)}
              />
              <TextInput
                id="rbac-environment"
                labelText="Environment"
                helperText="For example production or staging."
                value={draft.environment}
                disabled={disabled}
                onChange={(event) => update("environment", event.target.value)}
              />
              <TextInput
                id="rbac-criticality"
                labelText="Criticality"
                helperText="For example high, medium or low."
                value={draft.criticality}
                disabled={disabled}
                onChange={(event) => update("criticality", event.target.value)}
              />
              <Dropdown
                id="rbac-system-type"
                titleText="System type"
                label="Select system type"
                items={["platform", "external"] as const}
                itemToString={(item) => (item ? systemTypeLabels[item] : "")}
                selectedItem={draft.systemType}
                disabled={disabled}
                onChange={({ selectedItem }) => selectedItem && update("systemType", selectedItem)}
              />
            </div>
          </TabPanel>
          <TabPanel className="rbac-subtab-panel">
            <div className="rbac-form-grid">
              <TextInput
                id="rbac-launch-url"
                labelText="Launch URL"
                helperText="Where the launcher tile opens the application."
                value={draft.launchUrl}
                disabled={disabled}
                onChange={(event) => update("launchUrl", event.target.value)}
              />
              <TextInput
                id="rbac-authenticated-launch-url"
                labelText="Authenticated launch URL (optional)"
                helperText="Backend login URL used for cross-application SSO."
                value={draft.authenticatedLaunchUrl}
                disabled={disabled}
                onChange={(event) => update("authenticatedLaunchUrl", event.target.value)}
              />
              <TextInput
                id="rbac-sort-order"
                labelText="Sort order"
                helperText="Lower numbers appear first."
                type="number"
                value={String(draft.sortOrder)}
                disabled={disabled}
                onChange={(event) => update("sortOrder", Number(event.target.value) || 0)}
              />
              <Dropdown
                id="rbac-launch-mode"
                titleText="Launch mode"
                label="Select launch mode"
                items={["internal", "new_tab", "same_tab"] as const}
                itemToString={(item) => (item ? launchModeLabels[item] : "")}
                selectedItem={draft.launchMode}
                disabled={disabled}
                onChange={({ selectedItem }) => selectedItem && update("launchMode", selectedItem)}
              />
              <Toggle
                id="rbac-display-in-launcher"
                labelText="Show in the launcher"
                toggled={draft.displayInLauncher}
                disabled={disabled}
                onToggle={(value) => update("displayInLauncher", value)}
              />
              <Toggle
                id="rbac-display-in-side-nav"
                labelText="Show in the side navigation"
                toggled={draft.displayInSideNav}
                disabled={disabled}
                onToggle={(value) => update("displayInSideNav", value)}
              />
              <div>
                <Toggle
                  id="rbac-enabled"
                  labelText="Enabled"
                  toggled={draft.enabled}
                  disabled={disabled || system?.lifecycleStatus !== "active"}
                  onToggle={(value) => update("enabled", value)}
                />
                <p className="rbac-hint">Only active applications can be enabled or disabled.</p>
              </div>
            </div>
            <p className="rbac-hint">
              The launcher tile appears when the application is active, “Show in the launcher” is
              on, and the signed-in user holds one of its access roles.
            </p>
          </TabPanel>
          <TabPanel className="rbac-subtab-panel">
            <div className="rbac-form-grid">
              <div className="rbac-form-grid__wide">
                <TextArea
                  id="rbac-navigation"
                  labelText="Navigation JSON (optional)"
                  helperText="A JSON array of module entries. Each entry may set displayInLauncher and displayInSideNav."
                  rows={10}
                  value={draft.navigation}
                  invalid={Boolean(navigationPreview.error)}
                  invalidText={navigationPreview.error}
                  disabled={disabled}
                  onChange={(event) => update("navigation", event.target.value)}
                />
                {!navigationPreview.error && draft.navigation.trim() ? (
                  <div className="rbac-navigation-preview" aria-live="polite">
                    <span>Launcher preview</span>
                    <div className="rbac-tag-list">
                      {navigationPreview.launcherItems.length ? (
                        navigationPreview.launcherItems.map((item) => (
                          <Tag key={`launcher-${item.id}`} type="blue">
                            {item.label}
                          </Tag>
                        ))
                      ) : (
                        <small>No module launcher entries.</small>
                      )}
                    </div>
                    <span>Side navigation preview</span>
                    <div className="rbac-tag-list">
                      {navigationPreview.sideNavItems.length ? (
                        navigationPreview.sideNavItems.map((item) => (
                          <Tag key={`sidenav-${item.id}`} type="gray">
                            {item.label}
                          </Tag>
                        ))
                      ) : (
                        <small>No visible side-navigation entries.</small>
                      )}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </section>
  );
}
