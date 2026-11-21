import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const rootPages = [
  ["/admin/home", "Admin Overview"],
  ["/admin/systems", "Systems"],
  ["/admin/notifications/deliveries", "Notification Deliveries"],
  ["/admin/users", "Users"],
  ["/admin/clients", "Clients"],
  ["/admin/audit-logs", "Audit Logs"],
  ["/admin/announcements", "Announcements"],
  ["/admin/emails", "Email Outbox"],
  ["/admin/rbac", "RBAC Management"],
  ["/apps/dwh/data-visualizer", "Data Visualizer"],
  ["/apps/dwh/data-validation", "Data Validation"],
  ["/apps/dwh/documents", "Document Management"],
  ["/apps/dwh/issue-tracker", "Issue Tracker"],
  ["/apps/dwh/dashboards", "Report Browser"],
  ["/apps/dwh/report-scheduler", "Report Scheduler"],
  ["/apps/dwh/surveillance", "Surveillance Dashboard"],
  ["/apps/eservices/ihris/meeting-manager", "Meeting Manager"],
  ["/apps/utilities/self-service/timesheet", "Timesheet"],
];

const user = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "root-layout-admin",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: [
    "portal:access",
    "systems:read",
    "systems:launch",
    "users:read",
    "clients:read",
    "audit:read",
    "announcements:read",
    "email:read",
    "notifications:read",
    "rbac:read",
    "data_quality:read",
    "documents:read",
    "document_templates:read",
    "issue_tracker:read",
    "report_browser:read",
    "report_scheduler:read",
    "surveillance:read",
  ],
  systems: ["data-statistics", "eservices", "utilities"],
  accessibleSystems: [],
  emailVerified: true,
  requirePwdChange: false,
};

function fixtureData(path) {
  return path.endsWith("/auth/me")
    ? { user }
    : path.endsWith("/admin/audit-logs/actions")
      ? { actions: [] }
      : path.endsWith("/admin/audit-logs")
        ? { items: [], has_more: false, next_cursor: {} }
        : path.endsWith("/admin/rbac/drift")
          ? {
              source: "fixture",
              systems: [],
              realmRoles: [],
              warnings: [],
              summary: {
                systemsInSync: 0,
                systemsMissingInRbac: 0,
                systemsStaleInRbac: 0,
                rolesMissingInRbac: 0,
                rolesStaleInRbac: 0,
                realmRolesMissing: 0,
              },
            }
          : path.includes("unread")
            ? { count: 0 }
            : path.includes("geojson")
              ? { type: "FeatureCollection", features: [] }
              : [];
}

