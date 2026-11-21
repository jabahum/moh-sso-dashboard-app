import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const navigation = JSON.stringify([
  {
    id: "validation",
    label: "Data Validation",
    path: "/apps/dwh/data-validation",
    permission: "data_quality:read",
  },
  {
    id: "visualizer",
    label: "Data Visualizer",
    path: "/apps/dwh/data-visualizer",
    permission: "documents:read",
  },
]);
const user = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "shell-reviewer",
  firstName: "Portal",
  lastName: "Reviewer",
  email: "reviewer@example.invalid",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: [
    "portal:access",
    "systems:launch",
    "users:read",
    "clients:read",
    "data_quality:read",
    "documents:read",
  ],
  systems: ["data-statistics", "settings"],
  accessibleSystems: [
    {
      clientId: "data-statistics",
      displayName: "Data & Statistics",
      launchUrl: "/apps/dwh/data-validation",
      navigation,
      systemType: "platform",
      displayInLauncher: true,
      displayInSideNav: true,
      roles: [],
    },
    {
      clientId: "settings",
      displayName: "Settings",
      launchUrl: "/apps/settings/profile",
      systemType: "platform",
      displayInLauncher: true,
      displayInSideNav: false,
      roles: [],
    },
  ],
  emailVerified: true,
  requirePwdChange: false,
};

const groupedNavigation = [
  ["dashboards", "Dashboards", "report_browser:read"],
  ["data-validation", "Data Validation", "data_quality:read"],
  ["data-visualizer", "Data Visualizer", "documents:read"],
  ["documents", "Document Management", "documents:read"],
  ["surveillance", "Surveillance", "surveillance:read"],
  ["issue-tracker", "Issue Tracker", "issue_tracker:read"],
  ["report-scheduler", "Report Scheduler", "report_scheduler:read"],
].map(([id, label, permission], order) => ({
  id,
  label,
  path: `/apps/dwh/${id}`,
  permission,
  order,
  displayInLauncher: true,
}));
const groupedUser = {
  ...user,
  isAdmin: false,
  realmRoles: ["user"],
  permissions: [
    ...user.permissions,
    "report_browser:read",
    "surveillance:read",
    "issue_tracker:read",
    "report_scheduler:read",
  ],
  accessibleSystems: [
    {
      ...user.accessibleSystems[0],
      category: "platform",
      displayInLauncher: false,
      navigation: JSON.stringify([
        ...groupedNavigation,
        // The same destination must not be repeated in another section.
        { ...groupedNavigation[1], id: "validation-shortcut", category: "support" },
        {
          id: "custom-reference",
          label: "Reference Library",
          path: "/apps/reference-registers/library",
          category: "documents-support",
          displayInLauncher: true,
        },
      ]),
    },
    { ...user.accessibleSystems[1], category: "utilities" },
    {
      clientId: "administration-tools",
      displayName: "Administration tools",
      category: "administration",
      displayInLauncher: false,
      navigation: JSON.stringify([
        {
          id: "users",
          label: "User Management",
          path: "/admin/users",
          permission: "users:read",
          displayInLauncher: true,
        },
        {
          id: "roles",
          label: "Roles & Permissions",
          path: "/admin/rbac",
          permission: "rbac:read",
          displayInLauncher: true,
        },
      ]),
      roles: [],
    },
    {
      clientId: "research-library",
      displayName: "Research Library",
      category: "research",
      launchUrl: "https://research.example.invalid/",
      launchMode: "new_tab",
      systemType: "external",
      displayInLauncher: true,
      roles: [],
    },
    {
      clientId: "legacy-service",
      displayName: "Legacy Service",
      category: "external",
      launchUrl: "https://legacy.example.invalid/",
      launchMode: "same_tab",
      systemType: "external",
      displayInLauncher: true,
      roles: [],
    },
  ],
};

async function fixture(page, account = user) {
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  await page.route("http://127.0.0.1:5417/ssobackend/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/auth/me")
      ? { user: account }
      : path.includes("unread")
        ? { count: 0 }
        : [];
    return route.fulfill({ json: { success: true, data } });
  });
  return failures;
}

