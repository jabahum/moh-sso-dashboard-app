// Helpers for the identity-sync e2e suites. Everything talks to the disposable
// "idsync-e2e" compose stack (infra/compose/identity-sync-e2e) over HTTPS on
// 127.0.0.1:18443, trusting only Caddy's internal root CA. Every credential in
// this file is a FAKE fixture value from the stack's realm overlay.
import { spawnSync } from "node:child_process";
import { X509Certificate, createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import https from "node:https";
import tls from "node:tls";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = resolve(here, "../../..");
export const STACK_DIR = resolve(REPO_ROOT, "infra/compose/identity-sync-e2e");
export const CA_FILE = resolve(STACK_DIR, "generated/caddy-root.crt");
export const PORT = 18443;
export const ISSUER = `https://sso.e2e.test:${PORT}/realms/moh-realm`;
export const REALM = "moh-realm";
export const PASSWORD = "E2e-Fixture-Only-1"; // fake fixture password
export const origin = (host) => `https://${host}.e2e.test:${PORT}`;
export const SSO = origin("sso");
export const DASHBOARD = origin("dashboard");
export const CLIMS = origin("clims");
export const POES = origin("poes");
export const MAIL = origin("mail");
export const SYNC_BASE = "/api/v1/internal/identity-sync";

/** Fixed Keycloak user ID for fixture number n (see the stack README). */
export const kcId = (n) => `e2e00000-0000-4000-8000-${String(n).padStart(12, "0")}`;
/** CLIMS account ID for fixture number n (hex, 1-based). */
export const climsId = (n) => `c1000000-0000-4000-8000-${n.toString(16).padStart(12, "0")}`;

export const APPS = {
  clims: {
    app: "clims",
    origin: CLIMS,
    syncClient: "clims-identity-sync",
    syncSecret: "e2e-fake-clims-sync-secret",
    accessRole: "clims_access",
    db: { service: "clims-postgres", user: "clims_e2e", name: "clims_e2e" },
  },
  poesgo: {
    app: "poesgo",
    origin: POES,
    syncClient: "poesgo-identity-sync",
    syncSecret: "e2e-fake-poesgo-sync-secret",
    accessRole: "poesgo_access",
    db: { service: "poes-postgres", user: "poesgo_e2e", name: "poesgo_e2e" },
  },
};

let ca;
function caPem() {
  ca ??= readFileSync(CA_FILE, "utf8");
  return ca;
}

const lookup = (hostname, options, callback) => {
  if (!hostname.endsWith(".e2e.test")) return callback(new Error(`refusing to resolve ${hostname}`));
  if (options && options.all) return callback(null, [{ address: "127.0.0.1", family: 4 }]);
  return callback(null, "127.0.0.1", 4);
};

/**
 * HTTPS request to an *.e2e.test origin, pinned to 127.0.0.1 and verified
 * against Caddy's root CA only. Never follows redirects.
 */
export function request(method, url, { headers = {}, body, form, json, timeout = 20000 } = {}) {
  const u = new URL(url);
  let payload;
  const h = { ...headers };
  if (form) {
    payload = new URLSearchParams(form).toString();
    h["content-type"] = "application/x-www-form-urlencoded";
  } else if (json !== undefined) {
    payload = JSON.stringify(json);
    h["content-type"] = "application/json";
  } else if (body !== undefined) {
    payload = body;
  }
  if (payload !== undefined) h["content-length"] = Buffer.byteLength(payload);
  return new Promise((resolvePromise, reject) => {
    const req = https.request(
      {
        method,
        hostname: u.hostname,
        port: u.port || 443,
        path: u.pathname + u.search,
        headers: h,
        ca: caPem(),
        servername: u.hostname,
        lookup,
        timeout,
        agent: false,
      },
      (res) => {
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => {
          const text = Buffer.concat(chunks).toString("utf8");
          resolvePromise({
            status: res.statusCode,
            headers: res.headers,
            text,
            json() {
              return JSON.parse(text);
            },
          });
        });
      },
    );
    req.on("timeout", () => req.destroy(new Error(`timeout ${method} ${url}`)));
    req.on("error", reject);
    if (payload !== undefined) req.write(payload);
    req.end();
  });
}

