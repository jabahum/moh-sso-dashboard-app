import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { URL } from "node:url";
import { runtimeConfig, selectedRuntimeConfig } from "./runtime-config-check.mjs";

const html = readFileSync(new URL("../apps/shell/index.html", import.meta.url), "utf8");

test("the shell loads the selected runtime config on local and production hosts", () => {
  assert.equal(selectedRuntimeConfig(html, "localhost"), "/portal/config.js");
  assert.equal(selectedRuntimeConfig(html, "127.0.0.1"), "/portal/config.js");
  assert.equal(selectedRuntimeConfig(html, "dashboards.health.go.ug"), "/portal/config.js");
});

test("runtime validation reads configuration values independently of JS formatting", () => {
  const config = runtimeConfig(
    "window.__APP_CONFIG__={API_BASE_URL:'/ssobackend', singleSpaOrchestration:true};",
  );
  assert.equal(config.API_BASE_URL, "/ssobackend");
  assert.equal(config.singleSpaOrchestration, true);
});

test("runtime selection also supports a hostname-based inline loader", () => {
  const loader = `<script>
    const file = window.location.hostname === "localhost" ? "config.development.js" : "config.production.js";
    document.write('<script src="/portal/' + file + '"><' + '/script>');
  </script>`;
  assert.equal(selectedRuntimeConfig(loader, "localhost"), "/portal/config.development.js");
  assert.equal(
    selectedRuntimeConfig(loader, "portal.example.invalid"),
    "/portal/config.production.js",
  );
});
