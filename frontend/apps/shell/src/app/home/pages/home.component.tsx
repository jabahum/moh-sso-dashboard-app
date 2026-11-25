import {
  Application,
  Apps,
  Bullhorn,
  DocumentView,
  Email,
  Security,
  Time,
  UserFollow,
  UserMultiple,
} from "@carbon/react/icons";
import { Tile, Tag, Search, InlineLoading } from "@carbon/react";
import { lazy, Suspense, useMemo, useState, type ReactNode } from "react";
import { useDispatch, useSelector } from "react-redux";
import { useNavigate } from "react-router-dom";
import "./home.scss";

import { EmptyState, PageShell, useHeaderPanel } from "@moh-sso/ui";
import { ApplicationTile } from "@/app/home/components/app/ApplicationTile";
import { QuickAction } from "@/app/home/components/quick-action/quick-action.component";
import { SignalTile } from "@/app/home/components/signal-tile/signal-tile.component";
import {
  useGetAdminOverviewClientsQuery,
  useGetAdminOverviewFailedLoginsQuery,
  useGetAdminOverviewUsersQuery,
} from "@/app/home/api/admin-overview.api";
import { useGetUnreadNotificationsCountQuery } from "@/app/api";
import { NotificationsTable } from "@/app/notifications/components/NotificationsTable";
import { PERMISSIONS, selectUser, useAuthorization } from "@moh-sso/auth";
import { setActiveClient } from "@moh-sso/state";
import { buildAccessibleLauncherEntries } from "@/app/access/accessClients";

const ClientFormPanel = lazy(() =>
  import("@moh-sso/clients").then((module) => ({ default: module.ClientFormPanel })),
);
const UserFormPanel = lazy(() =>
  import("@moh-sso/users").then((module) => ({ default: module.UserFormPanel })),
);
const ManageAnnouncementsPanel = lazy(() =>
  import("@moh-sso/announcements").then((module) => ({ default: module.ManageAnnouncementsPanel })),
);

const MAX_QUICK_ACTIONS = 6;

