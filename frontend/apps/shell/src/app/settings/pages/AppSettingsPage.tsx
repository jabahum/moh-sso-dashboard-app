import { useMemo, useState } from "react";
import { Button, Checkbox, InlineNotification, RadioButton, RadioButtonGroup, Tile } from "@carbon/react";
import { Application, Save } from "@carbon/react/icons";

import { useAuthorization } from "@moh-sso/auth";
import {
  isSafeInternalPath,
  readPortalPreferences,
  savePortalPreferences,
  type PortalPreferences,
} from "../preferences";

function getLaunchPath(system: { launchUrl?: string; clientId: string }) {
  // Legacy launch URLs may still carry the retired /portal prefix.
  const launchPath = system.launchUrl?.replace(/^\/portal(?=\/|$)/, "") || "/";
  if (system.launchUrl && isSafeInternalPath(launchPath)) {
    return launchPath;
  }

  return `/apps/${system.clientId}`;
}

export default function AppSettingsPage() {
  const { accessibleSystems } = useAuthorization();
  const [preferences, setPreferences] = useState<PortalPreferences>(() => readPortalPreferences());
  const [saved, setSaved] = useState(false);

  const systems = useMemo(
    () =>
      accessibleSystems
        .filter((system) => system.displayInLauncher !== false)
        .map((system) => ({
          ...system,
          launchPath: getLaunchPath(system),
        }))
        .sort((a, b) => a.displayName.localeCompare(b.displayName)),
    [accessibleSystems],
  );

  const toggleFavorite = (clientId: string, checked: boolean) => {
    setSaved(false);
    setPreferences((current) => {
      const nextFavorites = checked
        ? Array.from(new Set([...current.favoriteSystemIds, clientId]))
        : current.favoriteSystemIds.filter((id) => id !== clientId);

      return {
        ...current,
        favoriteSystemIds: nextFavorites,
      };
    });
  };

  const setDefaultLanding = (path: string) => {
    setSaved(false);
    setPreferences((current) => ({
      ...current,
      defaultLandingPath: path,
    }));
  };

  const handleSave = () => {
    const allowedPaths = new Set(["/apps/news", ...systems.map((system) => system.launchPath)]);
    const safeDefaultLandingPath = allowedPaths.has(preferences.defaultLandingPath)
      ? preferences.defaultLandingPath
      : "/apps/news";

    savePortalPreferences({
      ...preferences,
      defaultLandingPath: safeDefaultLandingPath,
      favoriteSystemIds: preferences.favoriteSystemIds.filter((id) =>
        systems.some((system) => system.clientId === id),
      ),
    });
    setSaved(true);
  };

  return (
    <div className="settings-page">
      <div className="settings-page__header">
        <h2>Application preferences</h2>
        <p className="settings-page__description">
          Choose favorite applications and a default landing area from systems you can already
          access.
        </p>
      </div>

      <Tile className="settings-card">
        <div className="settings-card__heading">
          <Application size={20} />
          <h3>Accessible applications</h3>
        </div>

        {systems.length === 0 ? (
          <p className="settings-card__text">
            No launcher applications are available for your current access profile.
          </p>
        ) : (
          <div className="settings-app-list">
            {systems.map((system) => (
              <div key={system.clientId} className="settings-app-row">
                <span>
                  <strong>{system.displayName}</strong>
                  <small>{system.launchPath}</small>
                </span>

                <Checkbox
                  id={`settings-favorite-${system.clientId}`}
                  labelText="Favorite"
                  checked={preferences.favoriteSystemIds.includes(system.clientId)}
                  onChange={(_, { checked }) => toggleFavorite(system.clientId, checked)}
                />
              </div>
            ))}
          </div>
        )}
      </Tile>

      <Tile className="settings-card">
        <div className="settings-card__heading">
          <Application size={20} />
          <h3>Default landing page</h3>
        </div>

        <RadioButtonGroup
          legendText="Open this area after sign in"
          name="settings-default-landing"
          valueSelected={preferences.defaultLandingPath}
          onChange={(value) => setDefaultLanding(String(value))}
        >
          <RadioButton id="settings-default-news" value="/apps/news" labelText="News & Updates" />
          {systems.map((system) => (
            <RadioButton
              key={system.clientId}
              id={`settings-default-${system.clientId}`}
              value={system.launchPath}
              labelText={system.displayName}
            />
          ))}
        </RadioButtonGroup>
      </Tile>

      {saved ? (
        <InlineNotification
          kind="success"
          lowContrast
          hideCloseButton
          title="Application preferences saved"
          subtitle="Your launcher preferences were saved on this device."
        />
      ) : null}

      <div className="settings-card__actions">
        <Button kind="primary" renderIcon={Save} onClick={handleSave}>
          Save app preferences
        </Button>
      </div>
    </div>
  );
}
