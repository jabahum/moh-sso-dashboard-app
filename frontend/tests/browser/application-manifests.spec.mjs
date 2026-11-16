/* global window, PopStateEvent */
import { test, expect } from "@playwright/test";
import { Buffer } from "node:buffer";
import { readFileSync } from "node:fs";
import { URL } from "node:url";

const document = readFileSync(
  new URL("../../../examples/manifests/external.yaml", import.meta.url),
  "utf8",
);
const admin = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "manifest-admin",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  systems: ["example-reports"],
  accessibleSystems: [],
  permissions: [
    "portal:access",
    "systems:read",
    "systems:launch",
    "rbac:read",
    "rbac:write",
    "rbac:roles:write",
  ],
  emailVerified: true,
  requirePwdChange: false,
};

for (const width of [1280, 390]) {
  test(`manifest import requires a current reviewed preview at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const writes = [];
    const failures = [];
    page.on("pageerror", (error) => failures.push(error.message));
    await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      let data = [];
      if (path.endsWith("/auth/me")) data = { user: admin };
      else if (path.endsWith("/application-manifests/preview"))
        data = {
          operation: "create",
          reviewToken: "review-fixture",
          manifest: {
            metadata: { id: "health-bi", name: "Health BI", version: "1.0.0" },
            spec: { environment: "development" },
          },
          after: {
            clientId: "health-bi",
            enabled: false,
            launchUrl: "https://health-bi.example.org/",
          },
          rolesToCreate: ["report_viewer"],
          accessRoles: ["report_viewer"],
          warnings: ["Review the application lifecycle before activation."],
        };
      else if (path.endsWith("/application-manifests/import")) {
        writes.push(route.request().postDataJSON());
        data = { operation: "create", manifest: { metadata: { id: "health-bi" } } };
      } else if (path.includes("unread")) data = { count: 0 };
      await route.fulfill({ json: { success: true, data } });
    });
    await page.goto("/portal/admin/systems");
    await page.getByRole("button", { name: "Import manifest", exact: true }).click();
    await page
      .getByLabel("YAML or JSON manifest")
      .setInputFiles({
        name: "app.yaml",
        mimeType: "application/yaml",
        buffer: Buffer.from(document),
      });
    const editor = page.getByLabel("Manifest document", { exact: true });
    await expect(editor).toHaveValue(document);
    await page.getByRole("button", { name: "Validate and preview" }).click();
    const apply = page.getByRole("button", { name: "Import reviewed manifest" });
    await expect(apply).toBeDisabled();
    await page
      .getByText("I reviewed the application, environment, URLs and access roles.", { exact: true })
      .click();
    await expect(apply).toBeEnabled();
    await editor.fill(document + "\n");
    await expect(apply).toHaveCount(0);
    expect(writes).toHaveLength(0);
    await page.getByRole("button", { name: "Validate and preview" }).click();
    await page
      .getByText("I reviewed the application, environment, URLs and access roles.", { exact: true })
      .click();
    await page.getByRole("button", { name: "Import reviewed manifest" }).click();
    await expect(page.getByText("Manifest imported", { exact: true })).toBeVisible();
    expect(writes).toHaveLength(1);
    expect(writes[0]).toEqual({ document: document + "\n", reviewToken: "review-fixture" });
    expect(failures).toEqual([]);
  });
}

for (const authorized of [true, false]) {
  test(`manifest microfrontend ${authorized ? "mounts and unmounts" : "rejects direct access"}`, async ({
    page,
  }) => {
    const failures = [];
    let entryRequests = 0;
    page.on("pageerror", (error) => failures.push(error.message));
    await page.route("https://reports.example.org/single-spa.js", async (route) => {
      entryRequests++;
      await route.fulfill({
        contentType: "application/javascript",
        headers: { "access-control-allow-origin": "*" },
        body: `export async function bootstrap() {}\nexport async function mount(props) { props.domElement.textContent = 'Approved reports at ' + props.basename; }\nexport async function unmount(props) { window.__manifestUnmounts = (window.__manifestUnmounts || 0) + 1; props.domElement.textContent = ''; }`,
      });
    });
    await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      let data = [];
      if (path.endsWith("/auth/me")) data = { user: admin };
      else if (path.endsWith("/applications/example-reports/manifest")) {
        if (!authorized)
          return route.fulfill({
            status: 403,
            json: {
              success: false,
              error: { code: "FORBIDDEN", message: "Application access is required" },
            },
          });
        data = {
          id: "example-reports",
          name: "Example reports",
          version: "1.0.0",
          launch: {
            mode: "microfrontend",
            appName: "@moh-extensions/example-reports",
            entryUrl: "https://reports.example.org/single-spa.js",
            route: "/apps/extensions/example-reports",
          },
        };
      } else if (path.includes("unread")) data = { count: 0 };
      await route.fulfill({ json: { success: true, data } });
    });
    await page.goto("/portal/apps/extensions/example-reports/chapter-one");
    if (authorized) {
      await expect(
        page.getByText("Approved reports at /portal/apps/extensions/example-reports", {
          exact: true,
        }),
      ).toBeVisible();
      await page.reload();
      await expect(
        page.getByText("Approved reports at /portal/apps/extensions/example-reports", {
          exact: true,
        }),
      ).toBeVisible();
      await page.evaluate(() => {
        window.history.pushState({}, "", "/portal/apps/news");
        window.dispatchEvent(new PopStateEvent("popstate"));
      });
      await expect
        .poll(() => page.evaluate(() => window.__manifestUnmounts ?? 0))
        .toBeGreaterThan(0);
      expect(entryRequests).toBeGreaterThan(0);
    } else {
      await expect(page.getByText("Application unavailable", { exact: true })).toBeVisible();
      expect(entryRequests).toBe(0);
    }
    expect(failures).toEqual([]);
  });
}