export async function requestOK(method, url, opts) {
  const res = await request(method, url, opts);
  if (res.status < 200 || res.status > 299) {
    throw new Error(`${method} ${url} -> ${res.status}: ${res.text.slice(0, 500)}`);
  }
  return res;
}

/** Decodes a JWT payload without verifying it (for asserting what a test token carries). */
export function claims(jwt) {
  return JSON.parse(Buffer.from(jwt.split(".")[1], "base64url").toString("utf8"));
}

const tokenURL = `${ISSUER}/protocol/openid-connect/token`;

export async function clientToken(clientId, clientSecret) {
  const res = await requestOK("POST", tokenURL, {
    form: { grant_type: "client_credentials", client_id: clientId, client_secret: clientSecret },
  });
  return res.json().access_token;
}

export async function passwordToken(username, clientId = "e2e-user-token") {
  const res = await requestOK("POST", tokenURL, {
    form: { grant_type: "password", client_id: clientId, username, password: PASSWORD, scope: "openid" },
  });
  return res.json().access_token;
}

export const syncToken = (app) => clientToken(APPS[app].syncClient, APPS[app].syncSecret);

// ------------------------------------------------------------ Keycloak admin
// The bootstrap admin of the e2e Keycloak (fake fixture credentials). Tests use
// it only to inspect state and to inject faults; the product paths use their own
// clients (dashboard-admin, sync clients).
async function masterToken() {
  const res = await requestOK("POST", `${SSO}/realms/master/protocol/openid-connect/token`, {
    form: { grant_type: "password", client_id: "admin-cli", username: "e2e-kc-admin", password: "e2e-fake-kc-admin-password" },
  });
  return res.json().access_token;
}

export async function kc(method, path, json) {
  const token = await masterToken();
  const res = await request(method, `${SSO}/admin/realms/${REALM}${path}`, {
    headers: { authorization: `Bearer ${token}` },
    json,
  });
  if (res.status < 200 || res.status > 299) {
    throw new Error(`Keycloak admin ${method} ${path} -> ${res.status}: ${res.text.slice(0, 300)}`);
  }
  return res.text ? res.json() : null;
}

export async function clientUUID(clientId) {
  const list = await kc("GET", `/clients?clientId=${encodeURIComponent(clientId)}`);
  if (list.length !== 1) throw new Error(`client ${clientId} not found`);
  return list[0].id;
}

export async function directClientRoles(userId, clientId) {
  const uuid = await clientUUID(clientId);
  return (await kc("GET", `/users/${userId}/role-mappings/clients/${uuid}`)).map((r) => r.name);
}

export async function effectiveClientRoles(userId, clientId) {
  const uuid = await clientUUID(clientId);
  return (await kc("GET", `/users/${userId}/role-mappings/clients/${uuid}/composite`)).map((r) => r.name);
}

/** Adds (add=true) or removes a client role mapping on a user. */
export async function setClientRole(userId, clientId, roleName, add) {
  const uuid = await clientUUID(clientId);
  const role = await kc("GET", `/clients/${uuid}/roles/${encodeURIComponent(roleName)}`);
  await kc(add ? "POST" : "DELETE", `/users/${userId}/role-mappings/clients/${uuid}`, [role]);
}

export async function serviceAccountUser(clientId) {
  const uuid = await clientUUID(clientId);
  return kc("GET", `/clients/${uuid}/service-account-user`);
}

export async function findUsers(query) {
  return kc("GET", `/users?${new URLSearchParams({ ...query, exact: "true", briefRepresentation: "false" })}`);
}

// ------------------------------------------------------------ docker compose
function composeEnv() {
  for (const name of ["CLIMS_REPO", "POES_REPO"]) {
    if (!process.env[name]) throw new Error(`set ${name} (the compose file needs it)`);
  }
  return process.env;
}

