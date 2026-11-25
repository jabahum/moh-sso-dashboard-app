/* global document, window */
import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const base = "/apps/dwh/surveillance";
const weekId = "00000000-0000-0000-0000-000000000053";
const diseases = [
  { id: "measles", name: "Measles" },
  { id: "malaria", name: "Malaria" },
  { id: "paralysis", name: "Acute Flaccid Paralysis" },
];
const user = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "navigation-reviewer",
  enabled: true,
  emailVerified: true,
  requirePwdChange: false,
  isAdmin: true,
  isUser: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: [
    "portal:access",
    "systems:read",
    "systems:launch",
    "surveillance:read",
    "documents:read",
    "document_templates:read",
  ],
  systems: ["data-statistics", "utilities"],
  accessibleSystems: [],
};

async function fixture(page, hybrid = false) {
  if (hybrid) {
    await page.route("**/config.js", (route) =>
      route.fulfill({
        contentType: "application/javascript",
        body: 'window.__APP_CONFIG__ = { API_BASE_URL: "/ssobackend", microfrontendMode: "local", singleSpaOrchestration: false, microfrontendMountMode: "hybrid" };',
      }),
    );
  }
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/auth/me")
      ? { user }
      : path.endsWith("/notifications/count/unread")
        ? { count: 0 }
        : path.endsWith("/surveillance/weeks")
          ? [{ id: weekId, epi_year: new Date().getFullYear(), epi_week: 1 }]
          : path.endsWith("/surveillance/diseases")
            ? diseases
            : path.includes("/weekly-statuses/")
              ? diseases.map((disease) => ({
                  id: disease.id,
                  disease_id: disease.id,
                  disease_name: disease.name,
                  epi_week_id: weekId,
                  status: "RED",
                }))
              : path.includes("geojson")
                ? { type: "FeatureCollection", features: [] }
                : [];
    await route.fulfill({ json: { success: true, data } });
  });
  return errors;
}

