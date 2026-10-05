/* global process */
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));

const apps = [
"announcements", "audit", "clients", "data-validation", "data-visualizer",
  "documents", "e-services", "email", "issue-tracker", "report-browser", "report-scheduler",
  "rbac", "surveillance", "users", "utilities"
];

const packages = ["api", "auth", "config", "microfrontend", "state", "types", "ui", "utils"];
const versionedImports = process.env.FRONTEND_VERSIONED_IMPORTS === "true";
const assetBaseUrl = (process.env.FRONTEND_ASSET_BASE_URL ?? "").replace(/\/$/, "");

function readVersion(group, name) {
  const packageJson = JSON.parse(readFileSync(join(root, group, name, "package.json"), "utf8"));
  return packageJson.version;
}

function appUrl(app) {
  if (!assetBaseUrl) {
    return `/mf/${app}/single-spa.js`;
  }

  if (!versionedImports) {
    return `${assetBaseUrl}/mf/${app}/single-spa.js`;
  }

  return `${assetBaseUrl}/mf/${app}/${readVersion("apps", app)}/single-spa.js`;
}

function packageUrl(pkg) {
  if (!assetBaseUrl) {
    return `/packages/${pkg}/index.js`;
  }

  if (!versionedImports) {
    return `${assetBaseUrl}/packages/${pkg}/index.js`;
  }

  return `${assetBaseUrl}/packages/${pkg}/${readVersion("packages", pkg)}/index.js`;
}

// IMPORTANT: every esm.sh package with a React peer dependency MUST carry
// `?external=react,react-dom` (or `?external=react`). Without it, esm.sh
// bundles its OWN pinned React into the package (it emits imports from
// `/react@^19.2.0?...` instead of the bare `react` specifier), producing a
// second React instance on the page. That breaks shared-context hooks — e.g.
// @carbon/react's `usePrefix` calls `useContext` on a null dispatcher and the
// micro-frontend dies on mount with "Cannot read properties of null
// (reading 'useContext')". The `external` query forces esm.sh to emit a bare
// `from "react"`, which this import map then resolves to the single shared
// copy below.
const imports = {
  "@carbon/react": "https://esm.sh/@carbon/react@1.95.0?external=react,react-dom",
  "@carbon/react/icons": "https://esm.sh/@carbon/react@1.95.0/icons?external=react,react-dom",
  "@reduxjs/toolkit": "https://esm.sh/@reduxjs/toolkit@2.11.2?external=react",
  react: "https://esm.sh/react@19.2.0",
  "react/jsx-runtime": "https://esm.sh/react@19.2.0/jsx-runtime?external=react",
  "react/jsx-dev-runtime": "https://esm.sh/react@19.2.0/jsx-dev-runtime?external=react",
  "react-dom": "https://esm.sh/react-dom@19.2.0?external=react",
  "react-dom/client": "https://esm.sh/react-dom@19.2.0/client?external=react",
  "react-redux": "https://esm.sh/react-redux@9.2.0?external=react,react-dom",
  "react-router-dom": "https://esm.sh/react-router-dom@7.9.6?external=react,react-dom",
  "single-spa": "https://esm.sh/single-spa@6.0.3",
  "single-spa-react": "https://esm.sh/single-spa-react@6.0.2?external=react,react-dom",
};

for (const app of apps) {
  imports[`@moh-sso/${app}`] = appUrl(app);
}

for (const pkg of packages) {
  imports[`@moh-sso/${pkg}`] = packageUrl(pkg);
}

writeFileSync(join(root, "public", "import-map.json"), `${JSON.stringify({ imports }, null, 2)}\n`);
