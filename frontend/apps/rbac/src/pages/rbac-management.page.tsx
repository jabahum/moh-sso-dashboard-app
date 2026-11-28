import { useMemo, useState, type ReactNode } from "react";
import {
  Button,
  ComboBox,
  InlineLoading,
  Tab,
  TabList,
  TabPanel,
  TabPanels,
  Tabs,
  Tag,
} from "@carbon/react";
import { Add, Application, Close, Earth } from "@carbon/react/icons";

import {
  useGetRbacSystemQuery,
  useListRbacPermissionsQuery,
  useListRbacSystemsQuery,
} from "../api";
import { PERMISSIONS, useAuthorization } from "@moh-sso/auth";
import { PageLoading, PageShell } from "@moh-sso/ui";

import { EffectiveAccessPanel } from "../components/EffectiveAccessPanel";
import { GovernanceToolsPanel } from "../components/GovernanceToolsPanel";
import { IdentitySyncPanel } from "../components/IdentitySyncPanel";
import { ServiceIdentitiesPanel } from "../components/ServiceIdentitiesPanel";
import { SyncDriftPanel } from "../components/SyncDriftPanel";
import { AccessRolesPanel } from "../components/workspace/AccessRolesPanel";
import { ApplicationSettingsPanel } from "../components/workspace/ApplicationSettingsPanel";
import { UnsavedChangesProvider, useUnsavedGuard } from "../components/workspace/feedback";
import { GroupsPanel } from "../components/workspace/GroupsPanel";
import { LifecyclePanel } from "../components/workspace/LifecyclePanel";
import { RealmPermissionsPanel } from "../components/workspace/RealmPermissionsPanel";
import { SystemRolesPanel } from "../components/workspace/SystemRolesPanel";
import {
  groupPermissions,
  lifecycleLabels,
  lifecycleTagType,
} from "../components/workspace/workspace-helpers";
import "./rbac-management.page.scss";

const applicationSections = [
  { id: "settings", label: "Settings" },
  { id: "lifecycle", label: "Lifecycle" },
  { id: "access-roles", label: "Access roles" },
  { id: "system-roles", label: "System roles" },
  { id: "identity-sync", label: "Identity sync" },
  { id: "machine-clients", label: "Machine clients" },
] as const;
type ApplicationSection = (typeof applicationSections)[number]["id"];

const portalSections = [
  { id: "realm-roles", label: "Realm permissions" },
  { id: "groups", label: "Groups" },
  { id: "effective-access", label: "Effective access" },
  { id: "sync", label: "Sync & drift" },
  { id: "governance", label: "Governance" },
] as const;
type PortalSection = (typeof portalSections)[number]["id"];

type Scope = "application" | "portal";

const scopes: { id: Scope; label: string; description: string; icon: typeof Application }[] = [
  {
    id: "application",
    label: "This application",
    description: "Settings, roles and identity sync for one application",
    icon: Application,
  },
  {
    id: "portal",
    label: "Portal-wide",
    description: "Realm permissions, groups, access review and governance",
    icon: Earth,
  },
];

const pageTitle = "RBAC Management";
const pageSubtitle = "Manage who can use each application and what they can do across the portal.";

export default function RbacManagementPage() {
  return (
    <UnsavedChangesProvider>
      <RbacWorkspace />
    </UnsavedChangesProvider>
  );
}

