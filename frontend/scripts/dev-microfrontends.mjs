/* global console, process */
import { spawn } from "node:child_process";

const appScripts = {
  announcements: "dev:mf:announcements",
  audit: "dev:mf:audit",
  clients: "dev:mf:clients",
  "data-validation": "dev:mf:data-validation",
  "data-visualizer": "dev:mf:data-visualizer",
  documents: "dev:mf:documents",
  "e-services": "dev:mf:e-services",
  email: "dev:mf:email",
  "issue-tracker": "dev:mf:issue-tracker",
  "report-browser": "dev:mf:report-browser",
 "report-scheduler": "dev:mf:report-scheduler",
  rbac: "dev:mf:rbac",
  surveillance: "dev:mf:surveillance",
  users: "dev:mf:users",
  utilities: "dev:mf:utilities",
};

const previewScripts = Object.fromEntries(
  Object.entries(appScripts).map(([app, script]) => [app, script.replace("dev:mf:", "preview:mf:")]),
);

const args = process.argv.slice(2);
const previewMode = args.includes("--preview");
const requestedApps = args.filter((arg) => arg !== "--preview");
const scriptMap = previewMode ? previewScripts : appScripts;
const selectedApps = requestedApps.includes("all") || requestedApps.length === 0
  ? Object.keys(appScripts)
  : requestedApps;

const scripts = ["dev:shell", ...selectedApps.map((app) => scriptMap[app]).filter(Boolean)];
const unknownApps = selectedApps.filter((app) => !appScripts[app]);
const children = [];

if (unknownApps.length > 0) {
  console.error(`Unknown microfrontend app(s): ${unknownApps.join(", ")}`);
  console.error(`Known apps: ${Object.keys(appScripts).join(", ")}`);
  process.exit(1);
}

function start(script) {
  const child = spawn("npm", ["run", script], {
    env: {
      ...process.env,
      VITE_SINGLE_SPA_ORCHESTRATION: process.env.VITE_SINGLE_SPA_ORCHESTRATION ?? "true",
      VITE_MICROFRONTEND_MODE: process.env.VITE_MICROFRONTEND_MODE ?? "remote",
    },
    stdio: "inherit",
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      return;
    }

    if (code && code !== 0) {
      stopAll();
      process.exit(code);
    }
  });

  children.push(child);
}

function stopAll() {
  for (const child of children) {
    child.kill("SIGTERM");
  }
}

process.on("SIGINT", () => {
  stopAll();
  process.exit(130);
});

process.on("SIGTERM", () => {
  stopAll();
  process.exit(143);
});

console.log(`Starting frontend ${previewMode ? "preview" : "dev"} processes: ${scripts.join(", ")}`);

for (const script of scripts) {
  start(script);
}
