/* global console, process */
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function collectPackages(group) {
  const groupPath = join(root, group);
  const entries = {};

  if (!existsSync(groupPath)) {
    return entries;
  }

  for (const dirent of readdirSync(groupPath, { withFileTypes: true })) {
    if (!dirent.isDirectory()) {
      continue;
    }

    const packagePath = join(groupPath, dirent.name, "package.json");
    if (!existsSync(packagePath)) {
      continue;
    }

    const packageJson = readJson(packagePath);
    entries[dirent.name] = {
      name: packageJson.name,
      version: packageJson.version,
    };
  }

  return Object.fromEntries(Object.entries(entries).sort(([a], [b]) => a.localeCompare(b)));
}

const manifest = {
  generatedAt: new Date().toISOString(),
  service: {
    name: "moh-sso-dashboard-frontend",
    version:
      process.env.VITE_APP_VERSION || readJson(join(root, "apps", "shell", "package.json")).version,
    commit: process.env.FRONTEND_COMMIT || "none",
    buildTime: process.env.VITE_BUILD_TIME || "unknown",
  },
  apps: collectPackages("apps"),
  packages: collectPackages("packages"),
};

writeFileSync(
  join(root, "public", "version-manifest.json"),
  `${JSON.stringify(manifest, null, 2)}\n`,
);

console.log("Generated public/version-manifest.json");
