import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const admin = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "ci-admin",
  email: "ci@example.invalid",
  firstName: "CI",
  lastName: "Admin",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: ["portal:access", "users:read", "clients:read"],
  systems: [],
  accessibleSystems: [],
  emailVerified: true,
  requirePwdChange: false,
};

async function fixture(page, user) {
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") failures.push(message.text());
  });
  page.on("requestfailed", (request) => {
    if (request.failure()?.errorText !== "net::ERR_ABORTED")
      failures.push(`${request.url()}: ${request.failure()?.errorText}`);
  });
  page.on("response", (response) => {
    if (response.status() >= 400) failures.push(`HTTP ${response.status()}: ${response.url()}`);
  });
  // Only the disposable local API is mocked. Production shell, module, CSS and
  // shared runtime assets execute unchanged; no live application API is used.
  await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const data = path.endsWith("/auth/me") ? { user } : path.includes("unread") ? { count: 0 } : [];
    await route.fulfill({ json: { success: true, data } });
  });
  return failures;
}

test("public portal starts without browser errors", async ({ page }) => {
  const failures = await fixture(page, null);
  await page.goto("/portal/");
  await expect(page.getByRole("heading", { name: "News & Updates" })).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(failures).toEqual([]);
});

test("admin can mount users, navigate to clients and reload a module route", async ({ page }) => {
  const failures = await fixture(page, admin);
  await page.goto("/portal/admin/users");
  await expect(page.getByRole("heading", { name: "Users", exact: true })).toBeVisible();
  await page
    .getByRole("navigation", { name: "Admin navigation" })
    .getByText("Clients", { exact: true })
    .click();
  await expect(page).toHaveURL(/\/portal\/admin\/clients/);
  await expect(page.getByRole("heading", { name: "Clients", exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "Clients", exact: true })).toBeVisible();
  await page.waitForLoadState("networkidle");
  expect(failures).toEqual([]);
});