for (const width of [1440, 390]) {
  for (const [path, title] of rootPages) {
    test(`${title} has one aligned root header at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      const failures = [];
      page.on("pageerror", (error) => failures.push(error.message));
      await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
        const path = new URL(route.request().url()).pathname;
        const data = fixtureData(path);
        await route.fulfill({ json: { success: true, data } });
      });
      // The portal header is under test; the separately hosted report app is not.
      await page.route("https://dashboards.health.go.ug/**", (route) =>
        route.fulfill({ contentType: "text/html", body: "<p>Report browser fixture</p>" }),
      );

      // Root pages must mount without a live CDN, using lockfile dependencies.
      await page.route("https://esm.sh/**", (route) => route.abort("blockedbyclient"));
      await page.goto(`/portal${path}`);
      const main = page.locator("#main-content");
      if (path.startsWith("/admin/")) await expect(main.locator(".moh-page-shell")).toHaveCount(1);
      const header = main.locator(".moh-page-shell__header");
      await expect(header).toHaveCount(1);
      const heading = header.getByRole("heading", { name: title, exact: true });
      await expect(heading).toBeVisible();
      await expect(main.getByRole("heading", { name: title, exact: true })).toHaveCount(1);
      const containerBounds = await main.boundingBox();
      const headingBounds = await heading.boundingBox();
      const padding = await main.evaluate((element) =>
        parseFloat(element.ownerDocument.defaultView.getComputedStyle(element).paddingLeft),
      );
      expect(Math.abs(headingBounds.x - containerBounds.x - padding)).toBeLessThanOrEqual(2);
      expect(headingBounds.y - containerBounds.y).toBeGreaterThanOrEqual(0);
      expect(headingBounds.y - containerBounds.y).toBeLessThan(100);
      expect(headingBounds.x + headingBounds.width).toBeLessThanOrEqual(width);
      await expect(header.locator(".moh-page-shell__subtitle")).toBeVisible();
      expect(failures).toEqual([]);
      if (path.startsWith("/admin/")) {
        // Fields must remain inside the workspace; Carbon Stack uses a grid
        // unless wrapping filter layouts explicitly choose flex.
        const filters = main.locator(".user-filters, .client-filters, .audit-log-filters, .announcement-filter-grid, .email-outbox-filter-grid, .notification-delivery-filters");
        if (await filters.count()) {
          const fields = await filters.locator(":scope > *").evaluateAll((elements) => elements.map((element) => {
            const { x, y, width, height } = element.getBoundingClientRect();
            return { x, y, width, height };
          }));
          for (const [index, field] of fields.entries()) {
            expect(field.x + field.width).toBeLessThanOrEqual(width);
            for (const other of fields.slice(index + 1)) {
              const overlapX = Math.min(field.x + field.width, other.x + other.width) - Math.max(field.x, other.x);
              const overlapY = Math.min(field.y + field.height, other.y + other.height) - Math.max(field.y, other.y);
              expect(overlapX > 1 && overlapY > 1).toBe(false);
            }
          }
        }
        await page.screenshot({ path: testInfo.outputPath("workspace.png"), fullPage: true });
      }
    });
  }
}

// API delays and errors used to remove the page header on these admin routes.
for (const width of [1440, 390]) {
  for (const [path, title] of rootPages.filter(([path]) =>
    [
      "/admin/users",
      "/admin/clients",
      "/admin/announcements",
      "/admin/emails",
      "/admin/rbac",
    ].includes(path),
  )) {
    test(`${title} keeps its shell while loading and after an API error at ${width}px`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height: 900 });
      let fail = true;
      let release;
      const pending = new Promise((resolve) => {
        release = resolve;
      });
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
        const apiPath = new URL(route.request().url()).pathname;
        if (!apiPath.endsWith("/auth/me") && !apiPath.includes("unread")) {
          await pending;
          if (fail)
            return route.fulfill({
              status: 500,
              json: { success: false, error: { message: "Fixture API unavailable" } },
            });
        }
        await route.fulfill({ json: { success: true, data: fixtureData(apiPath) } });
      });
      await page.goto(`/portal${path}`);
      const main = page.locator("#main-content");
      const shell = main.locator(".moh-page-shell");
      const header = shell.locator(":scope > .moh-page-shell__header");
      const heading = header.getByRole("heading", { name: title, exact: true });
      await expect(heading).toBeVisible();
      await expect(shell).toHaveCount(1);
      const before = await heading.boundingBox();
      release();
      if (path !== "/admin/rbac") {
        await expect(main.getByRole("heading", { name: /Failed to load/ })).toBeVisible();
      } else {
        await expect(main.getByText("Loading RBAC mappings...", { exact: true })).toHaveCount(0);
      }
      await expect(heading).toBeVisible();
      await expect(shell).toHaveCount(1);
      const after = await heading.boundingBox();
      expect(Math.abs(after.x - before.x)).toBeLessThanOrEqual(1);
      expect(Math.abs(after.y - before.y)).toBeLessThanOrEqual(1);
      fail = false;
      await page.reload();
      await expect(heading).toBeVisible();
      await expect(shell).toHaveCount(1);
      if (path !== "/admin/rbac")
        await expect(main.getByRole("heading", { name: /Failed to load/ })).toHaveCount(0);
      expect(errors).toEqual([]);
    });
  }
}

// Search must cover the complete loaded collection, before slicing the current page.
for (const width of [1440, 390]) {
  for (const resource of ["users", "clients"]) {
    test(`${resource} search, filters and reset work at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      const records = Array.from({ length: 15 }, (_, index) => ({
        id: `record-${index}`,
        username: `person-${index}`,
        email: `person-${index}@example.invalid`,
        firstName: index === 14 ? "Target" : "Person",
        name: index === 14 ? "Target application" : `Application ${index}`,
        clientId: `application-${index}`,
        enabled: index !== 14,
        publicClient: true,
        realmRoles: ["user"],
      }));
      await page.route("http://127.0.0.1:5417/ssobackend/**", (route) => {
        const path = new URL(route.request().url()).pathname;
        const data = path.endsWith(`/${resource}`) ? records : fixtureData(path);
        return route.fulfill({ json: { success: true, data } });
      });
      await page.goto(`/portal/admin/${resource}`);
      const main = page.locator("#main-content");
      await expect(main.getByRole("status")).toContainText(`15 of 15 ${resource}`);
      await main.getByRole("searchbox", { name: `Search ${resource}`, exact: true }).fill("TARGET");
      await expect(main.getByRole("status")).toContainText(`1 of 15 ${resource}`);
      await expect(
        main.getByRole("cell", {
          name: resource === "users" ? "person-14" : "Target application",
          exact: true,
        }),
      ).toBeVisible();
      await main.getByRole("combobox", { name: "Status", exact: true }).click();
      await main
        .getByRole("option", { name: resource === "users" ? "active" : "Enabled", exact: true })
        .click();
      await expect(main.getByRole("heading", { name: `No ${resource} found` })).toBeVisible();
      await main.getByRole("button", { name: "Clear filters", exact: true }).first().click();
      await expect(main.getByRole("status")).toContainText(`15 of 15 ${resource}`);
      await expect(
        main.getByRole("searchbox", { name: `Search ${resource}`, exact: true }),
      ).toHaveValue("");
      if (resource === "users") {
        const toggle = main.getByRole("button", { name: "Never logged in" });
        await toggle.click();
        await expect(toggle).toHaveAttribute("aria-pressed", "true");
      }
      const overflowing = await main.evaluate(
        (element) => element.scrollWidth > element.clientWidth + 1,
      );
      expect(overflowing).toBe(false);
    });
  }
}

