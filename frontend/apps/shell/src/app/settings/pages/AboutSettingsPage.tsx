import { Button, InlineNotification, Tile } from "@carbon/react";
import { Information, Launch, Security } from "@carbon/react/icons";

import { useVersionInfo } from "@/app/version/useVersionInfo";

const PLATFORM_NAME = "MOH Integrated Health Portal";

export default function AboutSettingsPage() {
  const versionInfo = useVersionInfo();

  return (
    <div className="settings-page">
      <div className="settings-page__header">
        <h2>About the portal</h2>
        <p className="settings-page__description">
          Review version information, support contacts, privacy notices, and safe operational
          details.
        </p>
      </div>

      <Tile className="settings-card">
        <div className="settings-card__heading">
          <Information size={20} />
          <h3>Version information</h3>
        </div>

        <dl className="settings-about-list">
          <dt>Platform</dt>
          <dd>{PLATFORM_NAME}</dd>
          <dt>Version</dt>
          <dd>{versionInfo.frontend.version || "dev"}</dd>
          <dt>Frontend build time</dt>
          <dd>{versionInfo.frontend.buildTime ?? "Not available"}</dd>
          <dt>Backend build time</dt>
          <dd>{versionInfo.backend?.buildTime ?? "Not available"}</dd>
        </dl>

        {!versionInfo.isLoading && !versionInfo.backend ? (
          <InlineNotification
            kind="warning"
            lowContrast
            hideCloseButton
            title="Backend build details unavailable"
            subtitle="The portal is still usable, but backend build details could not be loaded."
          />
        ) : null}
      </Tile>

      <Tile className="settings-card">
        <div className="settings-card__heading">
          <Security size={20} />
          <h3>Privacy and monitoring</h3>
        </div>

        <p className="settings-card__text">
          Authorized activity may be logged for security, audit, support, and operational
          continuity. Access to applications is controlled by the MOH identity provider and portal
          RBAC policies.
        </p>

        <div className="settings-card__actions">
          <Button kind="ghost" href="mailto:admin@moh.go.ug" renderIcon={Launch}>
            Contact support
          </Button>
        </div>
      </Tile>
    </div>
  );
}
