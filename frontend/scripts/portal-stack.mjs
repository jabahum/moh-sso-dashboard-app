/* global console, process, fetch, AbortSignal, URL, URLSearchParams */
import { existsSync, readdirSync } from "node:fs";
import { resolve, basename } from "node:path";
import { root, repo, execute, probe, waitFor, assertFreePort, readMap } from "./portal-runtime.mjs";

export const managedServices = [
  "express",
  "backend",
  "keycloak",
  "keycloak-db",
  "backend-db",
  "remote-postgres",
  "redis",
  "mailhog",
];
const dependencies = [
  "backend-db",
  "keycloak-db",
  "redis",
  "remote-postgres",
  "mailhog",
  "keycloak",
];
export const serviceName = (name = "backend") => (name === "frontend" ? "express" : name);

export function stackContext(profile, moduleName, overrides = {}) {
  const env = {
    ...process.env,
    PORTAL_DB_NAME: profile.database,
    PORTAL_NETWORK: profile.network,
    PORTAL_PUBLIC_ORIGIN: profile.publicOrigin,
    PORTAL_PORT: String(profile.port),
    PORTAL_BIND_HOST: profile.host,
    PORTAL_DIST_DIR: profile.dist,
    PORTAL_IMPORT_MAP_FILE: /^https?:\/\//.test(profile.importMap)
      ? resolve(root, "public/import-map.json")
      : profile.importMap,
    PORTAL_IMPORT_MAP_URL: /^https?:\/\//.test(profile.importMap) ? profile.importMap : "",
    PORTAL_MODULE: moduleName || profile.module || "",
    KEYCLOAK_EXTERNAL_URL: profile.keycloak,
    KEYCLOAK_REALM: profile.realm,
    KEYCLOAK_WEB_CLIENT_ID: profile.client,
  };
  const composeArgs = [
    "compose",
    "--project-directory",
    repo,
    ...profile.composeFiles.flatMap((file) => ["-f", file]),
  ];
  return {
    profile,
    env,
    run: execute,
    probe,
    wait: waitFor,
    freePort: assertFreePort,
    compose(args, options) {
      return this.run("docker", [...composeArgs, ...args], { cwd: repo, env, ...options });
    },
    ...overrides,
  };
}

function requireFiles(profile) {
  for (const path of profile.composeFiles)
    if (!existsSync(path)) throw new Error(`Compose file missing: ${path}`);
}

export function migrationProblem(version, dirty, latest) {
  if (dirty)
    return `Database migration ${version} is dirty. Inspect backend logs; no automatic reset or force is performed.`;
  if (version > latest)
    return `Database is at migration ${version}, but this checkout ends at ${latest}. Select an isolated database in the profile.`;
}

function latestMigration() {
  return Math.max(
    ...readdirSync(resolve(repo, "backend/internal/db/migrations")).map((name) =>
      Number(name.match(/^(\d+).*\.up\.sql$/)?.[1] || 0),
    ),
  );
}

async function databaseVersion(ctx) {
  // The database name is passed as an argument, never interpolated into shell/SQL.
  const output = await ctx.compose(
    [
      "exec",
      "-T",
      "backend-db",
      "sh",
      "-c",
      'psql -U "$POSTGRES_USER" -d "$1" -Atc "SELECT version, dirty FROM schema_migrations"',
      "portal",
      ctx.profile.database,
    ],
    { capture: true, quiet: true },
  );
  const line = output.trim().split("\n").at(-1);
  if (!/^\d+\|[tf]$/.test(line)) throw new Error("Migration table is not initialized.");
  const [version, dirty] = line.split("|");
  return { version: Number(version), dirty: dirty === "t" };
}

async function containers(ctx) {
  const output = await ctx.compose(["ps", "--all", "--format", "json"], {
    capture: true,
    quiet: true,
  });
  if (!output.trim()) return [];
  return output.trim().startsWith("[")
    ? JSON.parse(output)
    : output
        .trim()
        .split("\n")
        .map((line) => JSON.parse(line));
}

async function activeBackend(ctx, service = "backend") {
  const id = (await ctx.compose(["ps", "-q", service], { capture: true, quiet: true })).trim();
  if (!id) return {};
  const output = await ctx.run("docker", ["inspect", id, "--format", "{{json .Config.Env}}"], {
    capture: true,
    quiet: true,
  });
  return Object.fromEntries(
    JSON.parse(output).map((line) => {
      const index = line.indexOf("=");
      return [line.slice(0, index), line.slice(index + 1)];
    }),
  );
}

