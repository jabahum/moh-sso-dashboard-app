/* global process, console, URL */
import { spawn } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

const root = fileURLToPath(new URL("../", import.meta.url));
const help = `MOH frontend command helper (Node 22+)

From frontend/: npm run portal -- <command> [options]
From the repository root: node frontend/scripts/portal.mjs <command> [options]

Commands:
  setup                 Install frontend and Express host dependencies
  build [all|shell|app]  Build/stage everything (default), shell only, or one app
  serve [app]           Start the full portal or a standalone app
  preview <app>         Serve an already-built app on its local import-map port
  check                 Run host HTTP tests and frontend TypeScript checks
  modules               List available apps and their preview ports

Options:
  --backend <url>       Backend target for serve (default: BACKEND_URL or port 9000)
  --import-map <path|url>  Local file or central map for serve
  --dist <path>         Built shell directory for serve
  --host <address>      Bind address for serve/preview
  --port <number>       Port for serve/preview
  --dry-run             Print commands without running them
  -h, --help            Show this help

Examples:
  npm run portal -- setup
  npm run portal -- build
  npm run portal -- serve users --backend http://127.0.0.1:9000
  npm run portal -- serve documents --import-map https://assets.example.com/import-map.json
  npm run portal -- build users
  npm run portal -- preview users

build <app> only builds that app; use preview for local-map development.
build shell is for externally hosted modules; build all stages local modules too.
Local option paths resolve from the directory where you invoke this helper.
SSO still requires the backend/Keycloak settings in docs/express-host.md.
`;

function modules() {
  const map = JSON.parse(readFileSync(resolve(root, "public/import-map.local.json"), "utf8"));
  return readdirSync(resolve(root, "apps"))
    .filter((name) => existsSync(resolve(root, "apps", name, "src/single-spa.tsx")))
    .sort()
    .map((name) => {
      const url = map.imports[`@moh-sso/${name}`];
      return { name, port: url ? new URL(url).port : undefined };
    });
}