function normalizePortalPath(href?: string): string | undefined {
  if (!href || /^https?:\/\//i.test(href)) return href;
  // The portal's own launch URL ("/", or legacy "/portal") opens the app catalogue.
  if (href === "/" || href === "/portal") return "/apps";
  // Stored legacy /portal/... paths map onto the root-served shell.
  return href.startsWith("/portal/") ? href.replace(/^\/portal/, "") : href;
}

function lazyPanel(content: ReactNode) {
  return <Suspense fallback={<InlineLoading description="Loading panel…" />}>{content}</Suspense>;
}

function formatLastLogin(value?: string | null) {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(
    date,
  );
}

export default function HomePage() {
  const { openPanel } = useHeaderPanel();
  const dispatch = useDispatch();
  const navigate = useNavigate();
  const { accessibleSystems, can } = useAuthorization();
  const user = useSelector(selectUser);
  const [search, setSearch] = useState("");
  const visibleClients = useMemo(
    () =>
      buildAccessibleLauncherEntries({
        accessibleSystems,
        can: (permission) => can(permission as never),
      }),
    [accessibleSystems, can],
  );
  const filteredClients = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    return visibleClients.filter((client) =>
      `${client.name} ${client.description ?? ""}`.toLocaleLowerCase().includes(query),
    );
  }, [visibleClients, search]);
  const lastLogin = formatLastLogin(user?.lastLoginAt);
  const displayName = user ? user.firstName || user.username : "";

  const canReadUsers = can(PERMISSIONS.usersRead);
  const canReadClients = can(PERMISSIONS.clientsRead);
  const canReadAudit = can(PERMISSIONS.auditRead);
  const canReadNotifications = can(PERMISSIONS.notificationsRead);

  // Ordered by priority; only the first MAX_QUICK_ACTIONS the user may perform are shown.
  const actions = [
    {
      id: "user",
      allowed: can(PERMISSIONS.usersWrite),
      icon: <UserFollow size={20} />,
      label: "Add user",
      description: "Create an account and set up their access.",
      onClick: () =>
        openPanel({
          title: "Create user",
          content: lazyPanel(<UserFormPanel mode="create" />),
          size: "md",
        }),
    },
    {
      id: "client",
      allowed: can(PERMISSIONS.clientsWrite),
      icon: <Application size={20} />,
      label: "Onboard client",
      description: "Connect an application to Ministry sign-in.",
      onClick: () =>
        openPanel({
          title: "Create client",
          content: lazyPanel(<ClientFormPanel mode="create" />),
          size: "md",
        }),
    },
    {
      id: "announcement",
      allowed: can(PERMISSIONS.announcementsWrite),
      icon: <Bullhorn size={20} />,
      label: "Post announcement",
      description: "Draft an update for portal users.",
      onClick: () =>
        openPanel({
          title: "Create announcement",
          content: lazyPanel(<ManageAnnouncementsPanel mode="create" />),
          size: "lg",
        }),
    },
    {
      id: "users",
      allowed: canReadUsers,
      icon: <UserMultiple size={20} />,
      label: "Manage users",
      description: "Find accounts and enable, disable, or update them.",
      onClick: () => navigate("/admin/users"),
    },
    {
      id: "access",
      allowed: can(PERMISSIONS.rbacRead),
      icon: <Security size={20} />,
      label: "Manage access",
      description: "Review application roles and permissions.",
      onClick: () => navigate("/admin/rbac"),
    },
    {
      id: "audit",
      allowed: canReadAudit,
      icon: <DocumentView size={20} />,
      label: "View audit logs",
      description: "Trace sign-ins and administrative changes.",
      onClick: () => navigate("/admin/audit-logs"),
    },
    {
      id: "clients",
      allowed: canReadClients,
      icon: <Apps size={20} />,
      label: "Manage clients",
      description: "Review connected applications and their settings.",
      onClick: () => navigate("/admin/clients"),
    },
    {
      id: "deliveries",
      allowed: canReadNotifications,
      icon: <Email size={20} />,
      label: "Notification deliveries",
      description: "Check failed or pending email and SMS deliveries.",
      onClick: () => navigate("/admin/notifications/deliveries"),
    },
  ]
    .filter((action) => action.allowed)
    .slice(0, MAX_QUICK_ACTIONS);

  // Fixed for the lifetime of the page so the query key stays stable.
  const [failedLoginsRange] = useState(() => {
    const to = new Date();
    const from = new Date(to.getTime() - 24 * 60 * 60 * 1000);
    return { from: from.toISOString(), to: to.toISOString() };
  });
  const usersSummary = useGetAdminOverviewUsersQuery(undefined, { skip: !canReadUsers });
  const clientsSummary = useGetAdminOverviewClientsQuery(undefined, { skip: !canReadClients });
  const failedLogins = useGetAdminOverviewFailedLoginsQuery(failedLoginsRange, {
    skip: !canReadAudit,
  });
  const unreadNotifications = useGetUnreadNotificationsCountQuery(undefined, {
    skip: !canReadNotifications,
  });

  // A failed request renders "—" in its tile rather than breaking the page.
  const signals = [
    {
      id: "users",
      allowed: canReadUsers,
      label: "Users",
      href: "/admin/users",
      loading: usersSummary.isLoading,
      value: usersSummary.isError ? null : usersSummary.data?.total,
    },
    {
      id: "disabled-users",
      allowed: canReadUsers,
      label: "Disabled users",
      href: "/admin/users",
      loading: usersSummary.isLoading,
      value: usersSummary.isError ? null : usersSummary.data?.disabled,
    },
    {
      id: "clients",
      allowed: canReadClients,
      label: "Client applications",
      href: "/admin/clients",
      loading: clientsSummary.isLoading,
      value: clientsSummary.isError ? null : clientsSummary.data?.total,
      helperText:
        !clientsSummary.isError && clientsSummary.data?.disabled
          ? `${clientsSummary.data.disabled.toLocaleString()} disabled`
          : undefined,
    },
    {
      id: "failed-logins",
      allowed: canReadAudit,
      label: "Failed sign-ins (24h)",
      href: "/admin/audit-logs",
      loading: failedLogins.isLoading,
      value: failedLogins.isError ? null : failedLogins.data,
    },
    {
      id: "unread",
      allowed: canReadNotifications,
      label: "Unread notifications",
      loading: unreadNotifications.isLoading,
      value: unreadNotifications.isError ? null : unreadNotifications.data,
    },
  ].filter((signal) => signal.allowed);

  const hasAdminSections = actions.length > 0 || signals.length > 0;

  const applicationsSection = (
    <section
      className={`home-applications${hasAdminSections ? " home-applications--secondary" : ""}`}
      aria-labelledby="home-applications-title"
    >
      <header className="home-section-heading">
        <div>
          <div className="home-section-heading__title">
            <h4 id="home-applications-title">Your applications</h4>
            <Tag type="cool-gray" size="sm">
              {visibleClients.length}
            </Tag>
          </div>
          {(visibleClients.length > 0 || !hasAdminSections) && (
            <p>Open the tools available to your account.</p>
          )}
        </div>
        {visibleClients.length > 0 && (
          <Search
            id="home-application-search"
            size="md"
            labelText="Search your applications"
            placeholder="Find an application"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            onClear={() => setSearch("")}
          />
        )}
      </header>
      {visibleClients.length === 0 ? (
        hasAdminSections ? (
          <p className="home-applications__empty">
            No applications are assigned to your account yet.
          </p>
        ) : (
          <EmptyState
            title="No applications assigned"
            description="Applications will appear here when access is assigned to your account."
          />
        )
      ) : (
        <>
          <p className="home-applications__results" role="status">
            {search.trim()
              ? `${filteredClients.length} of ${visibleClients.length} applications`
              : `${visibleClients.length} applications available`}
          </p>
          {filteredClients.length === 0 ? (
            <EmptyState
              title="No matching applications"
              description="Try a different application name or clear your search."
              primaryAction={{ label: "Clear search", onClick: () => setSearch("") }}
            />
          ) : (
            <div className="home-applications__grid">
              {filteredClients.map((client) => (
                <ApplicationTile
                  key={client.clientId}
                  clientId={client.clientId}
                  name={client.name}
                  description={client.description}
                  enabled={client.enabled}
                  rootUrl={normalizePortalPath(client.baseUrl)}
                  launchMode={
                    (client.attributes?.["ui.launchMode"] as "internal" | "new_tab" | "same_tab") ||
                    "internal"
                  }
                  onLaunch={() =>
                    dispatch(
                      setActiveClient(client.attributes?.["ui.parentClientId"] || client.clientId),
                    )
                  }
                />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );

  return (
    <div className="home-page">
      <PageShell
        title="Admin Overview"
        subtitle={
          displayName
            ? `Welcome back, ${displayName}. Manage people, applications, and access from here.`
            : "Manage people, applications, and access from here."
        }
      >
        {!user ? (
          <InlineLoading description="Loading your workspace…" />
        ) : (
          <div className="home-context">
            <Tag type="blue" size="sm">
              {user.isAdmin ? "Administrator" : "Signed in"}
            </Tag>
            {user.email && <span className="home-context__item">{user.email}</span>}
            {lastLogin && (
              <span className="home-context__item">
                <Time size={16} aria-hidden="true" />
                Last sign-in: {lastLogin}
              </span>
            )}
          </div>
        )}

        {!hasAdminSections && applicationsSection}

        {signals.length > 0 && (
          <section aria-labelledby="home-signals-title">
            <h4 id="home-signals-title" className="cds--visually-hidden">
              At a glance
            </h4>
            <div className="home-signals">
              {signals.map(({ id, label, href, loading, value, ...rest }) => (
                <SignalTile
                  key={id}
                  label={label}
                  href={href}
                  loading={loading}
                  value={value}
                  helperText={"helperText" in rest ? rest.helperText : undefined}
                />
              ))}
            </div>
          </section>
        )}

        {actions.length > 0 && (
          <section aria-labelledby="home-actions-title">
            <header className="home-section-heading">
              <div>
                <h4 id="home-actions-title">Quick actions</h4>
                <p>Common tasks for your administrative access.</p>
              </div>
            </header>
            <div className="home-actions">
              {actions.map(({ id, icon, label, description, onClick }) => (
                <QuickAction
                  key={id}
                  icon={icon}
                  label={label}
                  description={description}
                  onClick={onClick}
                />
              ))}
            </div>
          </section>
        )}

        {canReadNotifications && (
          <Tile className="home-notifications">
            <NotificationsTable />
          </Tile>
        )}

        {hasAdminSections && applicationsSection}
      </PageShell>
    </div>
  );
}
