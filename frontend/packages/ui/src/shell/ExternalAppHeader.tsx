import AppMenu from "../navigation/appmenu/AppMenu.component";
import AppGridContent from "../navigation/appmenu/AppGridContent";
import UserHeader from "./UserHeader.component";

export type ExternalAppLink = {
  id: string;
  name: string;
  /** Absolute HTTP(S) launch URL approved by the application's backend. */
  href: string;
};

export type ExternalAppHeaderProps = {
  appName: string;
  username: string;
  /** Absolute portal launcher URL, including its deployment base path. */
  portalUrl: string;
  /** Supply only applications this user may launch. No portal API is called here. */
  applications?: ExternalAppLink[];
  isLoading?: boolean;
  isError?: boolean;
  onRetry?: () => void;
  onProfile: () => void;
  /** End the local application session and initiate the application's OIDC logout. */
  onLogout: () => void;
};

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return (
      (url.protocol === "https:" || url.protocol === "http:") && !url.username && !url.password
    );
  } catch {
    return false;
  }
}

/** Shared MOH chrome for independently hosted React apps. Requires no portal auth/store/router. */
export function ExternalAppHeader({
  appName,
  username,
  portalUrl,
  applications = [],
  isLoading,
  isError,
  onRetry,
  onProfile,
  onLogout,
}: ExternalAppHeaderProps) {
  if (!isHttpUrl(portalUrl)) {
    throw new Error("ExternalAppHeader requires an absolute HTTP(S) portalUrl without credentials");
  }

  const clients = applications
    .filter((app) => isHttpUrl(app.href))
    .map((app) => ({
      id: app.id,
      clientId: app.id,
      name: app.name,
      baseUrl: app.href,
      enabled: true,
      publicClient: true,
      redirectUris: [],
    }));

  return (
    <UserHeader
      appName={appName}
      username={username}
      onNavigateHome={() => window.location.assign(portalUrl)}
      onProfile={onProfile}
      onLogout={onLogout}
      appMenu={
        <AppMenu>
          {(closeMenu) => (
            <>
              <a className="moh-portal-link" href={portalUrl}>
                All portal applications
              </a>
              <AppGridContent
                clients={clients}
                isLoading={isLoading}
                isError={isError}
                onRetry={onRetry}
                onSelect={closeMenu}
                onOpenClient={(href) => window.location.assign(href)}
              />
            </>
          )}
        </AppMenu>
      }
    />
  );
}
