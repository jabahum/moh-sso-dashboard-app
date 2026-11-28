import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const user = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "rbac-admin",
  isAdmin: true,
  isUser: true,
  enabled: true,
  realmRoles: ["admin"],
  clientRoles: {},
  permissions: [
    "portal:access",
    "rbac:read",
    "rbac:write",
    "rbac:roles:write",
    "systems:read",
    "systems:write",
  ],
  systems: [],
  accessibleSystems: [],
  emailVerified: true,
  requirePwdChange: false,
};

const system = {
  clientId: "alerts",
  displayName: "Alerts Manager",
  enabled: true,
  lifecycleStatus: "active",
};

const revocation = {
  id: "rev-1",
  applicationId: "alerts",
  issuer: "https://idp.example",
  subject: "user-123",
  revokedBy: "rbac-admin",
  reason: "Left the programme",
  revokedAt: "2026-11-20T08:00:00Z",
  roleRemoved: "done",
  consentsRevoked: "done",
  sessionsRevoked: "done",
  attempts: 1,
  updatedAt: "2026-11-20T08:00:00Z",
  complete: true,
  sessionsStillOpen: false,
};

// Clearing a revocation asks for confirmation in the portal's own modal, not
// the browser's native confirm box, and does nothing when cancelled.
for (const width of [1440, 390]) {
  test(`Clearing an identity-sync revocation confirms in a portal modal at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const nativeDialogs = [];
    page.on("dialog", (dialog) => {
      nativeDialogs.push(dialog.message());
      void dialog.dismiss();
    });
    const restores = [];
    await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const ok = (data) => route.fulfill({ json: { success: true, data } });
      if (path.endsWith("/auth/me")) return ok({ user });
      if (path.endsWith("/admin/rbac/systems")) return ok([system]);
      if (path.endsWith("/admin/rbac/systems/alerts"))
        return ok({ ...system, roles: [], accessRoles: [] });
      if (path.endsWith("/identity-sync")) return ok(null);
      if (path.endsWith("/identity-sync/runs")) return ok({ runs: [], jobs: [] });
      if (path.endsWith("/identity-sync/revocations"))
        return ok({ revocations: [revocation], jobs: [] });
      if (path.endsWith("/identity-sync/revocations/restore") && request.method() === "POST") {
        restores.push(request.postDataJSON());
        return ok({ ...revocation, restoredAt: "2026-11-28T10:00:00Z" });
      }
      if (path.includes("unread")) return ok({ count: 0 });
      return ok([]);
    });

    await page.goto("/admin/rbac");
    const main = page.locator("#main-content");
    await main.getByRole("tab", { name: "Identity sync", exact: true }).click();
    const card = main.getByRole("region", { name: "Identity sync revocations" });
    await expect(card.getByRole("cell", { name: "user-123" })).toBeVisible();
    await card
      .getByRole("textbox", { name: "Reason for clearing a revocation" })
      .fill("CHG-2: rejoined");

    const clear = card.getByRole("button", { name: "Clear revocation", exact: true });
    // ReusableModal's dialog has no accessible name, so find it by its heading.
    const modal = page
      .getByRole("dialog")
      .filter({ has: page.getByRole("heading", { name: "Clear revocation" }) });

    await clear.click();
    await expect(modal).toBeVisible();
    await modal.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(modal).toBeHidden();
    expect(restores).toEqual([]);

    await clear.click();
    await modal.getByRole("button", { name: "Clear revocation", exact: true }).click();
    await expect(modal).toBeHidden();
    await expect(card.getByText("Revocation cleared. Access was not granted again.")).toBeVisible();
    expect(restores).toEqual([{ subject: "user-123", reason: "CHG-2: rejoined" }]);

    expect(nativeDialogs).toEqual([]);
    expect(errors).toEqual([]);
  });
}
