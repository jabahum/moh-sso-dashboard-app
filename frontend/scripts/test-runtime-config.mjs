import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { URL } from "node:url";
import { runtimeConfig, selectedRuntimeConfig } from "./runtime-config-check.mjs";

const html = readFileSync(new URL("../apps/shell/index.html", import.meta.url), "utf8");

test("the shell selects development locally and production on the shared host", () => {
  assert.equal(selectedRuntimeConfig(html, "localhost"), "/portal/config.development.js");
  assert.equal(selectedRuntimeConfig(html, "127.0.0.1"), "/portal/config.development.js");
  assert.equal(
    selectedRuntimeConfig(html, "dashboards.health.go.ug"),
    "/portal/config.production.js",
  );
});

test("runtime validation reads configuration values independently of JS formatting", () => {
  const config = runtimeConfig(
    "window.__APP_CONFIG__={API_BASE_URL:'/ssobackend', singleSpaOrchestration:true};",
  );
  assert.equal(config.API_BASE_URL, "/ssobackend");
  assert.equal(config.singleSpaOrchestration, true);
});