async function registerCallback(ctx) {
  const { profile } = ctx;
  const credentials = await ctx.compose(
    [
      "exec",
      "-T",
      "keycloak",
      "sh",
      "-c",
      'printf "%s\\n%s" "$KC_BOOTSTRAP_ADMIN_USERNAME" "$KC_BOOTSTRAP_ADMIN_PASSWORD"',
    ],
    { capture: true, quiet: true },
  );
  const newline = credentials.indexOf("\n");
  if (newline < 0) throw new Error("Local Keycloak administrator credentials are unavailable.");
  const response = await fetch(`${profile.keycloak}/realms/master/protocol/openid-connect/token`, {
    method: "POST",
    signal: AbortSignal.timeout(10000),
    body: new URLSearchParams({
      grant_type: "password",
      client_id: "admin-cli",
      username: credentials.slice(0, newline),
      password: credentials.slice(newline + 1),
    }),
  });
  if (!response.ok)
    throw new Error(
      `Local Keycloak administration returned HTTP ${response.status}. Check the configured administrator credentials.`,
    );
  const headers = {
    Authorization: `Bearer ${(await response.json()).access_token}`,
    "Content-Type": "application/json",
  };
  const base = `${profile.keycloak}/admin/realms/${encodeURIComponent(profile.realm)}/clients`;
  const clients = await fetch(`${base}?clientId=${encodeURIComponent(profile.client)}`, {
    headers,
    signal: AbortSignal.timeout(10000),
  });
  if (!clients.ok) throw new Error(`Keycloak client lookup returned HTTP ${clients.status}`);
  const client = (await clients.json())[0];
  if (!client) throw new Error(`Keycloak client ${profile.client} is missing.`);
  const callback = `${profile.publicOrigin}/backend/api/v1/auth/callback`;
  if (!client.redirectUris?.includes(callback)) {
    const update = await fetch(`${base}/${client.id}`, {
      method: "PUT",
      headers,
      body: JSON.stringify({ ...client, redirectUris: [...(client.redirectUris || []), callback] }),
      signal: AbortSignal.timeout(10000),
    });
    if (!update.ok) throw new Error(`Keycloak callback update returned HTTP ${update.status}`);
  }
  console.log(`SSO callback ready: ${callback}`);
}

export async function status(ctx) {
  const { profile } = ctx;
  console.log(
    `Profile: ${profile.name} | ${profile.publicOrigin}/portal/ | database: ${profile.database}`,
  );
  try {
    const rows = await containers(ctx);
    for (const name of managedServices) {
      const row = rows.find((entry) => entry.Service === name);
      const ports =
        row?.Publishers?.map((port) => `${port.PublishedPort}→${port.TargetPort}`).join(", ") || "";
      console.log(`${name.padEnd(18)} ${(row?.State || "not created").padEnd(14)} ${ports}`);
    }
    const env = await activeBackend(ctx);
    if (env.DB_NAME)
      console.log(
        `Active backend database: ${env.DB_NAME}${env.DB_NAME !== profile.database ? " (PROFILE MISMATCH)" : ""}`,
      );
  } catch (error) {
    console.log(
      `Docker unavailable or Compose failed: ${error.message} Start Docker Desktop and run portal doctor.`,
    );
  }
  const [backend, frontend, config] = await Promise.all([
    ctx.probe(`${profile.backend}/health/ready`),
    ctx.probe(`${profile.publicOrigin}/healthz`),
    ctx.probe(`${profile.publicOrigin}/portal/config.js`),
  ]);
  console.log(`Backend readiness: ${backend.status || "unreachable"}`);
  console.log(
    `Express health: ${frontend.json?.status === "ok" ? "ready" : `unavailable (${frontend.status})`}`,
  );
  const selected = config.body?.match(/"standaloneModule"\s*:\s*"([^"]+)"/)?.[1];
  console.log(
    `Active module: ${selected || (config.body?.includes('"hostRuntime":true') ? "full portal" : "unknown")}`,
  );
}