function RbacWorkspace() {
  const { can } = useAuthorization();
  const guard = useUnsavedGuard();
  const canWriteSystems = can(PERMISSIONS.rbacWrite);
  const canWriteRoles = can(PERMISSIONS.rbacRolesWrite);
  const canWritePermissions = can(PERMISSIONS.rbacPermissionsWrite);

  const { data: systems = [], isLoading: systemsLoading } = useListRbacSystemsQuery();
  const { data: permissions = [], isLoading: permissionsLoading } = useListRbacPermissionsQuery();
  const categories = useMemo(() => groupPermissions(permissions), [permissions]);

  const [scope, setScope] = useState<Scope>("application");
  const [applicationSection, setApplicationSection] = useState<ApplicationSection>("settings");
  const [portalSection, setPortalSection] = useState<PortalSection>("realm-roles");
  // Sections stay mounted once opened, so their filters and results survive tab switches.
  const [opened, setOpened] = useState<Set<string>>(() => new Set(["settings"]));
  const [selectedClientId, setSelectedClientId] = useState("");
  const [registering, setRegistering] = useState(false);

  const activeClientId = registering ? "" : selectedClientId || systems[0]?.clientId || "";
  const { currentData: system, isFetching: systemLoading } = useGetRbacSystemQuery(activeClientId, {
    skip: !activeClientId,
  });
  const selectedSystem = systems.find((item) => item.clientId === activeClientId) ?? null;

  const open = (section: string) =>
    setOpened((current) => (current.has(section) ? current : new Set(current).add(section)));

  const showApplicationSection = (section: ApplicationSection) => {
    setApplicationSection(section);
    open(section);
  };
  const showPortalSection = (section: PortalSection) => {
    setPortalSection(section);
    open(section);
  };

  const changeScope = (next: Scope) => {
    if (next === scope) return;
    void guard(() => {
      setScope(next);
      open(next === "application" ? applicationSection : portalSection);
    });
  };

  const selectApplication = (clientId: string) =>
    guard(() => {
      setRegistering(false);
      setSelectedClientId(clientId);
    });

  const startRegistration = () =>
    guard(() => {
      setRegistering(true);
      showApplicationSection("settings");
    });

  if (systemsLoading || permissionsLoading) {
    return <PageLoading description="Loading RBAC mappings..." />;
  }

  const needsApplication = (content: ReactNode) => {
    if (registering) {
      return (
        <p className="rbac-empty rbac-empty--card">
          Register the new application in Settings first. This section becomes available once it is
          saved.
        </p>
      );
    }
    if (!activeClientId) {
      return (
        <p className="rbac-empty rbac-empty--card">
          No applications are registered yet.
          {canWriteSystems ? " Use “Register application” to add one." : ""}
        </p>
      );
    }
    if (!system) return <InlineLoading description="Loading application details..." />;
    return content;
  };

  const applicationContent: Record<ApplicationSection, () => ReactNode> = {
    settings: () =>
      registering ? (
        <ApplicationSettingsPanel
          key="new-application"
          registering
          canWriteSystems={canWriteSystems}
          onRegistered={(clientId) => {
            setRegistering(false);
            setSelectedClientId(clientId);
          }}
        />
      ) : (
        needsApplication(
          <ApplicationSettingsPanel
            key={activeClientId}
            system={system}
            registering={false}
            canWriteSystems={canWriteSystems}
            onRegistered={setSelectedClientId}
          />,
        )
      ),
    lifecycle: () =>
      needsApplication(
        system && (
          <LifecyclePanel
            key={system.clientId}
            system={system}
            canWriteSystems={canWriteSystems}
            canWriteRoles={canWriteRoles}
          />
        ),
      ),
    "access-roles": () =>
      needsApplication(
        system && (
          <AccessRolesPanel key={system.clientId} system={system} canWriteRoles={canWriteRoles} />
        ),
      ),
    "system-roles": () =>
      needsApplication(
        system && (
          <SystemRolesPanel
            key={system.clientId}
            system={system}
            categories={categories}
            canWriteRoles={canWriteRoles}
            canWritePermissions={canWritePermissions}
          />
        ),
      ),
    "identity-sync": () => (
      <IdentitySyncPanel
        clientId={registering ? "" : activeClientId}
        canWrite={canWriteSystems && canWriteRoles}
      />
    ),
    "machine-clients": () => (
      <ServiceIdentitiesPanel
        clientId={registering ? "" : activeClientId}
        canWrite={canWriteSystems && canWriteRoles}
      />
    ),
  };

  const portalContent: Record<PortalSection, () => ReactNode> = {
    "realm-roles": () => (
      <RealmPermissionsPanel categories={categories} canWritePermissions={canWritePermissions} />
    ),
    groups: () => (
      <GroupsPanel
        systems={systems}
        categories={categories}
        canWriteRoles={canWriteRoles}
        canWritePermissions={canWritePermissions}
      />
    ),
    "effective-access": () => <EffectiveAccessPanel />,
    sync: () => <SyncDriftPanel />,
    governance: () => <GovernanceToolsPanel activeClientId={activeClientId} />,
  };

  const lifecycleStatus = system?.lifecycleStatus;

  return (
    <div className="rbac-page">
      <PageShell
        title={pageTitle}
        subtitle={pageSubtitle}
        actions={
          <div className="rbac-scope-switch" role="group" aria-label="Workspace scope">
            {scopes.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  type="button"
                  className="rbac-scope-switch__option"
                  aria-pressed={scope === item.id}
                  title={item.description}
                  onClick={() => changeScope(item.id)}
                >
                  <Icon size={16} aria-hidden="true" />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        }
      >
        <div className="rbac-scope" hidden={scope !== "application"}>
          <div className="rbac-app-bar" role="group" aria-label="Application">
            <ComboBox
              id="rbac-selected-application"
              className="rbac-app-bar__selector"
              titleText="Selected application"
              placeholder={registering ? "New application" : "Find an application"}
              items={systems}
              itemToString={(item) => (item ? `${item.displayName} (${item.clientId})` : "")}
              selectedItem={registering ? null : selectedSystem}
              onChange={({ selectedItem }) => {
                if (selectedItem && selectedItem.clientId !== activeClientId)
                  void selectApplication(selectedItem.clientId);
              }}
              shouldFilterItem={({ item, inputValue }) =>
                `${item.displayName} ${item.clientId}`
                  .toLowerCase()
                  .includes((inputValue ?? "").toLowerCase())
              }
            />
            <div className="rbac-app-bar__status" aria-live="polite">
              {registering ? (
                <Tag type="gray">New application</Tag>
              ) : lifecycleStatus ? (
                <Tag type={lifecycleTagType[lifecycleStatus]}>
                  <span className="cds--visually-hidden">Status: </span>
                  {lifecycleLabels[lifecycleStatus]}
                </Tag>
              ) : systemLoading ? (
                <InlineLoading description="Loading" />
              ) : null}
            </div>
            <div className="rbac-app-bar__actions">
              {registering ? (
                <Button
                  kind="ghost"
                  size="md"
                  renderIcon={Close}
                  onClick={() => void guard(() => setRegistering(false))}
                >
                  Cancel registration
                </Button>
              ) : (
                canWriteSystems && (
                  <Button
                    kind="tertiary"
                    size="md"
                    renderIcon={Add}
                    onClick={() => void startRegistration()}
                  >
                    Register application
                  </Button>
                )
              )}
            </div>
          </div>
          <Tabs
            selectedIndex={applicationSections.findIndex((item) => item.id === applicationSection)}
            onChange={({ selectedIndex }) => {
              const next = applicationSections[selectedIndex].id;
              if (next !== applicationSection) void guard(() => showApplicationSection(next));
            }}
          >
            <TabList aria-label="Application sections" className="rbac-section-tabs">
              {applicationSections.map((item) => (
                <Tab key={item.id}>{item.label}</Tab>
              ))}
            </TabList>
            <TabPanels>
              {applicationSections.map((item) => (
                <TabPanel key={item.id} className="rbac-section-panel">
                  {opened.has(item.id) && applicationContent[item.id]()}
                </TabPanel>
              ))}
            </TabPanels>
          </Tabs>
        </div>

        <div className="rbac-scope" hidden={scope !== "portal"}>
          <p className="rbac-scope__note">
            These settings apply across the whole portal, not to one application.
          </p>
          <Tabs
            selectedIndex={portalSections.findIndex((item) => item.id === portalSection)}
            onChange={({ selectedIndex }) => {
              const next = portalSections[selectedIndex].id;
              if (next !== portalSection) void guard(() => showPortalSection(next));
            }}
          >
            <TabList aria-label="Portal-wide sections" className="rbac-section-tabs">
              {portalSections.map((item) => (
                <Tab key={item.id}>{item.label}</Tab>
              ))}
            </TabList>
            <TabPanels>
              {portalSections.map((item) => (
                <TabPanel key={item.id} className="rbac-section-panel">
                  {opened.has(item.id) && portalContent[item.id]()}
                </TabPanel>
              ))}
            </TabPanels>
          </Tabs>
        </div>
      </PageShell>
    </div>
  );
}