test("Systems panel docks beside the table and becomes an overlay on narrow screens", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.setViewportSize({ width: 1440, height: 900 });
  const system = { clientId: "dashboard-web", displayName: "Integrated Health Portal", enabled: true };
  await page.route("**/ssobackend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/admin/rbac/systems")
      ? [system]
      : path.endsWith("/admin/rbac/systems/dashboard-web")
        ? { ...system, roles: [], accessRoles: [] }
        : fixtureData(path);
    await route.fulfill({ json: { success: true, data } });
  });
  await page.goto("/portal/admin/systems");

  const main = page.locator("#main-content");
  const manage = main.getByRole("button", { name: "Manage" });
  const table = main.locator(".moh-data-table-shell__table-scroll");
  await expect(manage).toBeVisible();
  const fullWidth = (await main.boundingBox()).width;

  await manage.focus();
  await manage.press("Enter");

  const panel = page.getByRole("dialog", { name: "System: Integrated Health Portal" });
  await expect(panel.getByRole("button", { name: "Close panel", exact: true })).toBeFocused();
  await expect(panel).not.toHaveAttribute("aria-modal", "true");
  const drawer = await panel.boundingBox();
  expect((await main.boundingBox()).width).toBeLessThan(fullWidth);
  const tableBounds = await table.boundingBox();
  expect(tableBounds.x + tableBounds.width).toBeLessThanOrEqual(drawer.x);

  const search = main.getByRole("searchbox");
  await search.click();
  await expect(search).toBeFocused();
  await expect(panel).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();
  await expect(manage).toBeFocused();
  expect((await main.boundingBox()).width).toBe(fullWidth);

  await manage.press("Enter");
  await page.setViewportSize({ width: 390, height: 900 });
  await expect(panel).toHaveAttribute("aria-modal", "true");
  await expect(page.getByRole("button", { name: "Close panel backdrop" })).toBeVisible();
  expect((await panel.boundingBox()).width).toBe(390);
});
