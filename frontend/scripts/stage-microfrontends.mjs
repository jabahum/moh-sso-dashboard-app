/* global process */
import { copyFileSync, existsSync, mkdirSync, readdirSync, rmSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));

const apps = [
  "announcements",
  "audit",
  "clients",
  "data-validation",
  "data-visualizer",
  "documents",
  "e-services",
  "email",
  "issue-tracker",
  "report-browser", "report-scheduler",
  "rbac",
  "surveillance",
  "users",
  "utilities",
];

const packages = ["api", "auth", "config", "microfrontend", "state", "types", "ui", "utils"];
const basePath = (process.env.FRONTEND_BASE_PATH ?? "/").replace(/^\/?/, "/").replace(/\/$/, "");

function copyRequired(from, to) {
  if (!existsSync(from)) {
    throw new Error(`Missing build artifact: ${from}`);
  }

  mkdirSync(dirname(to), { recursive: true });
  copyFileSync(from, to);
}

function assertRequired(path) {
  if (!existsSync(path)) {
    throw new Error(`Missing build artifact: ${path}`);
  }
}

function copyDirectory(from, to) {
  if (!existsSync(from)) {
    throw new Error(`Missing build directory: ${from}`);
  }

  rmSync(to, { recursive: true, force: true });
  mkdirSync(to, { recursive: true });

  for (const entry of readdirSync(from, { withFileTypes: true })) {
    const source = join(from, entry.name);
    const target = join(to, entry.name);

    if (entry.isDirectory()) {
      if (entry.name !== "node_modules") {
        copyDirectory(source, target);
      }
      continue;
    }

    copyFileSync(source, target);
  }
}

assertRequired(join(root, "apps", "shell", "dist", "index.html"));
copyDirectory(join(root, "apps", "shell", "dist"), join(root, "dist"));

for (const app of apps) {
  assertRequired(join(root, "apps", app, "dist", "single-spa.js"));
  copyDirectory(join(root, "apps", app, "dist"), join(root, "dist", "mf", app));
}

for (const pkg of packages) {
  assertRequired(join(root, "packages", pkg, "dist", "index.js"));
  copyDirectory(join(root, "packages", pkg, "dist"), join(root, "dist", "packages", pkg));
}

copyRequired(
  join(root, "public", "version-manifest.json"),
  join(root, "dist", "version-manifest.json"),
);

if (basePath && basePath !== "/") {
  const baseDir = join(root, "dist", basePath.slice(1));
  rmSync(baseDir, { recursive: true, force: true });
  mkdirSync(baseDir, { recursive: true });

  for (const entry of [
    "assets",
    "mf",
    "packages",
    "config.js",
    "config.production.js",
    "icons",
    "import-map.json",
    "import-map.local.json",
    "index.html",
    "logo.png",
    "manifest.webmanifest",
    "offline.html",
    "sw.js",
    "version-manifest.json",
  ]) {
    const source = join(root, "dist", entry);
    if (existsSync(source)) {
      const target = join(baseDir, entry);

      if (statSync(source).isDirectory()) {
        copyDirectory(source, target);
      } else {
        mkdirSync(dirname(target), { recursive: true });
        copyFileSync(source, target);
      }
    }
  }

  for (const entry of readdirSync(join(root, "dist"), { withFileTypes: true })) {
    if (!entry.isFile() || !/^workbox-[A-Za-z0-9_-]+\.js(\.map)?$/.test(entry.name)) {
      continue;
    }

    copyRequired(join(root, "dist", entry.name), join(baseDir, entry.name));
  }
}