async function run() {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      help: { type: "boolean", short: "h" },
      "dry-run": { type: "boolean" },
      backend: { type: "string" },
      "import-map": { type: "string" },
      dist: { type: "string" },
      host: { type: "string" },
      port: { type: "string" },
    },
  });
  if (values.help || positionals.length === 0) {
    console.log(help);
    return;
  }
  const [command, rawApp] = positionals;
  if (!Object.hasOwn({ setup: 1, build: 1, serve: 1, preview: 1, check: 1, modules: 1 }, command)) {
    throw new Error(`Unknown command: ${command}. Use --help.`);
  }
  if (positionals.length > 2 || (rawApp && !["build", "serve", "preview"].includes(command))) {
    throw new Error(`Unexpected arguments for ${command}. Use --help.`);
  }
  const available = modules();
  const app = rawApp?.replace(/^@moh-sso\//, "");
  const selected = available.find((entry) => entry.name === app);
  if (app && !selected && !(command === "build" && ["all", "shell"].includes(app))) {
    throw new Error(`Unknown app: ${rawApp}. Run 'portal modules' to list apps.`);
  }
  if (command === "preview" && !selected) throw new Error("preview requires an app name.");
  const allowed =
    command === "serve"
      ? ["backend", "import-map", "dist", "host", "port"]
      : command === "preview"
        ? ["host", "port"]
        : [];
  for (const option of Object.keys(values)) {
    if (!["help", "dry-run", ...allowed].includes(option)) {
      throw new Error(`--${option} is not supported by ${command}.`);
    }
  }
  if (
    values.port &&
    (!/^\d+$/.test(values.port) || Number(values.port) < 1 || Number(values.port) > 65535)
  ) {
    throw new Error("--port must be between 1 and 65535.");
  }
  const env = { ...process.env };
  const settings = {};
  if (command === "serve") {
    // Full portal mode must not inherit an accidental standalone selection.
    delete env.STANDALONE_MODULE;
    if (selected) settings.STANDALONE_MODULE = selected.name;
    if (values.backend) {
      const url = new URL(values.backend);
      if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) {
        throw new Error("--backend must be an HTTP(S) URL without credentials.");
      }
      settings.BACKEND_URL = values.backend;
    }
    if (values["import-map"]) {
      delete env.IMPORT_MAP_URL;
      delete env.IMPORT_MAP_FILE;
      const source = values["import-map"];
      if (/^https?:\/\//.test(source)) {
        const url = new URL(source);
        if (url.username || url.password)
          throw new Error("Import map URLs must not contain credentials.");
        settings.IMPORT_MAP_URL = source;
      } else {
        settings.IMPORT_MAP_FILE = resolve(source);
      }
    }
    if (values.dist) settings.FRONTEND_DIST_DIR = resolve(values.dist);
    if (values.host) settings.HOST = values.host;
    if (values.port) settings.PORT = values.port;
  }
  Object.assign(env, settings);
  if (command === "modules") {
    for (const entry of available)
      console.log(`${entry.name.padEnd(20)} ${entry.port || "no preview port"}`);
    return;
  }
  const tasks = [];
  if (command === "setup") tasks.push(["ci"], ["ci", "--prefix", "server"]);
  if (command === "build")
    tasks.push(
      selected
        ? ["run", "build", "--workspace", `@moh-sso/${selected.name}`]
        : ["run", app === "shell" ? "build:shell" : "build:docker"],
    );
  if (command === "serve") tasks.push(["run", "host:start"]);
  if (command === "check") tasks.push(["run", "host:test"], ["run", "typecheck"]);
  if (command === "preview") {
    if (!values.port && !selected.port)
      throw new Error("No preview port is configured; pass --port.");
    tasks.push([
      "run",
      "preview",
      "--workspace",
      `@moh-sso/${selected.name}`,
      "--",
      "--port",
      values.port || selected.port,
      "--strictPort",
      ...(values.host ? ["--host", values.host] : []),
    ]);
  }
  if (values["dry-run"]) {
    console.log(`Working directory: ${root}`);
    for (const [key, value] of Object.entries(settings))
      console.log(`${key}=${JSON.stringify(value)}`);
    for (const args of tasks) console.log(`npm ${args.join(" ")}`);
    return;
  }
  if (Number(process.versions.node.split(".")[0]) < 22) throw new Error("Use Node 22 or later.");
  if (command === "serve") {
    const dist = env.FRONTEND_DIST_DIR || "dist";
    if (!existsSync(resolve(root, dist, "index.html")))
      throw new Error("Shell build missing. Run 'portal build' first.");
    if (!existsSync(resolve(root, "server/node_modules/express")))
      throw new Error("Host dependencies missing. Run 'npm run host:install' or 'portal setup'.");
  }
  if (
    command === "preview" &&
    !existsSync(resolve(root, "apps", selected.name, "dist/single-spa.js"))
  ) {
    throw new Error(`App build missing. Run 'portal build ${selected.name}' first.`);
  }
  // Use npm belonging to this Node installation, even when PATH contains an older npm.
  const npm =
    process.env.npm_execpath ||
    resolve(dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js");
  if (!existsSync(npm))
    throw new Error("Cannot locate npm. Invoke this helper through 'npm run portal -- ...'.");
  for (const args of tasks) {
    console.log(`> npm ${args.join(" ")}`);
    const code = await new Promise((resolveTask, reject) => {
      const child = spawn(process.execPath, [npm, ...args], {
        cwd: root,
        env: { ...env, PATH: `${dirname(process.execPath)}:${env.PATH || ""}` },
        stdio: "inherit",
      });
      child.once("error", reject);
      child.once("exit", (status, signal) =>
        resolveTask(signal ? (signal === "SIGINT" ? 130 : 143) : (status ?? 1)),
      );
    });
    if (code !== 0) {
      process.exitCode = code;
      return;
    }
  }
}

run().catch((error) => {
  console.error(`portal: ${error.message}`);
  process.exitCode = 1;
});
