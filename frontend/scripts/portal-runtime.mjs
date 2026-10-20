/* global process, console, URL, fetch, AbortSignal, setTimeout, clearTimeout */
import { spawn } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";

export const root = fileURLToPath(new URL("../", import.meta.url));
export const repo = resolve(root, "..");
export const sleep = (ms) => new Promise((done) => setTimeout(done, ms));

export function httpUrl(value, label) {
  const url = new URL(value);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) {
    throw new Error(`${label} must be an HTTP(S) URL without credentials.`);
  }
  return url;
}

export function loadProfile(name = "local", filename) {
  const defaults = JSON.parse(readFileSync(resolve(root, "portal.profiles.json"), "utf8"));
  const customFile = filename ? resolve(filename) : resolve(repo, ".local-sso/profiles.json");
  const custom = existsSync(customFile) ? JSON.parse(readFileSync(customFile, "utf8")) : {};
  if (filename && !existsSync(customFile)) throw new Error(`Profiles file missing: ${customFile}`);
  if (!Object.hasOwn(defaults, name) && !Object.hasOwn(custom, name))
    throw new Error(`Unknown profile: ${name}`);
  const profile = { ...defaults.local, ...defaults[name], ...custom[name], name };
  for (const key of ["backend", "publicOrigin", "keycloak"])
    profile[key] = httpUrl(profile[key], key).href.replace(/\/$/, "");
  const origin = new URL(profile.publicOrigin);
  if (origin.pathname !== "/" || origin.search || origin.hash)
    throw new Error("publicOrigin must be an origin without a path, query, or fragment.");
  if (!Number.isInteger(Number(profile.port)) || profile.port < 1 || profile.port > 65535)
    throw new Error("Profile port must be between 1 and 65535.");
  if (!/^[a-z_][a-z0-9_]{0,62}$/.test(profile.database))
    throw new Error(
      "Profile database must be a PostgreSQL identifier (lowercase letters, digits, underscores).",
    );
  if (!Array.isArray(profile.composeFiles) || !profile.composeFiles.length)
    throw new Error("Profile composeFiles must be a nonempty array.");
  profile.composeFiles = profile.composeFiles.map((file) => resolve(repo, file));
  profile.dist = resolve(root, profile.dist);
  profile.importMap = /^https?:\/\//.test(profile.importMap)
    ? httpUrl(profile.importMap, "importMap").href
    : resolve(root, profile.importMap);
  profile.port = Number(profile.port);
  return profile;
}

export function modules() {
  const map = JSON.parse(readFileSync(resolve(root, "public/import-map.local.json"), "utf8"));
  return readdirSync(resolve(root, "apps"))
    .filter((name) => existsSync(resolve(root, "apps", name, "src/single-spa.tsx")))
    .sort()
    .map((name) => ({
      name,
      port: map.imports[`@moh-sso/${name}`]
        ? new URL(map.imports[`@moh-sso/${name}`]).port
        : undefined,
    }));
}

export async function execute(
  file,
  args,
  { env = process.env, cwd = root, capture = false, quiet = false } = {},
) {
  if (!quiet) console.log(`> ${file === process.execPath ? "node" : file} ${args.join(" ")}`);
  return new Promise((done, reject) => {
    const child = spawn(file, args, {
      cwd,
      env: { ...env, PATH: `${dirname(process.execPath)}:${env.PATH || ""}` },
      stdio: capture ? ["ignore", "pipe", "pipe"] : "inherit",
    });
    let output = "";
    let errorOutput = "";
    if (capture) {
      child.stdout.on("data", (chunk) => {
        output += chunk;
      });
      child.stderr.on("data", (chunk) => {
        errorOutput += chunk;
      });
    }
    child.once("error", reject);
    child.once("close", (code, signal) => {
      if (code === 0) return done(output);
      const error = new Error(
        `${file} ${args.slice(0, 3).join(" ")} failed (${signal || code}).${capture && !quiet ? ` ${errorOutput.trim()}` : ""}`,
      );
      error.exitCode = code || 1;
      error.stderr = errorOutput;
      reject(error);
    });
  });
}