for (const [route, navLabel] of [
  ["/admin/users", "Admin navigation"],
  ["/apps/dwh/data-validation", "Application navigation"],
]) {
  test(`mobile navigation closes on Escape and route changes: ${navLabel}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const failures = await fixture(page);
    await page.goto(`/portal${route}`);
    const toggle = page.getByRole("button", { name: "Open navigation", exact: true });
    await expect(toggle).toBeVisible();
    await expect(page.getByRole("navigation", { name: navLabel })).toHaveCount(0);
    await toggle.click();
    const nav = page.getByRole("navigation", { name: navLabel });
    await expect(nav).toBeVisible();
    await expect(page.locator("body")).toHaveCSS("overflow", "hidden");
    await page.keyboard.press("Escape");
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(toggle).toBeFocused();
    await expect(page.locator("body")).not.toHaveCSS("overflow", "hidden");
    await toggle.click();
    const destination = navLabel === "Admin navigation" ? "Clients" : "Data Visualizer";
    const link = nav.getByRole("link", { name: destination, exact: true });
    await expect(link).toHaveAttribute(
      "href",
      navLabel === "Admin navigation"
        ? "/portal/admin/clients"
        : "/portal/apps/dwh/data-visualizer",
    );
    await link.focus();
    await page.keyboard.press("Enter");
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(page.locator("body")).not.toHaveCSS("overflow", "hidden");
    expect(failures).toEqual([]);
  });

  test(`desktop navigation preference survives mobile use: ${navLabel}`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await fixture(page);
    await page.goto(`/portal${route}`);
    await expect(page.getByRole("navigation", { name: navLabel })).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    const toggle = page.getByRole("button", { name: "Open navigation", exact: true });
    await expect(toggle).toBeVisible();
    await toggle.click();
    await expect(page.locator("body")).toHaveCSS("overflow", "hidden");
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(page.locator("body")).not.toHaveCSS("overflow", "hidden");
    await expect(page.getByRole("navigation", { name: navLabel })).toBeVisible();
    await page.getByRole("button", { name: "Collapse navigation", exact: true }).click();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(toggle).toBeVisible();
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(page.getByRole("navigation", { name: navLabel })).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Expand navigation", exact: true }),
    ).toBeVisible();
  });

  test(`launcher search and account panel work: ${navLabel}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const failures = await fixture(page);
    await page.goto(`/portal${route}`);
    const launcher = page.getByRole("button", { name: "Open application menu", exact: true });
    await launcher.click();
    const menu = page.getByRole("dialog", { name: "Application menu", exact: true });
    const search = menu.getByRole("searchbox");
    await expect(search).toBeFocused();
    await search.fill("statistics");
    await expect(menu.getByRole("listitem")).toHaveCount(1);
    await expect(menu.getByRole("status")).toContainText("1 application");
    await search.fill("no-matching-service");
    await expect(menu.getByText("No matching applications")).toBeVisible();
    await menu.getByRole("button", { name: "Clear search", exact: true }).click();
    await expect(search).toHaveValue("");
    await expect(search).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);
    await expect(launcher).toBeFocused();
    const profile = page.getByRole("button", { name: "Signed in as shell-reviewer" });
    await profile.click();
    const account = page.getByRole("dialog", { name: "Your account", exact: true });
    await expect(
      account.getByRole("heading", { name: "Portal Reviewer", exact: true }),
    ).toBeVisible();
    await expect(account.getByRole("button", { name: "Profile and settings" })).toBeVisible();
    await expect(account.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
    await expect(account.getByRole("button", { name: "Maximize panel" })).toHaveCount(0);
    await expect
      .poll(async () => {
        const bounds = await account.boundingBox();
        return bounds
          ? { left: Math.round(bounds.x), right: Math.round(bounds.x + bounds.width) }
          : null;
      })
      .toEqual({ left: 0, right: 390 });
    await page.keyboard.press("Escape");
    await expect(profile).toBeFocused();
    await expect(account).toHaveCount(0);
    for (const panel of await page.locator(".app-side-panel").all()) {
      await expect(panel).toHaveAttribute("inert", "");
    }
    expect(failures).toEqual([]);
  });
}

test("mobile navigation hands focus to the launcher", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await fixture(page);
  await page.goto("/portal/admin/users");
  await page.getByRole("button", { name: "Open navigation", exact: true }).click();
  await page.getByRole("button", { name: "Open application menu", exact: true }).click();
  await expect(page.getByRole("navigation", { name: "Admin navigation" })).toHaveCount(0);
  await expect(
    page.getByRole("dialog", { name: "Application menu" }).getByRole("searchbox"),
  ).toBeFocused();
});

test("public shell has a visible sign-in action on a narrow screen", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  await fixture(page, null);
  await page.goto("/portal/");
  const signIn = page.getByRole("button", { name: "Sign in", exact: true });
  await expect(signIn).toBeVisible();
  await expect(signIn).toContainText("Sign in");
  const box = await signIn.boundingBox();
  expect(Math.round(box.x + box.width)).toBeLessThanOrEqual(320);
  await expect(page.getByRole("heading", { name: "News & Updates" })).toBeVisible();
});

