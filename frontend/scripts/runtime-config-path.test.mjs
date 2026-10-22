import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, realpath, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath, URL } from "node:url";
import { build, createServer } from "vite";

const root = fileURLToPath(new URL("../", import.meta.url));

function assertConfigPath(html) {
  const match = html.match(/<script\b[^>]*\bsrc=["']([^"']*config\.js)["']/);
  assert.ok(match, "runtime config script must be present");
  assert.equal(new URL(match[1], "http://localhost:3000/portal/").pathname, "/portal/config.js");
}

test("Vite serves and builds the shell with exactly one /portal config prefix", async () => {
  const fixture = await realpath(await mkdtemp(join(tmpdir(), "moh-config-path-")));
  let server;
  try {
    const html = await readFile(join(root, "apps/shell/index.html"), "utf8");
    await writeFile(join(fixture, "index.html"), html);
    await mkdir(join(fixture, "src"));
    await writeFile(join(fixture, "src/main.tsx"), "export {};\n");
    const config = {
      configFile: false,
      root: fixture,
      publicDir: join(root, "public"),
      base: "/portal/",
      logLevel: "silent",
    };
    server = await createServer({
      ...config,
      server: { middlewareMode: true, hmr: false, watch: null },
    });
    assertConfigPath(await server.transformIndexHtml("/portal/", html));
    await server.close();
    server = null;
    await build({ ...config, build: { outDir: join(fixture, "dist") } });
    assertConfigPath(await readFile(join(fixture, "dist/index.html"), "utf8"));
  } finally {
    await server?.close();
    await rm(fixture, { recursive: true, force: true });
  }
});