for (const width of [390, 1440]) {
  test(`disease tile and breadcrumb use one surveillance path at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    const errors = await fixture(page);
    await page.goto(base);
    await expect(
      page.getByRole("heading", { name: "Surveillance Dashboard", exact: true }),
    ).toBeVisible();
    for (const [label, slug] of [
      ["Measles", "measles"],
      ["Malaria", "malaria"],
      ["Acute Flaccid Paralysis", "acute-flaccid-paralysis"],
    ]) {
      await page
        .locator(".surveillance-tile")
        .getByRole("button", { name: label, exact: true })
        .click();
      await expect(page).toHaveURL(`${base}/${slug}?weekId=${weekId}`);
      await expect(
        page
          .locator(".disease-details-page__header")
          .getByRole("heading", { name: label, exact: true }),
      ).toBeVisible();
      const breadcrumb = page.getByRole("navigation", { name: "Breadcrumb", exact: true });
      await expect(breadcrumb).toHaveCount(1);
      await expect(breadcrumb.locator(".moh-breadcrumbs__current")).toHaveText(label);
      await expect(
        breadcrumb.getByRole("link", { name: "Surveillance", exact: true }),
      ).toHaveAttribute("href", "/apps/dwh/surveillance");
      await breadcrumb.getByRole("link", { name: "Surveillance", exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`${base}/?$`));
      await expect(
        page.getByRole("heading", { name: "Surveillance Dashboard", exact: true }),
      ).toBeVisible();
    }
    expect(errors).toEqual([]);
  });
}

test("malformed accumulated paths recover to the app root before a tile click", async ({
  page,
}) => {
  await fixture(page);
  await page.goto(`${base}/apps/utilities/self-service/eservice/document-upload/malaria/measles`);
  await expect(page).toHaveURL(new RegExp(`${base}/?$`));
  await page
    .locator(".surveillance-tile")
    .getByRole("button", { name: "Measles", exact: true })
    .click();
  await expect(page).toHaveURL(`${base}/measles?weekId=${weekId}`);
});

test("relevant document links leave the surveillance router at the portal root", async ({
  page,
}) => {
  await fixture(page);
  await page.goto(`${base}/measles?weekId=${weekId}`);
  const link = page.getByRole("link", { name: "Measles Surveillance Guidelines", exact: true });
  await expect(link).toHaveAttribute(
    "href",
    "/apps/utilities/self-service/eservice/document-upload",
  );
  await link.click();
  await expect(page).toHaveURL("/apps/utilities/self-service/eservice/document-upload");
});

test("direct disease detail loads use the single shell breadcrumb", async ({ page }) => {
  await fixture(page);
  await page.goto(`${base}/measles?weekId=${weekId}`);
  const breadcrumb = page.getByRole("navigation", { name: "Breadcrumb", exact: true });
  await expect(breadcrumb).toHaveCount(1);
  await expect(breadcrumb.locator(".moh-breadcrumbs__current")).toHaveText("Measles");
  await expect(page.locator(".disease-details-page nav")).toHaveCount(0);
});

for (const hybrid of [false, true]) {
  test(`detail breadcrumb follows embedded navigation in ${hybrid ? "hybrid" : "orchestrated"} mode`, async ({
    page,
  }) => {
    const errors = await fixture(page, hybrid);
    await page.goto(base);
    await page
      .locator(".surveillance-tile")
      .getByRole("button", { name: "Measles", exact: true })
      .click();
    await expect(page.locator(".moh-breadcrumbs__current")).toHaveText("Measles");
    await expect(page.getByLabel("Reporting week", { exact: true })).toHaveValue(weekId);
    await expect(
      page.getByRole("heading", { name: "Weekly case trend", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("No facility reports for the selected week and location.", { exact: false }),
    ).toBeVisible();
    await page.goBack();
    await expect(page.locator(".moh-breadcrumbs__current")).toHaveText("Surveillance");
    expect(errors).toEqual([]);
  });
}

for (const width of [390, 1440]) {
  test(`disease reports and week selection remain usable at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    const errors = await fixture(page);
    const otherWeek = "00000000-0000-0000-0000-000000000054";
    await page.route("**/ssobackend/api/v1/surveillance/weeks*", (route) =>
      route.fulfill({
        json: {
          success: true,
          data: [
            { id: weekId, epi_year: 2026, epi_week: 1 },
            { id: otherWeek, epi_year: 2026, epi_week: 53 },
          ],
        },
      }),
    );
    await page.route(
      "**/ssobackend/api/v1/surveillance/facility-weekly-metrics/disease-trend?*",
      (route) =>
        route.fulfill({
          json: {
            success: true,
            data: [
              { epi_year: 2026, epi_week: 1, total_cases: 12 },
              { epi_year: 2026, epi_week: 53, total_cases: 7 },
            ],
          },
        }),
    );
    await page.route("**/ssobackend/api/v1/surveillance/facility-weekly-metrics/week/**", (route) =>
      route.fulfill({
        json: {
          success: true,
          data: route.request().url().includes(otherWeek)
            ? []
            : [
                {
                  id: "report",
                  facility_id: "facility",
                  facility_name: "Review Health Centre",
                  region_id: "north",
                  region_name: "Northern",
                  district_name: "Review District",
                  disease_id: "measles",
                  disease_name: "Measles",
                  epi_week_id: weekId,
                  metric_value: 12,
                },
              ],
        },
      }),
    );
    await page.route("**/ssobackend/api/v1/surveillance/alerts?*", (route) =>
      route.fulfill({
        json: {
          success: true,
          data: route.request().url().includes(otherWeek)
            ? []
            : [
                {
                  id: "alert",
                  disease_id: "measles",
                  epi_week_id: weekId,
                  status: "RED",
                  narrative: "Review this alert",
                },
              ],
        },
      }),
    );
    await page.goto(`${base}/measles?weekId=${weekId}&regionId=north`);
    await expect(page.locator(".disease-details-page__stats dd").first()).toHaveText("12");
    await expect(page.getByText("Review Health Centre", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("region", { name: "Disease alerts", exact: true }).getByRole("columnheader"),
    ).toHaveText(["District", "Region", "Status", "Narrative"]);
    await expect(
      page.getByRole("region", { name: "Facility reports", exact: true }).getByRole("columnheader"),
    ).toHaveText(["Facility", "Region", "District", "Subcounty", "Cases", "Actions"]);
    await expect(page.getByRole("button", { name: "View trends", exact: true })).toBeVisible();

    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);

    await expect(page.locator(".disease-details-page__chart svg").first()).toBeVisible();
    await expect(page.locator(".disease-details-page__chart .bar")).toHaveCount(2);
    await page.getByLabel("Reporting week", { exact: true }).selectOption(otherWeek);
    await expect(page).toHaveURL(`${base}/measles?weekId=${otherWeek}&regionId=north`);
    await expect(page.locator(".moh-breadcrumbs__current")).toHaveText("Measles");
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
    await page.screenshot({ path: `/tmp/moh-disease-details-${width}.png`, fullPage: true });
    expect(errors).toEqual([]);
  });
}

test("failed report requests show errors instead of confirmed zero counts", async ({ page }) => {
  await fixture(page);
  await page.route("**/ssobackend/api/v1/surveillance/facility-weekly-metrics/**", (route) =>
    route.fulfill({ status: 500, json: { success: false } }),
  );
  await page.goto(`${base}/measles?weekId=${weekId}`);
  await expect(
    page.getByRole("alert").filter({ hasText: "Facility reports could not be loaded" }),
  ).toBeVisible();
  await expect(
    page.getByRole("alert").filter({ hasText: "The weekly trend could not be loaded" }),
  ).toBeVisible();
  await expect(page.locator(".disease-details-page__stats dd").nth(1)).toHaveText("—");
  await expect(
    page.getByText("No weekly reports available for this year and location."),
  ).toHaveCount(0);
});
