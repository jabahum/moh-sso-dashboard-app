// src/components/footer/PublicFooter.tsx
import "./PublicFooter.scss";

export type FooterVersionInfo = {
  frontendVersion?: string;
  backendVersion?: string;
  backendUnavailable?: boolean;
};

function formatVersion(version?: string) {
  if (!version) {
    return undefined;
  }

  return version === "dev" || version.startsWith("v") ? version : `v${version}`;
}

export function PublicFooter({
  frontendVersion,
  backendVersion,
  backendUnavailable = false,
}: FooterVersionInfo = {}) {
  const versionLabel = formatVersion(
    frontendVersion || (backendUnavailable ? undefined : backendVersion),
  );

  return (
    <footer className="public-footer">
      <div className="footer-bottom">
        <span>© {new Date().getFullYear()} Ministry of Health – Uganda</span>
        {versionLabel && (
          <span className="footer-version" aria-label="Application version">
            {versionLabel}
          </span>
        )}
      </div>
    </footer>
  );
}
