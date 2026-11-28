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
  permissions: ["portal:access", "rbac:read", "rbac:write", "rbac:roles:write", "systems:read"],
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

const catalogue = [
  {
    name: "identity:read",
    description: "Read the calling client's own identity and effective scopes.",
    tokens: ["machine", "user"],
    endpoints: ["GET /api/v1/platform/identity"],
  },
  {
    name: "notifications:write",
    description: "Queue email notifications to individual recipients.",
    tokens: ["machine"],
    endpoints: ["POST /api/v1/platform/notifications"],
  },
];

// The Machine clients tab lists bindings with catalogue descriptions and binds a
// new client: look up its service account, pick scopes, give a reason, save.
for (const width of [1440, 390]) {
  test(`Machine clients lists and binds platform scopes at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const identities = [
      {
        applicationId: "alerts",
        clientId: "alerts-identity-sync",
        subject: "sa-sync",
        scopes: [],
        enabled: true,
      },
    ];
    const saved = [];
    await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (path.endsWith("/auth/me"))
        return route.fulfill({ json: { success: true, data: { user } } });
      if (path.endsWith("/admin/rbac/systems"))
        return route.fulfill({ json: { success: true, data: [system] } });
      if (path.endsWith("/admin/rbac/systems/alerts"))
        return route.fulfill({
          json: { success: true, data: { ...system, roles: [], accessRoles: [] } },
        });
      if (path.endsWith("/admin/platform/scopes"))
        return route.fulfill({ json: { success: true, data: catalogue } });
      if (path.endsWith("/admin/platform/applications/alerts/identities"))
        return route.fulfill({ json: { success: true, data: { identities } } });
      if (path.endsWith("/identities/alerts-platform/service-account"))
        return route.fulfill({
          json: { success: true, data: { clientId: "alerts-platform", subject: "sa-platform" } },
        });
      if (path.endsWith("/identities/alerts-platform") && request.method() === "PUT") {
        const body = request.postDataJSON();
        saved.push(body);
        identities.push({ applicationId: "alerts", clientId: "alerts-platform", ...body });
        return route.fulfill({
          json: {
            success: true,
            data: { applicationId: "alerts", clientId: "alerts-platform", ...body },
          },
        });
      }
      if (path.includes("unread"))
        return route.fulfill({ json: { success: true, data: { count: 0 } } });
      return route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto("/admin/rbac");
    const main = page.locator("#main-content");
    await main.getByRole("tab", { name: "Machine clients", exact: true }).click();
    const panel = main.getByRole("region", { name: "Machine clients" });
    await expect(panel.getByRole("cell", { name: "alerts-identity-sync" })).toBeVisible();
    await expect(panel.getByText("No platform scopes")).toBeVisible();

    await panel.getByRole("button", { name: "Bind a machine client" }).click();
    const save = panel.getByRole("button", { name: "Save", exact: true });
    await expect(save).toBeDisabled();
    await panel.getByRole("textbox", { name: "Keycloak client ID" }).fill("alerts-platform");
    await panel.getByRole("button", { name: "Look up service account" }).click();
    await expect(panel.getByText("sa-platform")).toBeVisible();

    // Every machine-grantable scope is offered with its catalogue description.
    await expect(panel.getByText(catalogue[0].description)).toBeVisible();
    await expect(panel.getByText(catalogue[1].description)).toBeVisible();
    await panel.getByText("identity:read", { exact: true }).click();
    await panel.getByText("notifications:write", { exact: true }).click();
    await panel
      .getByRole("textbox", { name: "Reason for this change" })
      .fill("CHG-1: alerts backend");
    await expect(save).toBeEnabled();
    await save.click();

    await expect(
      panel.getByText("alerts-platform takes effect on its next request."),
    ).toBeVisible();
    expect(saved).toEqual([
      {
        subject: "sa-platform",
        scopes: ["identity:read", "notifications:write"],
        enabled: true,
        reason: "CHG-1: alerts backend",
      },
    ]);
    await expect(panel.getByRole("cell", { name: "alerts-platform" })).toBeVisible();

    const overflowing = await main.evaluate(
      (element) => element.scrollWidth > element.clientWidth + 1,
    );
    expect(overflowing).toBe(false);
    expect(errors).toEqual([]);
  });
}