export function compose(args, { env = {}, input, allowFail = false, timeout = 300000 } = {}) {
  const res = spawnSync(
    "docker",
    ["compose", "-p", "idsync-e2e", "-f", resolve(STACK_DIR, "compose.yaml"), ...args],
    { env: { ...composeEnv(), ...env }, input, encoding: "utf8", timeout, maxBuffer: 64 << 20 },
  );
  if (res.error) throw res.error;
  const out = { code: res.status, stdout: res.stdout, stderr: res.stderr };
  if (!allowFail && res.status !== 0) {
    throw new Error(`docker compose ${args.join(" ")} exited ${res.status}\n${res.stderr}\n${res.stdout}`);
  }
  return out;
}

/**
 * Runs the dashboard CLI (`moh-sso <args>`) in a one-shot "cli" container.
 * envNames are inherited from `env` without putting values on the command line.
 */
export function cli(args, { env = {}, volumes = [], allowFail = false } = {}) {
  const flags = ["run", "--rm", "--no-deps", "-T"];
  for (const name of Object.keys(env)) flags.push("-e", name);
  for (const v of volumes) flags.push("-v", v);
  const res = compose([...flags, "cli", ...args], { env, allowFail: true });
  let parsed;
  try {
    parsed = res.stdout.trim() ? JSON.parse(res.stdout) : undefined;
  } catch {
    parsed = undefined;
  }
  const out = { ...res, json: parsed };
  if (!allowFail && res.code !== 0) {
    throw new Error(`moh-sso ${args.join(" ")} exited ${res.code}\nstderr: ${res.stderr}\nstdout: ${res.stdout}`);
  }
  return out;
}

/** Runs SQL in one of the stack's Postgres containers and returns rows as arrays of strings. */
export function sql({ service, user, name }, query) {
  const res = compose(["exec", "-T", service, "psql", "-U", user, "-d", name, "-v", "ON_ERROR_STOP=1", "-AtF", "\t", "-c", query]);
  return res.stdout.split("\n").filter((l) => l !== "").map((l) => l.split("\t"));
}

export const dashboardDB = { service: "dashboard-postgres", user: "sso_e2e", name: "sso_dashboard_e2e" };

// ------------------------------------------------------------ mailpit
export async function mailTo(address) {
  const res = await requestOK("GET", `${MAIL}/api/v1/search?${new URLSearchParams({ query: `to:"${address}"` })}`);
  return res.json().messages ?? [];
}

export async function mailMessage(id) {
  return (await requestOK("GET", `${MAIL}/api/v1/message/${id}`)).json();
}

// ------------------------------------------------------------ polling
export async function waitFor(fn, { timeout = 30000, interval = 500, what = "condition" } = {}) {
  const deadline = Date.now() + timeout;
  let last;
  for (;;) {
    try {
      const v = await fn();
      if (v) return v;
    } catch (err) {
      last = err;
    }
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}${last ? `: ${last.message}` : ""}`);
    await new Promise((r) => setTimeout(r, interval));
  }
}

// ------------------------------------------------------------ browser trust
/**
 * Chromium cannot be pointed at a CA file, so the browser trusts exactly the
 * key that Caddy's chain uses: Node first verifies the served chain against
 * Caddy's root CA (and nothing else), then the verified intermediate's SPKI is
 * passed to --ignore-certificate-errors-spki-list.
 */
export async function chromiumTrustArgs() {
  const chain = await new Promise((resolvePromise, reject) => {
    const socket = tls.connect(
      { host: "127.0.0.1", port: PORT, servername: "dashboard.e2e.test", ca: caPem(), rejectUnauthorized: true },
      () => {
        const certs = [];
        let cert = socket.getPeerCertificate(true);
        while (cert && cert.raw && !certs.some((c) => c.fingerprint256 === cert.fingerprint256)) {
          certs.push(cert);
          cert = cert.issuerCertificate;
        }
        socket.end();
        resolvePromise(certs);
      },
    );
    socket.on("error", reject);
  });
  const root = new X509Certificate(caPem());
  const spki = (raw) =>
    createHash("sha256").update(new X509Certificate(raw).publicKey.export({ type: "spki", format: "der" })).digest("base64");
  const pins = new Set([spki(root.raw)]);
  for (const c of chain.slice(1)) pins.add(spki(c.raw));
  return [
    "--host-resolver-rules=MAP *.e2e.test 127.0.0.1",
    `--ignore-certificate-errors-spki-list=${[...pins].join(",")}`,
  ];
}
