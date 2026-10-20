/* global process, console, URL */
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { parseArgs } from "node:util";
import { root, loadProfile, execute, npmArgs, modules } from "./portal-runtime.mjs";
import { runStack, stackContext } from "./portal-stack.mjs";
import { develop } from "./portal-dev.mjs";

const help = `MOH frontend command helper (Node 22+)

From frontend/: npm run portal -- <command> [options]
From the repository root: node frontend/scripts/portal.mjs <command> [options]

Commands:
  setup                 Install frontend and Express host dependencies
  build [all|shell|app]  Build/stage everything (default), shell only, or one app
  serve [app]           Start the full portal or a standalone app
  preview <app>         Serve an already-built app on its local import-map port
  check                 Run CLI tests, host HTTP tests and TypeScript checks
  modules               List available apps and their preview ports
  up [app]              Start dependencies, backend and Docker-managed Express
  down [service]        Stop services, preserving database volumes
  status                Show containers, ports, readiness, DB and active module
  doctor                Diagnose config, migrations, assets, ports and SSO
  logs [service]        Follow logs (default backend; frontend aliases express)
  restart [service]     Recreate with profile settings (default backend)
  dev <app>             Watch/rebuild app, preview it, and run standalone Express

Options:
  --backend <url>       Backend target for foreground serve/dev
  --import-map <path|url>  Local file or central map for serve/dev/up
  --dist <path>         Built shell directory for serve/dev/up
  --host <address>      Bind address for serve/dev/up/preview
  --port <number>       Express port for serve/dev/up; app port for preview
  --dry-run             Print commands without running them
  --profile <name>      Named profile (default local)
  --profiles-file <file> Extra profile definitions (default .local-sso/profiles.json)
  --timeout <seconds>   Readiness timeout for up/restart (default 180)
  --build              Rebuild Docker images for up/restart
  --module-port <port>  App preview port for dev
  -h, --help            Show this help

Examples:
  npm run portal -- setup
  npm run portal -- build
  npm run portal -- serve users --backend http://127.0.0.1:9000
  npm run portal -- serve documents --import-map https://assets.example.com/import-map.json
  npm run portal -- build users
  npm run portal -- preview users
  npm run portal -- up --profile local
  npm run portal -- doctor
  npm run portal -- restart backend
  npm run portal -- dev users

build <app> only builds that app; use preview for local-map development.
build shell is for externally hosted modules; build all stages local modules too.
Local option paths resolve from the directory where you invoke this helper.
SSO still requires the backend/Keycloak settings in docs/express-host.md.
`;

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
      profile: { type: "string" },
      "profiles-file": { type: "string" },
      timeout: { type: "string" },
      build: { type: "boolean" },
      "module-port": { type: "string" },
    },
  });
  if (values.help || positionals.length === 0) {
    console.log(help);
    return;
  }
  const [command, rawApp] = positionals;
  const profile = loadProfile(values.profile || "local", values["profiles-file"]);
  const stackCommands = ["up", "down", "status", "doctor", "logs", "restart"];
  if (stackCommands.includes(command)) {
    if (positionals.length > 2 || (rawApp && !["up", "logs", "restart", "down"].includes(command)))
      throw new Error(`Unexpected arguments for ${command}.`);
    const allowed =
      command === "up"
        ? ["port", "host", "dist", "import-map", "build", "timeout"]
        : command === "restart"
          ? ["build", "timeout"]
          : [];
    for (const key of Object.keys(values))
      if (!["profile", "profiles-file", "dry-run", "help", ...allowed].includes(key))
        throw new Error(`--${key} is not supported by ${command}.`);
    if (values.port) {
      if (!/^\d+$/.test(values.port) || Number(values.port) < 1 || Number(values.port) > 65535)
        throw new Error("--port must be between 1 and 65535.");
      profile.port = Number(values.port);
      const origin = new URL(profile.publicOrigin);
      origin.port = values.port;
      profile.publicOrigin = origin.origin;
    }
    if (values.host) profile.host = values.host;
    if (values.dist) profile.dist = resolve(values.dist);
    if (values["import-map"])
      profile.importMap = /^https?:\/\//.test(values["import-map"])
        ? values["import-map"]
        : resolve(values["import-map"]);
    if (
      values.timeout &&
      (!/^\d+$/.test(values.timeout) || Number(values.timeout) < 1 || Number(values.timeout) > 3600)
    )
      throw new Error("--timeout must be between 1 and 3600 seconds.");
    const moduleName =
      command === "up" ? (rawApp || profile.module)?.replace(/^@moh-sso\//, "") : undefined;
    if (moduleName && !modules().some((entry) => entry.name === moduleName))
      throw new Error(`Unknown app: ${moduleName}`);
    if (!values["dry-run"] && Number(process.versions.node.split(".")[0]) < 22)
      throw new Error("Use Node 22 or later.");
    return runStack(command, stackContext(profile, moduleName), {
      service: ["logs", "restart", "down"].includes(command) ? rawApp : undefined,
      dryRun: values["dry-run"],
      build: values.build,
      timeout: Number(values.timeout || 180) * 1000,
    });
  }
  if (command === "dev") {
    if (positionals.length !== 2) throw new Error("dev requires an app name.");
    const selected = modules().find((entry) => entry.name === rawApp.replace(/^@moh-sso\//, ""));
    if (!selected) throw new Error(`Unknown app: ${rawApp}`);
    for (const key of Object.keys(values))
      if (
        ![
          "profile",
          "profiles-file",
          "dry-run",
          "help",
          "port",
          "host",
          "dist",
          "import-map",
          "backend",
          "module-port",
        ].includes(key)
      )
        throw new Error(`--${key} is not supported by dev.`);
    return develop(profile, selected, values);
  }
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
    if (!["help", "dry-run", "profile", "profiles-file", ...allowed].includes(option)) {
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
  if (command === "serve") {
    env.BACKEND_URL ||= profile.backend;
    env.PORT ||= String(profile.port);
    env.HOST ||= profile.host;
    env.FRONTEND_DIST_DIR ||= profile.dist;
    if (!env.IMPORT_MAP_URL && !env.IMPORT_MAP_FILE)
      env[/^https?:\/\//.test(profile.importMap) ? "IMPORT_MAP_URL" : "IMPORT_MAP_FILE"] =
        profile.importMap;
    if (!rawApp && profile.module) env.STANDALONE_MODULE = profile.module;
  }
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
  if (command === "check")
    tasks.push(["run", "portal:test"], ["run", "host:test"], ["run", "typecheck"]);
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
    console.log(`Profile: ${profile.name}`);
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
  for (const args of tasks) {
    await execute(process.execPath, npmArgs(args), { env });
  }
}

run().catch((error) => {
  console.error(`portal: ${error.message}`);
  process.exitCode = error.exitCode || 1;
});
