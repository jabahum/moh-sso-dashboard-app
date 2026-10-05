import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { check, finish, readJson, root } from "./workspace-lib.mjs";

const failures = [];
const requiredApps = [
  "announcements",
  "audit",
  "clients",
  "data-validation",
  "data-visualizer",
  "documents",
  "e-services",
  "email",
  "issue-tracker",
  "report-browser", "report-scheduler",
  "rbac",
  "surveillance",
  "users",
  "utilities",
];
const requiredPackages = ["api", "auth", "config", "microfrontend", "state", "types", "ui", "utils"];

const htmlPath = join(root, "dist", "index.html");
check(existsSync(htmlPath), "dist/index.html is missing", failures);

if (existsSync(htmlPath)) {
  const html = readFileSync(htmlPath, "utf8");
  check(html.includes('src="/portal/config.js"'),
    "dist/index.html should load the selected runtime config from /portal/config.js",
    failures,
  );
  check(html.includes('type="importmap"'), "dist/index.html should inline an import map", failures);
  check(html.includes("/portal/assets/"), "dist/index.html should reference /portal assets", failures);
}

const importMapPath = join(root, "dist", "import-map.json");
check(existsSync(importMapPath), "dist/import-map.json is missing", failures);

if (existsSync(importMapPath)) {
  const imports = readJson(importMapPath).imports ?? {};
  for (const app of requiredApps) {
    check(imports[`@moh-sso/${app}`]?.startsWith("/portal/mf/"), `import map should point @moh-sso/${app} at /portal/mf`, failures);
    check(existsSync(join(root, "dist", "portal", "mf", app, "single-spa.js")), `staged /portal/mf/${app}/single-spa.js is missing`, failures);
  }
  for (const pkg of requiredPackages) {
    check(imports[`@moh-sso/${pkg}`]?.startsWith("/portal/packages/"), `import map should point @moh-sso/${pkg} at /portal/packages`, failures);
    check(existsSync(join(root, "dist", "portal", "packages", pkg, "index.js")), `staged /portal/packages/${pkg}/index.js is missing`, failures);
  }
}

check(existsSync(join(root, "dist", "portal", "version-manifest.json")), "dist/portal/version-manifest.json is missing", failures);
const portalConfigPath = join(root, "dist", "portal", "config.js");
check(existsSync(portalConfigPath), "dist/portal/config.js is missing", failures);

if (existsSync(portalConfigPath)) {
  const config = readFileSync(portalConfigPath, "utf8");
  const expectedConfig = readFileSync(join(root, "public", "config.production.js"), "utf8");
  check(config === expectedConfig, "staged runtime config must match config.production.js", failures);
  check(config.includes('microfrontendMode: "remote"'), "production config should use remote microfrontend mode", failures);
  check(config.includes("singleSpaOrchestration: true"), "production config should enable single-spa orchestration", failures);
  check(config.includes('microfrontendMountMode: "orchestrated"'), "production config should use orchestrated mount mode", failures);
}

finish("Production runtime artifact check", failures);
