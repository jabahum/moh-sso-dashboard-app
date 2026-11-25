/* global Crypto */
import { test, expect } from "@playwright/test";
import { URL } from "node:url";
import { Buffer } from "node:buffer";

async function fixture(page) {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() => {
    // HTTP on a LAN/Tailscale IP does not expose this secure-context API.
    Object.defineProperty(Crypto.prototype, "randomUUID", { value: undefined, configurable: true });
  });
  const user = {
    id: "00000000-0000-0000-0000-000000000001",
    username: "http-reviewer",
    enabled: true,
    emailVerified: true,
    isAdmin: true,
    isUser: true,
    realmRoles: ["admin"],
    clientRoles: {},
    permissions: [
      "portal:access",
      "systems:launch",
      "report_scheduler:read",
      "report_scheduler:history",
      "email:read",
      "email:write",
    ],
    systems: ["data-statistics"],
    accessibleSystems: [],
  };
  await page.route("**/ssobackend/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path.includes("/report-scheduler"))
      return route.fulfill({
        status: 503,
        json: { success: false, error: { message: "Scheduler fixture unavailable" } },
      });
    const data = path.endsWith("/auth/me") ? { user } : path.includes("unread") ? { count: 0 } : [];
    return route.fulfill({ json: { success: true, data } });
  });
  return errors;
}

for (const width of [390, 1440]) {
  test(`scheduler errors show a toast without randomUUID at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = await fixture(page);
    await page.goto("/apps/dwh/report-scheduler");
    await expect(
      page.getByRole("heading", { name: "Report Scheduler", exact: true }),
    ).toBeVisible();
    const toast = page.locator(".moh-toast-stack .cds--toast-notification");
    await expect(toast).toHaveCount(1);
    await expect(toast).toContainText("Report Scheduler is unavailable");
    await toast.getByRole("button").click();
    await expect(toast).toHaveCount(0);
    await expect(page.getByText(/^App Failed:/)).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test("identical email attachments get independent UI IDs without randomUUID", async ({ page }) => {
  const errors = await fixture(page);
  await page.goto("/admin/emails");
  await page.getByRole("button", { name: "Send email", exact: true }).click();
  const panel = page.getByRole("dialog", { name: "Send email", exact: true });
  const file = {
    name: "diagnostic.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("test attachment"),
  };
  await panel.getByLabel("Upload files", { exact: true }).setInputFiles([file, file]);
  const attachments = panel.locator(".email-form__attachment");
  await expect(attachments).toHaveCount(2);
  const inlineIds = await attachments
    .getByRole("checkbox", { name: "Inline", exact: true })
    .evaluateAll((inputs) => inputs.map((input) => input.id));
  expect(new Set(inlineIds).size).toBe(2);
  await attachments
    .first()
    .getByRole("button", { name: /Remove/ })
    .click();
  await expect(attachments).toHaveCount(1);
  await expect(attachments.first()).toContainText("diagnostic.txt");
  expect(errors).toEqual([]);
});
