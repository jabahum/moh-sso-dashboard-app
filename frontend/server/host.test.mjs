/* global fetch, process, URL */
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "node:http";
import { once } from "node:events";
import { Script } from "node:vm";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { createHost, hostOptions } from "./host.mjs";

async function listen(t, server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  );
  return `http://127.0.0.1:${server.address().port}`;
}

async function fixture(t, overrides = {}) {
  const dir = await mkdtemp(join(tmpdir(), "moh-host-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await mkdir(join(dir, "assets"));
  await mkdir(join(dir, "mf", "users"), { recursive: true });
  await writeFile(join(dir, "assets", "shell.js"), "export {};");
  await writeFile(join(dir, "mf", "users", "single-spa.js"), "export const mount = () => {};");
  await writeFile(
    join(dir, "index.html"),
    '<html><head><script type="importmap">{"imports":{"old":"/old.js"}}</script></head><body><script type="module" src="/portal/assets/shell.js"></script></body></html>',
  );
  const mapPath = join(dir, "import-map.json");
  const writeMap = (url) =>
    writeFile(mapPath, JSON.stringify({ imports: { "@moh-sso/users": url } }));
  await writeMap("/mf/users/single-spa.js");
  const app = await createHost({
    ...hostOptions(),
    distDir: dir,
    importMapSource: mapPath,
    ...overrides,
  });
  const origin = await listen(t, createServer(app));
  return { origin, writeMap, dir };
}

test("host startup reports an occupied port and exits unsuccessfully", async (t) => {
  const { origin, dir } = await fixture(t);
  assert.throws(
    () =>
      execFileSync(process.execPath, [fileURLToPath(new URL("./index.mjs", import.meta.url))], {
        encoding: "utf8",
        env: {
          ...process.env,
          HOST: "127.0.0.1",
          PORT: new URL(origin).port,
          FRONTEND_DIST_DIR: dir,
        },
        stdio: ["ignore", "pipe", "pipe"],
      }),
    (error) => {
      assert.equal(error.status, 1);
      assert.match(error.stderr, /Unable to start frontend host:.*EADDRINUSE/);
      assert.doesNotMatch(error.stdout, /listening/);
      return true;
    },
  );
});

test("serves fresh map, runtime settings, static assets, and deep links", async (t) => {
  const { origin, writeMap } = await fixture(t);
  const page = await fetch(`${origin}/portal/admin/users`);
  const html = await page.text();
  assert.equal(page.status, 200);
  assert.equal(page.headers.get("cache-control"), "no-store");
  assert.match(html, /hostRuntime/);
  assert.match(html, /\/mf\/users\/single-spa.js/);
  assert.doesNotMatch(html, /old.js/);
  assert.equal((html.match(/type="importmap"/g) || []).length, 1);
  assert.ok(html.indexOf('type="importmap"') < html.indexOf('type="module"'));
  await writeMap("https://assets.example.com/users/2/single-spa.js");
  assert.match(
    await (await fetch(`${origin}/portal/index.html`)).text(),
    /users\/2\/single-spa.js/,
  );
  assert.equal((await fetch(`${origin}/portal/assets/shell.js`)).status, 200);
  assert.equal((await fetch(`${origin}/mf/users/single-spa.js`)).status, 200);
  for (const path of [
    "/portal/assets/missing.js",
    "/portal/mf/missing",
    "/unknown",
    "/portal/missing.json",
  ]) {
    assert.equal((await fetch(origin + path)).status, 404, path);
  }
  const config = await (await fetch(`${origin}/portal/config.production.js`)).text();
  assert.match(config, /"API_BASE_URL":"\/backend"/);
  assert.match(config, /"singleSpaOrchestration":true/);
  const redirect = await fetch(`${origin}/portal`, { redirect: "manual" });
  assert.equal(redirect.status, 302);
  assert.equal(redirect.headers.get("location"), "/portal/");
  assert.equal((await fetch(`${origin}/portal/`)).status, 200);
  const worker = await (await fetch(`${origin}/portal/sw.js`)).text();
  assert.doesNotThrow(() => new Script(worker));
});

test("selected module config mounts remotely without global orchestration", async (t) => {
  const { origin } = await fixture(t, { moduleName: "@moh-sso/users" });
  const config = await (await fetch(`${origin}/portal/config.development.js`)).text();
  assert.match(config, /"standaloneModule":"@moh-sso\/users"/);
  assert.match(config, /"singleSpaOrchestration":false/);
  assert.match(config, /"microfrontendMode":"remote"/);
  const missing = await fixture(t, { moduleName: "@moh-sso/missing" });
  assert.equal((await fetch(`${missing.origin}/portal/`)).status, 503);
});

test("central map URLs resolve against the map origin and inline JSON is escaped", async (t) => {
  const central = await listen(
    t,
    createServer((_req, res) => {
      res.setHeader("Content-Type", "application/json");
      res.end(
        JSON.stringify({ imports: { "@moh-sso/users": "./users.js", "</script>": "/vendor.js" } }),
      );
    }),
  );
  const { origin } = await fixture(t, { importMapSource: `${central}/maps/import-map.json` });
  const map = await (await fetch(`${origin}/portal/import-map.json`)).json();
  assert.equal(map.imports["@moh-sso/users"], `${central}/maps/users.js`);
  const html = await (await fetch(`${origin}/portal/`)).text();
  assert.match(html, /\\u003c\/script>/);
  assert.doesNotMatch(html, /"<\/script>"/);
});

test("proxies API paths, query, request body, status, and cookies", async (t) => {
  const backend = await listen(
    t,
    createServer(async (req, res) => {
      let body = "";
      for await (const chunk of req) body += chunk;
      res.writeHead(201, {
        "Content-Type": "application/json",
        "Set-Cookie": "session=value; Domain=backend.example; Path=/prefix; HttpOnly",
      });
      res.end(
        JSON.stringify({ url: req.url, method: req.method, body, cookie: req.headers.cookie }),
      );
    }),
  );
  const { origin } = await fixture(t, { backend: `${backend}/prefix` });
  const response = await fetch(`${origin}/backend/api/v1/example?q=1`, {
    method: "POST",
    body: "payload",
    headers: { Cookie: "session=original" },
  });
  assert.equal(response.status, 201);
  assert.deepEqual(await response.json(), {
    url: "/prefix/api/v1/example?q=1",
    method: "POST",
    body: "payload",
    cookie: "session=original",
  });
  assert.equal(response.headers.get("set-cookie"), "session=value; Path=/; HttpOnly");
});

test("invalid or unavailable central maps fail closed", async (t) => {
  const central = await listen(
    t,
    createServer((_req, res) => {
      res.writeHead(503);
      res.end();
    }),
  );
  const { origin } = await fixture(t, { importMapSource: `${central}/map.json` });
  assert.equal((await fetch(`${origin}/portal/`)).status, 503);
  assert.equal((await fetch(`${origin}/portal/import-map.json`)).status, 503);
});
