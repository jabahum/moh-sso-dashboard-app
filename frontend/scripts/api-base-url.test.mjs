import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { URL } from "node:url";
import { runInNewContext } from "node:vm";
import ts from "typescript";

function compile(path) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8");
  return ts.transpileModule(source.replaceAll("import.meta.env", "buildEnv"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
}
const config = compile("../packages/config/src/lib/constants/api.constants.ts");
const launchCode = compile("../packages/auth/src/auth/system-launch-url.ts");
const launchExports = {};
runInNewContext(launchCode, { exports: launchExports, URL });

function loadAPI(runtime, buildEnv = {}) {
  const exports = {};
  const context = { exports, buildEnv };
  if (runtime !== undefined) context.window = { __APP_CONFIG__: { API_BASE_URL: runtime } };
  runInNewContext(config, context);
  return exports.API;
}

for (const [value, service] of [
  [undefined, ""],
  ["/api", ""],
  ["/api///", ""],
  ["/", ""],
  ["/ssobackend", "/ssobackend"],
  [" /ssobackend/ ", "/ssobackend"],
  ["/ssobackend/api/", "/ssobackend"],
  ["/backend", "/backend"],
  ["http://localhost:9000/", "http://localhost:9000"],
  ["https://example.org/api", "https://example.org"],
  ["https://example.org/backend/api/", "https://example.org/backend"],
]) {
  test(`API endpoints resolve once for ${value ?? "the default"}`, () => {
    const api = loadAPI(value);
    assert.equal(api.serviceBase, service);
    assert.equal(api.root, `${service}/api`);
    assert.equal(api.base, `${service}/api/v1`);
    assert.equal(api.auth.me(), `${service}/api/v1/auth/me`);
    assert.equal(api.auth.login(), `${service}/api/v1/auth/login`);
    assert.equal(api.build.version(), `${service}/version`);
    const launch = launchExports.resolveSystemLaunchUrl(
      { authenticatedLaunchUrl: "/api/v1/auth/launch?client=fixture" },
      { apiBaseUrl: api.serviceBase, hasPortalSession: true },
    );
    assert.equal(launch, `${service}/api/v1/auth/launch?client=fixture`);
  });
}

test("runtime configuration takes precedence over the build-time service URL", () => {
  assert.equal(
    loadAPI("/api", { VITE_API_BASE_URL: "https://old.example/backend" }).base,
    "/api/v1",
  );
});

test("build-time origins and visualizer overrides retain their behavior", () => {
  const api = loadAPI(undefined, {
    VITE_API_BASE_URL: "http://localhost:9000",
    VITE_VISUALIZER_API_BASE: "/visualizer-proxy",
  });
  assert.equal(api.base, "http://localhost:9000/api/v1");
  assert.equal(api.visualizer.themes(), "/visualizer-proxy/visualizer/themes");
  assert.equal(api.issue.list(), "/visualizer-proxy/issues");
  assert.equal(
    api.dataValidation.dqa.tables.list(),
    "/visualizer-proxy/data-validation/dqa/tables",
  );

  const underscoreApi = loadAPI(undefined, {
    VITE_API_BASE_URL: "http://localhost:9000",
    VITE_VISUALIZER_API_BASE: "/visualizer_proxy/",
  });
  assert.equal(underscoreApi.visualizer.datasets(), "/visualizer_proxy/visualizer/datasets");
  assert.equal(underscoreApi.issue.list(), "/visualizer_proxy/issues");
});