export async function doctor(ctx) {
  let failures = 0;
  const check = (okay, message) => {
    console.log(`${okay ? "OK" : "FAIL"} ${message}`);
    if (!okay) failures++;
  };
  const { profile } = ctx;
  console.log(`Doctor: ${profile.name}`);
  check(existsSync(resolve(profile.dist, "index.html")), "Shell build exists (portal build)");
  check(
    existsSync(resolve(root, "server/node_modules/express")),
    "Foreground host dependencies installed (portal setup)",
  );
  for (const path of profile.composeFiles)
    check(existsSync(path), `Compose file: ${basename(path)}`);
  try {
    const map = await readMap(profile.importMap);
    const missing = Object.entries(map.imports).filter(
      ([key, value]) =>
        key.startsWith("@moh-sso/") &&
        typeof value === "string" &&
        value.startsWith("/") &&
        !existsSync(resolve(profile.dist, value.replace(/^\/portal\//, "").replace(/^\//, ""))),
    );
    check(
      missing.length === 0,
      missing.length
        ? `Missing staged assets: ${missing.map(([name]) => name).join(", ")} (portal build)`
        : "Import map and staged assets available",
    );
  } catch (error) {
    check(false, `Import map: ${error.message}`);
  }
  try {
    const config = JSON.parse(
      await ctx.compose(["config", "--format", "json"], { capture: true, quiet: true }),
    );
    const env = config.services.backend.environment;
    check(
      env.DB_NAME === profile.database,
      `Resolved database: ${env.DB_NAME}; expected ${profile.database}`,
    );
    check(
      env.KEYCLOAK_REDIRECT_URI === `${profile.publicOrigin}/backend/api/v1/auth/callback`,
      "Resolved SSO callback matches the profile",
    );
    await containers(ctx);
    check(true, "Docker is available");
    const active = await activeBackend(ctx);
    check(
      active.DB_NAME === profile.database,
      `Running backend uses ${active.DB_NAME || "no database"}; expected ${profile.database} (portal restart backend)`,
    );
    check(
      active.KEYCLOAK_REDIRECT_URI === `${profile.publicOrigin}/backend/api/v1/auth/callback`,
      "Running backend SSO callback matches the profile",
    );
    const migration = await databaseVersion(ctx);
    const problem = migrationProblem(migration.version, migration.dirty, latestMigration());
    check(!problem, problem || `Database migration ${migration.version} is compatible`);
  } catch (error) {
    check(false, `Stack configuration/database: ${error.message}`);
  }
  const [backend, express] = await Promise.all([
    ctx.probe(`${profile.backend}/health/ready`),
    ctx.probe(`${profile.publicOrigin}/healthz`),
  ]);
  check(backend.status === 200, `Backend readiness HTTP ${backend.status || "unreachable"}`);
  check(express.json?.status === "ok", `Express health HTTP ${express.status || "unreachable"}`);
  if (express.json?.status !== "ok") {
    try {
      await ctx.freePort(profile.port, profile.host);
      console.log(`INFO Port ${profile.port} is free; run portal up.`);
    } catch (error) {
      check(false, error.message);
    }
  }
  const login = await ctx.probe(`${profile.publicOrigin}/backend/health/ready`);
  check(login.status === 200, `API proxy readiness HTTP ${login.status || "unreachable"}`);
  try {
    const response = await fetch(
      `${profile.publicOrigin}/backend/api/v1/auth/login?returnTo=%2Fportal%2F`,
      { redirect: "manual", signal: AbortSignal.timeout(5000) },
    );
    const destination = response.headers.get("location");
    const url = destination && new URL(destination);
    check(
      Boolean(
        url &&
        url.searchParams.get("redirect_uri") ===
          `${profile.publicOrigin}/backend/api/v1/auth/callback`,
      ),
      "Login redirects to the configured callback",
    );
    if (url) {
      const form = await ctx.probe(url.href);
      check(
        form.status === 200 && form.body?.includes('id="kc-form-login"'),
        "Keycloak accepts the callback and serves its login form",
      );
    }
  } catch (error) {
    check(false, `Login flow: ${error.message}`);
  }
  console.log(`${failures} problem(s). No databases were reset or migration versions forced.`);
  return failures;
}

export async function runStack(
  action,
  ctx,
  { service, dryRun = false, build = false, timeout = 180000, register = registerCallback } = {},
) {
  const name = serviceName(service);
  if (
    (["logs", "restart"].includes(action) || (action === "down" && service)) &&
    !managedServices.includes(name)
  )
    throw new Error(`Unknown service: ${service}`);
  requireFiles(ctx.profile);
  if (dryRun) {
    console.log(
      `Profile: ${ctx.profile.name}; database: ${ctx.profile.database}; origin: ${ctx.profile.publicOrigin}`,
    );
    if (["up", "restart"].includes(action))
      console.log(
        "Check assets/ports → start dependencies → create database if missing → configure callback → start backend → wait for readiness → start Express",
      );
    const args =
      action === "down"
        ? ["stop", ...(service ? [name] : managedServices)]
        : action === "logs"
          ? ["logs", "--follow", "--tail", "100", name]
          : action === "restart"
            ? ["up", "-d", "--no-deps", "--force-recreate", ...(build ? ["--build"] : []), name]
            : action === "up"
              ? ["up", "-d", ...(build ? ["--build"] : []), "backend", "express"]
              : ["ps", "--all"];
    console.log(
      `docker compose ${ctx.profile.composeFiles.map((file) => `-f ${file}`).join(" ")} ${args.join(" ")}`,
    );
    return;
  }
  if (action === "status") return status(ctx);
  if (action === "doctor") {
    const problems = await doctor(ctx);
    if (problems) throw new Error(`Doctor found ${problems} problem(s).`);
    return;
  }
  if (action === "down") return ctx.compose(["stop", ...(service ? [name] : managedServices)]);
  if (action === "logs") return ctx.compose(["logs", "--follow", "--tail", "100", name]);
  if (["up", "restart"].includes(action)) {
    if (action === "restart") {
      if (name === "express" && !ctx.profile.module) {
        const active = await activeBackend(ctx, "express");
        ctx.env.PORTAL_MODULE = active.STANDALONE_MODULE || "";
      }
      // up --force-recreate reapplies the profile; plain restart keeps old env.
      await ctx.compose([
        "up",
        "-d",
        "--no-deps",
        "--force-recreate",
        ...(build ? ["--build"] : []),
        name,
      ]);
      if (name === "backend")
        await ctx.wait("backend", `${ctx.profile.backend}/health/ready`, timeout);
      if (name === "express")
        await ctx.wait(
          "Express",
          `${ctx.profile.publicOrigin}/healthz`,
          timeout,
          (result) => result.json?.status === "ok",
        );
      return;
    }
    if (!existsSync(resolve(ctx.profile.dist, "index.html")))
      throw new Error("Shell build missing. Run 'portal build' first.");
    await readMap(ctx.profile.importMap);
    const rows = await containers(ctx);
    if (!rows.some((row) => row.Service === "express" && row.State === "running"))
      await ctx.freePort(ctx.profile.port, ctx.profile.host);
    try {
      await ctx.run("docker", ["network", "inspect", ctx.profile.network], {
        capture: true,
        quiet: true,
      });
    } catch {
      await ctx.run("docker", ["network", "create", ctx.profile.network]);
    }
    await ctx.compose(["up", "-d", ...dependencies]);
    await ctx.compose(["run", "--rm", "--no-deps", "portal-db-init"]);
    // A fresh DB has no migration table yet. An existing newer/dirty DB must stop here.
    try {
      const version = await databaseVersion(ctx);
      const problem = migrationProblem(version.version, version.dirty, latestMigration());
      if (problem) throw Object.assign(new Error(problem), { migrationFailure: true });
    } catch (error) {
      if (!/relation "schema_migrations" does not exist/.test(error.stderr || "")) throw error;
    }
    await ctx.wait(
      "Keycloak realm",
      `${ctx.profile.keycloak}/realms/${encodeURIComponent(ctx.profile.realm)}/.well-known/openid-configuration`,
      timeout,
    );
    await register(ctx);
    await ctx.compose(["up", "-d", ...(build ? ["--build"] : []), "backend"]);
    await ctx.wait("backend", `${ctx.profile.backend}/health/ready`, timeout);
    await ctx.compose(["up", "-d", "--no-deps", ...(build ? ["--build"] : []), "express"]);
    await ctx.wait(
      "Express",
      `${ctx.profile.publicOrigin}/healthz`,
      timeout,
      (result) => result.json?.status === "ok",
    );
    await ctx.wait("API proxy", `${ctx.profile.publicOrigin}/backend/health/ready`, timeout);
    console.log(`Ready: ${ctx.profile.publicOrigin}/portal/`);
  }
}