export function npmArgs(args) {
  const npm =
    process.env.npm_execpath ||
    resolve(dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js");
  if (!existsSync(npm)) throw new Error("Cannot locate npm. Use 'npm run portal -- ...'.");
  return [npm, ...args];
}

export async function probe(url) {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(2500), redirect: "manual" });
    const body = await response.text();
    let json;
    try {
      json = JSON.parse(body);
    } catch {
      /* HTML or plain-text response */
    }
    return { status: response.status, json, body };
  } catch {
    return { status: 0 };
  }
}

export async function waitFor(label, url, timeout, accept = (result) => result.status === 200) {
  console.log(`Waiting for ${label} (${url})…`);
  const deadline = Date.now() + timeout;
  let nextNotice = Date.now() + 20000;
  do {
    if (accept(await probe(url))) return;
    if (Date.now() >= nextNotice) {
      console.log(`Still waiting for ${label}…`);
      nextNotice += 20000;
    }
    await sleep(1000);
  } while (Date.now() < deadline);
  throw new Error(`${label} did not become ready. Run 'portal logs backend' and 'portal doctor'.`);
}

export async function assertFreePort(port, host = "127.0.0.1") {
  await new Promise((done, reject) => {
    const server = createServer();
    server.once("error", () =>
      reject(
        new Error(
          `Port ${port} on ${host} is occupied. Stop the existing server or choose --port.`,
        ),
      ),
    );
    server.listen(Number(port), host, () => server.close(done));
  });
}

// Each child has its own process group, so shutdown also stops npm's descendants.
export async function supervise(commands, env) {
  const children = [];
  let stopping = false;
  let killTimer;
  const stop = () => {
    if (stopping) return;
    stopping = true;
    for (const child of children) {
      try {
        process.kill(-child.pid, "SIGTERM");
      } catch {
        /* already stopped */
      }
    }
    killTimer = setTimeout(() => {
      for (const child of children) {
        if (child.exitCode !== null || child.signalCode !== null) continue;
        try {
          process.kill(-child.pid, "SIGKILL");
        } catch {
          /* already stopped */
        }
      }
    }, 5000);
    killTimer.unref();
  };
  const interrupt = () => {
    process.exitCode = 130;
    stop();
  };
  process.once("SIGINT", interrupt);
  process.once("SIGTERM", interrupt);
  try {
    await new Promise((done, reject) => {
      let remaining = commands.length;
      let failure;
      for (const [file, args] of commands) {
        const child = spawn(file, args, {
          cwd: root,
          env: { ...env, PATH: `${dirname(process.execPath)}:${env.PATH || ""}` },
          stdio: "inherit",
          detached: true,
        });
        children.push(child);
        child.once("error", (error) => {
          failure ||= error;
          stop();
        });
        child.once("close", (code, signal) => {
          if (!stopping) {
            failure = new Error(
              `Development process stopped (${signal || code}); stopping the other processes.`,
            );
            stop();
          }
          if (--remaining === 0) {
            if (failure) reject(failure);
            else done();
          }
        });
      }
    });
  } finally {
    stop();
    clearTimeout(killTimer);
    process.removeListener("SIGINT", interrupt);
    process.removeListener("SIGTERM", interrupt);
  }
}

export async function readMap(source) {
  const remote = /^https?:\/\//.test(source);
  const map = remote
    ? await (async () => {
        const response = await fetch(source, { signal: AbortSignal.timeout(10000) });
        if (!response.ok) throw new Error(`Import map returned HTTP ${response.status}`);
        return response.json();
      })()
    : JSON.parse(readFileSync(source, "utf8"));
  if (!map.imports || typeof map.imports !== "object" || Array.isArray(map.imports))
    throw new Error("Import map must contain an imports object.");
  const entries = (items) =>
    Object.fromEntries(
      Object.entries(items).map(([key, value]) => {
        if (typeof value !== "string") throw new Error(`Invalid import map target: ${key}`);
        return [key, remote ? new URL(value, source).href : value];
      }),
    );
  map.imports = entries(map.imports);
  if (map.scopes)
    map.scopes = Object.fromEntries(
      Object.entries(map.scopes).map(([scope, items]) => [
        remote ? new URL(scope, source).href : scope,
        entries(items),
      ]),
    );
  return map;
}
