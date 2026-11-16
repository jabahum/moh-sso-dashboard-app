/* global console, process */
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, realpathSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { frontendRoot, installedModules, moduleSelection } from "./module-source.mjs";

const npm = process.env.NPM || "npm";
const run = (args, env) => execFileSync(npm, args, { cwd: frontendRoot, stdio: "inherit", env });
export function prepareModules(selection) {
  if (selection.source === "workspace") return;
  const manifest = {
    name: "moh-installed-frontend-modules",
    private: true,
    dependencies: selection.versions,
  };
  const path = join(selection.directory, "package.json");
  const unchanged =
    existsSync(path) &&
    JSON.stringify(JSON.parse(readFileSync(path, "utf8"))) === JSON.stringify(manifest);
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
  run(
    [
      unchanged && existsSync(join(selection.directory, "package-lock.json")) ? "ci" : "install",
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
  if (!["prepare", "dev", "build"].includes(action))
    throw new Error("Usage: frontend-modules.mjs prepare|dev|build [development|production]");
  const selection = moduleSelection(environment);
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
