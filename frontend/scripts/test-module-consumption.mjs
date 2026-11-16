/* global console, process */
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { closeViteTestServer, failOnOpenTestResources, transformOnlyViteOptions } from "./vite-test-server.mjs";
import { frontendRoot, moduleSelection, workspaceModules } from "./module-source.mjs";

const fixture = mkdtempSync(join(tmpdir(), "moh-npm-environments-"));
const originals = ["config.js", "import-map.json", "version-manifest.json"].map((name) => {
  const path = join(frontendRoot, "public", name);
  return [path, readFileSync(path)];
});
const savedEnv = { ...process.env };
let server;
try {
  // Real npm tarballs exercise the published artifact boundary without requiring
  // an npm release or altering the user's installed module cache.
  const versions = Object.fromEntries(workspaceModules().map(({ pkg }) => [pkg.name, pkg.version]));
  const configPath = join(fixture, "module-sources.json");
  writeFileSync(
    configPath,
    JSON.stringify({
      development: { source: "npm", versions },
      production: { source: "npm", versions },
    }),
  );
  const selection = moduleSelection("production", {
    FRONTEND_MODULE_SOURCE: "npm",
    FRONTEND_MODULE_DIRECTORY: fixture,
    FRONTEND_MODULE_CONFIG: configPath,
  });
  mkdirSync(join(fixture, "node_modules"));
  for (const name of readdirSync(join(frontendRoot, "node_modules"))) {
    if (name.startsWith(".") || name === "@moh-sso") continue;
    symlinkSync(join(frontendRoot, "node_modules", name), join(fixture, "node_modules", name));
  }
  for (const { group, name, pkg } of selection.modules) {
    const packed = JSON.parse(
      execFileSync(
        "npm",
        [
          "pack",
          join(frontendRoot, group, name),
          "--json",
          "--ignore-scripts",
          "--offline",
          "--pack-destination",
          fixture,
          "--cache",
          join(fixture, "cache"),
        ],
        { cwd: frontendRoot, encoding: "utf8" },
      ),
    )[0];
    const directory = join(fixture, "node_modules", pkg.name);
    mkdirSync(directory, { recursive: true });
    execFileSync("tar", [
      "-xzf",
      join(fixture, packed.filename),
      "--strip-components=1",
      "-C",
      directory,
    ]);
  }
  writeFileSync(
    join(fixture, "package.json"),
    JSON.stringify({
      name: "moh-installed-frontend-modules",
      private: true,
      dependencies: selection.versions,
    }),
  );
  Object.assign(process.env, {
    FRONTEND_MODULE_SOURCE: "npm",
    FRONTEND_MODULE_DIRECTORY: fixture,
    FRONTEND_MODULE_CONFIG: configPath,
    FRONTEND_MODULE_ENV: "development",
    FRONTEND_ASSET_BASE_URL: "/portal",
    FRONTEND_BASE_PATH: "/portal",
  });
  server = await createServer({
    configFile: join(frontendRoot, "vite.config.ts"),
    mode: "development",
    ...transformOnlyViteOptions(),
  });
  const result = await server.transformRequest("/src/main.tsx");
  assert.ok(
    result?.code.includes("@moh-sso/auth/dist/index.js"),
    "Development shell must resolve AuthBootstrap from a packed npm export",
  );
  await closeViteTestServer(server);
  server = undefined;
  console.log("Development shell resolves packed npm artifacts.");
  process.env.FRONTEND_MODULE_ENV = "production";
  const run = (script, ...args) =>
    execFileSync(process.execPath, [join(frontendRoot, "scripts", script), ...args], {
      cwd: frontendRoot,
      env: process.env,
      stdio: "inherit",
    });
  run("use-runtime-config.mjs", "production");
  run("frontend-modules.mjs", "build");
  const manifest = JSON.parse(
    readFileSync(join(frontendRoot, "dist/version-manifest.json"), "utf8"),
  );
  for (const { group, name, pkg } of selection.modules) {
    assert.equal(manifest[group][name].version, selection.versions[pkg.name]);
    const directory = group === "apps" ? "mf" : "packages";
    assert.ok(
      existsSync(
        join(
          frontendRoot,
          "dist/portal",
          directory,
          name,
          group === "apps" ? "single-spa.js" : "index.js",
        ),
      ),
    );
  }
  run("verify-production-runtime.mjs");
  console.log("Production shell, staged modules and manifest use the selected npm artifacts.");
} finally {
  try {
    await closeViteTestServer(server);
  } finally {
    for (const [path, contents] of originals) writeFileSync(path, contents);
    for (const key of Object.keys(process.env)) if (!(key in savedEnv)) delete process.env[key];
    Object.assign(process.env, savedEnv);
    rmSync(fixture, { recursive: true, force: true });
  }
}

failOnOpenTestResources();
