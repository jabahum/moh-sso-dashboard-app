import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const applications = [
  ["data-statistics", "Data Validation", "/apps/dwh/data-validation", "Data & statistics"],
  ["surveillance", "Surveillance", "/apps/dwh/surveillance", "Data & statistics"],
  ["documents", "Document Management", "/apps/dwh/documents", "Data & statistics"],
  ["reports", "Report Scheduler", "/apps/dwh/report-scheduler", "Reporting"],
  ["training", "Training Portal", "https://training.example.invalid/", "Learning"],
  ["cases", "Case Management", "/apps/outbreak/case-management", "Outbreak management"],
  ["timesheets", "Timesheets", "/apps/utilities/self-service/timesheet", "Self service"],
  ["knowledge", "Knowledge Base", "/apps/utilities/self-service/elearning", "Learning"],
].map(([clientId, displayName, launchUrl, category]) => ({
  clientId,
  displayName,
  launchUrl,
  category,
  roles: [],
  systemType: launchUrl.startsWith("https") ? "external" : "platform",
  launchMode: launchUrl.startsWith("https") ? "new_tab" : "internal",
  displayInLauncher: true,
  displayInSideNav: false,
}));
const admin = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "overview-admin",
  firstName: "Jane",
  lastName: "Nabirye",
  email: "jane.nabirye@example.invalid",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: [
    "portal:access",
    "systems:launch",
    "users:read",
    "users:write",
    "clients:read",
    "clients:write",
    "rbac:read",
    "announcements:write",
    "notifications:read",
    "notifications:write",
    "data_quality:read",
  ],
  systems: applications.map((app) => app.clientId),
  accessibleSystems: applications,
  lastLoginAt: "2026-11-07T07:20:00Z",
};

async function fixture(page, user = admin) {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/auth/me")
      ? { user }
      : path.includes("unread")
        ? { count: 1 }
        : path.endsWith("/notifications")
          ? [
              {
                id: "fixture",
                title: "Access request reviewed",
                message: "An application access request was reviewed.",
                severity: "info",
                created_at: "2026-11-07T07:30:00Z",
                read: false,
              },
            ]
          : [];
    await route.fulfill({ json: { success: true, data } });
  });
  await page
    .context()
    .route("https://training.example.invalid/**", (route) =>
      route.fulfill({ contentType: "text/html", body: "<h1>Training fixture</h1>" }),
    );
  return errors;
}

for (const width of [1440, 390]) {
  test(`Overview is compact, searchable and usable at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    const errors = await fixture(page);
    await page.goto("/portal/admin/home");
    const main = page.locator("#main-content");
    await expect(main.getByRole("heading", { name: "Admin Overview", exact: true })).toBeVisible();
    await expect(main.locator(".moh-page-shell")).toHaveCount(1);
    await expect(main.locator(".app-tile")).toHaveCount(8);
    await expect(main.getByRole("button", { name: "Add user", exact: true })).toBeVisible();
    await expect(main.locator(".home-actions .quick-action")).toHaveCount(6);
    await expect(main.locator(".home-signals .signal-tile")).toHaveCount(4);
    await expect(
      main.locator(".signal-tile").filter({ hasText: "Unread notifications" }),
    ).toContainText("1");
    const card = main.locator(".app-tile").first();
    expect((await card.boundingBox()).height).toBeLessThan(220);
    expect(await main.evaluate((node) => node.scrollWidth <= node.clientWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath("overview.png"), fullPage: true });
    const search = main.getByRole("searchbox", { name: "Search your applications" });
    await search.fill("docuMENT");
    await expect(main.locator(".app-tile")).toHaveCount(1);
    await expect(main.getByRole("status").filter({ hasText: "1 of 8 applications" })).toBeVisible();
    await search.fill("no-matching-app");
    await expect(main.getByRole("heading", { name: "No matching applications" })).toBeVisible();
    await main.getByRole("button", { name: "Clear search", exact: true }).click();
    await expect(main.locator(".app-tile")).toHaveCount(8);
    const create = main.getByRole("button", { name: "Add user", exact: true });
    await create.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("dialog", { name: "Create user", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Close panel", exact: true }).click();
    await main.getByRole("button", { name: "Open Data Validation", exact: true }).click();
    await expect(page).toHaveURL(/\/portal\/apps\/dwh\/data-validation$/);
    expect(errors).toEqual([]);
  });
}

test("Overview respects permissions, empty access and unavailable identity metadata", async ({
  page,
}) => {
  const errors = await fixture(page, {
    ...admin,
    permissions: ["portal:access"],
    accessibleSystems: [],
    systems: [],
    lastLoginAt: "invalid-date",
  });
  await page.goto("/portal/admin/home");
  const main = page.locator("#main-content");
  await expect(main.getByRole("heading", { name: "No applications assigned" })).toBeVisible();
  await expect(main.getByRole("heading", { name: "Quick actions" })).toHaveCount(0);
  await expect(main.getByRole("heading", { name: "Notifications", exact: true })).toHaveCount(0);
  await expect(main.getByText(/Invalid Date|Last sign-in:/)).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("External application launch announces and opens a new tab", async ({ page }) => {
  const errors = await fixture(page);
  await page.goto("/portal/admin/home");
  const popup = page.waitForEvent("popup");
  await page
    .getByRole("button", { name: "Open Training Portal in a new tab", exact: true })
    .click();
  const tab = await popup;
  await tab.waitForLoadState();
  expect(tab.url()).toBe("https://training.example.invalid/");
  await tab.close();
  expect(errors).toEqual([]);
});
