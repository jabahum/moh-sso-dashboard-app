import assert from "node:assert/strict";
import test from "node:test";
import { verifyFrontend } from "./verify-frontend-deployment.mjs";

const names = ["announcements", "audit", "clients", "data-validation", "data-visualizer",
  "documents", "e-services", "email", "issue-tracker", "report-browser", "rbac",
  "surveillance", "users", "utilities", "api", "auth", "config", "microfrontend",
  "state", "types", "ui", "utils"];

function fixture(failure) {
  const imports = Object.fromEntries(names.map((name) => ["@moh-sso/" + name, "/portal/mf/" + name + ".js"]));
  if (failure === "missing import") delete imports["@moh-sso/users"];
  return async (url) => {
    if (url.pathname === "/portal/") {
      return new Response('<div id="root"></div><script src="/portal/assets/index-123.js"></script><link rel="stylesheet" href="/portal/assets/index-123.css">', { headers: { "content-type": "text/html" } });
    }
    if (url.pathname.endsWith("import-map.json")) {
      return new Response(JSON.stringify({ imports }), { headers: { "content-type": "application/json" } });
    }
    if (url.pathname.endsWith("/users.js")) {
      if (failure === "404") return new Response("missing", { status: 404 });
      if (failure === "SPA fallback") return new Response("<!doctype html><html></html>", { headers: { "content-type": "text/html" } });
      if (failure === "disguised HTML") return new Response("<html></html>", { headers: { "content-type": "application/javascript" } });
    }
    if (url.pathname.endsWith(".css")) {
      return new Response("body { margin: 0; }", { headers: { "content-type": "text/css" } });
    }
    const body = url.pathname.endsWith("config.js") ? "window.__APP_CONFIG__ = {};" : "export const mount = () => {};";
    return new Response(body, { headers: { "content-type": "application/javascript" } });
  };
}

test("accepts a complete staged frontend", async () => {
  assert.equal(await verifyFrontend("http://localhost:3000/portal/", fixture()), 27);
});
for (const failure of ["404", "SPA fallback", "disguised HTML", "missing import"]) {
  test("rejects " + failure, async () => {
    await assert.rejects(verifyFrontend("http://localhost:3000/portal/", fixture(failure)));
  });
}