for (const width of [390, 1440]) {
  test(`application groups preserve order, search and launch at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const failures = await fixture(page, groupedUser);
    await page.goto("/portal/apps/dwh/data-validation");
    await page.getByRole("button", { name: "Open application menu", exact: true }).click();
    const menu = page.getByRole("dialog", { name: "Application menu", exact: true });
    const bounds = await menu.boundingBox();
    expect(Math.round(bounds.x)).toBeGreaterThanOrEqual(0);
    expect(Math.round(bounds.x + bounds.width)).toBeLessThanOrEqual(width);
    expect(Math.round(bounds.y + bounds.height)).toBeLessThanOrEqual(900);
    await expect(menu.locator(".app-grid-group__title")).toHaveText([
      "Data & Reporting4",
      "Surveillance1",
      "Documents & Support3",
      "Services & Utilities1",
      "Administration1",
      "Research1",
      "Other applications1",
    ]);
    const reports = menu.getByRole("list", { name: "Data & Reporting", exact: true });
    await expect(reports.getByRole("link")).toHaveText([
      "Dashboards",
      "Data Validation",
      "Data Visualizer",
      "Report Scheduler",
    ]);
    await expect(menu.getByRole("link", { name: "Open Data Validation", exact: true })).toHaveCount(
      1,
    );
    await expect(
      menu.getByRole("link", { name: "Open Roles & Permissions", exact: true }),
    ).toHaveCount(0);
    await expect(menu.getByRole("status")).toHaveText("12 applications available");
    const lastApp = menu.getByRole("link", { name: "Open Legacy Service", exact: true });
    await lastApp.scrollIntoViewIfNeeded();
    await expect(lastApp).toBeVisible();
    expect(await page.evaluate(() => globalThis.window.scrollY)).toBe(0);
    await expect(
      menu
        .getByRole("list", { name: "Documents & Support", exact: true })
        .getByRole("link", { name: "Open Reference Library", exact: true }),
    ).toBeVisible();
    const search = menu.getByRole("searchbox");
    await search.fill("documents & support");
    await expect(menu.getByRole("listitem")).toHaveCount(3);
    await expect(menu.locator(".app-grid-group")).toHaveCount(1);
    await search.fill("visualizer");
    await expect(menu.getByRole("listitem")).toHaveCount(1);
    const link = menu.getByRole("link", { name: "Open Data Visualizer", exact: true });
    await link.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/\/portal\/apps\/dwh\/data-visualizer$/);
    await expect(menu).toHaveCount(0);
    expect(failures).toEqual([]);
  });
}

test("permission filtering hides unavailable groups and their search results", async ({ page }) => {
  await fixture(page, {
    ...groupedUser,
    permissions: groupedUser.permissions.filter(
      (permission) =>
        !["documents:read", "issue_tracker:read", "users:read", "surveillance:read"].includes(
          permission,
        ),
    ),
    accessibleSystems: groupedUser.accessibleSystems.map((system) =>
      system.clientId === "data-statistics"
        ? { ...system, navigation: JSON.stringify(groupedNavigation) }
        : system,
    ),
  });
  await page.goto("/portal/apps/dwh/data-validation");
  await page.getByRole("button", { name: "Open application menu", exact: true }).click();
  const menu = page.getByRole("dialog", { name: "Application menu", exact: true });
  for (const label of ["Documents & Support", "Administration", "Surveillance"]) {
    await expect(menu.getByRole("region", { name: label, exact: true })).toHaveCount(0);
  }
  await menu.getByRole("searchbox").fill("documents & support");
  await expect(menu.getByText("No matching applications")).toBeVisible();
  await expect(menu.getByRole("status")).toContainText("0 applications");
  await menu.getByRole("button", { name: "Clear search", exact: true }).click();
  await expect(menu.getByRole("region", { name: "Data & Reporting", exact: true })).toBeVisible();
});

test("grouped external applications retain their new-tab launch behavior", async ({ page }) => {
  await page.addInitScript(() => {
    globalThis.window.open = (...args) => {
      globalThis.window.__lastOpenedApplication = args;
      return null;
    };
  });
  await fixture(page, groupedUser);
  await page.goto("/portal/apps/dwh/data-validation");
  await page.getByRole("button", { name: "Open application menu", exact: true }).click();
  const menu = page.getByRole("dialog", { name: "Application menu", exact: true });
  await menu.getByRole("searchbox").fill("research");
  await menu.getByRole("link", { name: "Open Research Library in a new tab", exact: true }).click();
  expect(await page.evaluate(() => globalThis.window.__lastOpenedApplication)).toEqual([
    "https://research.example.invalid/",
    "_blank",
    "noopener,noreferrer",
  ]);
  await expect(page).toHaveURL(/\/portal\/apps\/dwh\/data-validation$/);
  await expect(menu).toHaveCount(0);
});

test("nested sidebar groups and pages share label columns at every level", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await fixture(page, {
    ...user,
    accessibleSystems: [
      {
        ...user.accessibleSystems[0],
        navigation: JSON.stringify([
          ...JSON.parse(navigation),
          {
            id: "issue-tracking",
            label: "Issue Tracking",
            children: [
              {
                id: "issue-summary",
                label: "Issue Summary",
                path: "/apps/dwh/issue-tracker/dashboard",
              },
            ],
          },
          {
            id: "self-service",
            label: "Self Service",
            children: [
              { id: "timesheet", label: "My Timesheet", path: "/apps/utilities/timesheet" },
              {
                id: "requests",
                label: "eService Requests",
                children: [
                  {
                    id: "service-access",
                    label: "Service Access",
                    path: "/apps/utilities/service-access",
                  },
                  {
                    id: "equipment",
                    label: "Equipment Request",
                    path: "/apps/utilities/equipment",
                  },
                ],
              },
            ],
          },
        ]),
      },
    ],
  });
  await page.goto("/portal/apps/dwh/data-validation");
  const nav = page.getByRole("navigation", { name: "Application navigation" });
  const selfService = nav.getByRole("button", { name: "Self Service", exact: true });
  await selfService.click();
  const requests = nav.getByRole("button", { name: "eService Requests", exact: true });
  await requests.click();
  const labelX = (row) =>
    row
      .locator(".cds--side-nav__submenu-title, .cds--side-nav__link-text")
      .evaluate((label) => label.getBoundingClientRect().x);
  expect(
    Math.abs(
      (await labelX(selfService)) -
        (await labelX(nav.getByRole("link", { name: "Data Validation", exact: true }))),
    ),
  ).toBeLessThan(1);
  expect(
    Math.abs(
      (await labelX(requests)) -
        (await labelX(nav.getByRole("link", { name: "My Timesheet", exact: true }))),
    ),
  ).toBeLessThan(1);
  const access = nav.getByRole("link", { name: "Service Access", exact: true });
  const equipment = nav.getByRole("link", { name: "Equipment Request", exact: true });
  expect(Math.abs((await labelX(access)) - (await labelX(equipment)))).toBeLessThan(1);
  expect(await labelX(access)).toBeGreaterThan(await labelX(requests));
  const bounds = await nav.boundingBox();
  for (const row of [selfService, requests, access, equipment]) {
    const box = await row.boundingBox();
    expect(box.x + box.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1);
  }
  await page.screenshot({ path: "/tmp/moh-sidebar-indentation.png" });
});

for (const sidePanelDocking of [true, false]) {
  test(`account panel respects desktop docking configuration (${sidePanelDocking})`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await fixture(page);
    await page.route("**/portal/config.js", async (route) => {
      const response = await route.fetch();
      const body = await response.text();
      await route.fulfill({
        response,
        body: `${body}\nwindow.__APP_CONFIG__.sidePanelDocking = ${sidePanelDocking};`,
      });
    });
    await page.goto("/portal/apps/dwh/data-validation");

    const main = page.locator(".user-layout #main-content");
    const profile = page.getByRole("button", { name: "Signed in as shell-reviewer" });
    await expect(main).toBeVisible();
    const initialWidth = (await main.boundingBox()).width;
    await profile.click();

    const panel = page.getByRole("dialog", { name: "Your account", exact: true });
    await expect(panel.getByRole("button", { name: "Close panel", exact: true })).toBeFocused();
    if (sidePanelDocking) {
      await expect(panel).not.toHaveAttribute("aria-modal", "true");
      const content = await main.boundingBox();
      expect(content.width).toBeLessThan(initialWidth);
      expect(content.x + content.width).toBeLessThanOrEqual((await panel.boundingBox()).x);
    } else {
      await expect(panel).toHaveAttribute("aria-modal", "true");
      expect((await main.boundingBox()).width).toBe(initialWidth);
    }

    await page.keyboard.press("Escape");
    await expect(profile).toBeFocused();
    expect((await main.boundingBox()).width).toBe(initialWidth);
  });
}
