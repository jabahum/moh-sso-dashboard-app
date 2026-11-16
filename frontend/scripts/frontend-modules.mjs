/* global console, process */
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { frontendRoot, installedModules, moduleSelection } from "./module-source.mjs";

const npm = process.env.NPM || "npm";
const run = (args, env) => execFileSync(npm, args, { cwd: frontendRoot, stdio: "inherit", env });
export function moduleManifest(selection) {
  return {
    name: "moh-installed-frontend-modules",
    private: true,
    dependencies: selection.versions,
  };
}
export function validatedModuleLock(selection) {
  if (!selection.lockfile) return undefined;
  const contents = readFileSync(selection.lockfile, "utf8");
  const lock = JSON.parse(contents);
  const expected = moduleManifest(selection);
  const root = lock.packages?.[""];
  const samePins = (pins) =>
    pins &&
    JSON.stringify(Object.entries(pins).sort()) ===
      JSON.stringify(Object.entries(expected.dependencies).sort());
  if (lock.lockfileVersion !== 3 || root?.name !== expected.name || !samePins(root.dependencies))
    throw new Error("Frontend module lock does not match selected pins; run npm run modules:lock");
  for (const [name, version] of Object.entries(expected.dependencies)) {
    const entry = lock.packages[`node_modules/${name}`];
    if (
      entry?.version !== version ||
      !entry.integrity ||
      entry.link ||
      !entry.resolved?.startsWith("https://")
    )
      throw new Error(`Missing registry integrity for ${name} in frontend module lock`);
  }
  return contents;
}
function lockModules(selection) {
  if (selection.source !== "npm" || !selection.lockfile)
    throw new Error("Select npm modules and configure a lockfile before updating pins");
  const directory = dirname(selection.lockfile);
  mkdirSync(directory, { recursive: true });
  writeFileSync(
    join(directory, "package.json"),
    `${JSON.stringify(moduleManifest(selection), null, 2)}\n`,
  );
  run(
    [
      "install",
      "--package-lock-only",
      "--prefix",
      directory,
      "--workspaces=false",
      "--ignore-scripts",
      "--legacy-peer-deps",
      "--no-audit",
      "--no-fund",
    ],
    process.env,
  );
  validatedModuleLock(selection);
}
export function prepareModules(selection) {
  if (selection.source === "workspace") return;
  const manifest = moduleManifest(selection);
  const locked = validatedModuleLock(selection);
  const path = join(selection.directory, "package.json");
  const installedLock = join(selection.directory, "package-lock.json");
  const unchanged =
    existsSync(path) &&
    JSON.stringify(JSON.parse(readFileSync(path, "utf8"))) === JSON.stringify(manifest) &&
    (!locked || (existsSync(installedLock) && readFileSync(installedLock, "utf8") === locked));
  if (unchanged) {
    try {
      installedModules(selection);
      console.log(`Using installed npm pins for ${selection.environment}`);
      return;
    } catch {
      /* Reconcile an incomplete or stale installation. */
    }
  }
  mkdirSync(selection.directory, { recursive: true });
  // Use a separate npm project; npm workspace links are never overwritten.
  writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
  if (locked) copyFileSync(selection.lockfile, installedLock);
  run(
    [
      locked || (unchanged && existsSync(installedLock)) ? "ci" : "install",
      "--prefix",
      selection.directory,
      "--workspaces=false",
      "--ignore-scripts",
      "--legacy-peer-deps",
      "--no-audit",
      "--no-fund",
    ],
    process.env,
  );
  installedModules(selection);
}
function main() {
  const [action, environment = action === "dev" ? "development" : "production"] =
    process.argv.slice(2);
  if (!["prepare", "lock", "dev", "build"].includes(action))
    throw new Error("Usage: frontend-modules.mjs prepare|lock|dev|build [development|production]");
  const selection = moduleSelection(environment);
  if (action === "lock") return lockModules(selection);
  prepareModules(selection);
  if (action === "prepare") return;
  const env = {
    ...process.env,
    FRONTEND_MODULE_ENV: selection.environment,
    FRONTEND_MODULE_SOURCE: selection.source,
    ...(action === "build" ? { NODE_ENV: "production" } : {}),
  };
  console.log(`Frontend ${selection.environment}: ${selection.source} modules`);
  if (selection.source === "npm")
    execFileSync(process.execPath, ["scripts/check-npm-shell.mjs"], {
      cwd: frontendRoot,
      stdio: "inherit",
      env,
    });
  if (action === "dev") {
    if (env.FRONTEND_RUNTIME_CONFIG_PREPARED !== "true")
      execFileSync(process.execPath, ["scripts/use-runtime-config.mjs", "development"], {
        cwd: frontendRoot,
        stdio: "inherit",
        env,
      });
    if (selection.source === "npm") run(["run", "generate:versions"], env);
    run(["run", "dev", "-w", "@moh-sso/shell"], env);
  } else {
    run(
      ["run", selection.source === "npm" ? "build:docker:npm-prepared" : "build:docker:source"],
      env,
    );
  }
}
if (process.argv[1] && realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
