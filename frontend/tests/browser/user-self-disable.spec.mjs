import { test, expect } from "@playwright/test";
import { URL } from "node:url";

test("users cannot disable themselves individually, in bulk, or through profile edits", async ({
  page,
}) => {
  const actor = {
    id: "00000000-0000-0000-0000-000000000001",
    username: "current-admin",
    email: "admin@example.invalid",
    enabled: true,
    isAdmin: true,
    isUser: true,
    realmRoles: ["admin"],
    clientRoles: {},
    permissions: ["portal:access", "users:read", "users:write"],
    accessibleSystems: [],
  };
  const other = {
    ...actor,
    id: "00000000-0000-0000-0000-000000000002",
    username: "other-user",
    email: "other@example.invalid",
  };
  const mutations = [];
  await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "PATCH") {
      mutations.push({ path, body: request.postDataJSON() });
      other.enabled = false;
      return route.fulfill({ status: 204 });
    }
    const data = path.endsWith("/auth/me")
      ? { user: actor }
      : path.endsWith("/users")
        ? [actor, other]
        : path.includes("unread")
          ? { count: 0 }
          : [];
    return route.fulfill({ json: { success: true, data } });
  });
  await page.goto("/admin/users");
  const ownRow = page
    .getByRole("row")
    .filter({ has: page.getByText("current-admin", { exact: true }) });
  const otherRow = page
    .getByRole("row")
    .filter({ has: page.getByText("other-user", { exact: true }) });
  await ownRow.locator("button.cds--overflow-menu").click();
  await expect(
    page.getByRole("menuitem", { name: "Cannot disable your own account" }),
  ).toBeDisabled();
  await page.getByRole("menuitem", { name: "Edit user", exact: true }).click();
  await expect(page.getByRole("checkbox", { name: "User enabled", exact: true })).toBeDisabled();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "Edit user", exact: true })).toHaveCount(0);
  await ownRow.locator("label.cds--checkbox-label").click();
  await expect(ownRow.getByRole("checkbox")).toBeChecked();
  await otherRow.locator("label.cds--checkbox-label").click();
  await expect(otherRow.getByRole("checkbox")).toBeChecked();
  await expect(page.getByRole("button", { name: "Disable (2)", exact: true })).toBeDisabled();
  await expect(
    page.getByText(
      "Deselect your account to disable other users. You cannot disable your own account.",
    ),
  ).toBeVisible();
  await ownRow.locator("label.cds--checkbox-label").click();
  await expect(ownRow.getByRole("checkbox")).not.toBeChecked();
  await page.getByRole("button", { name: "Disable (1)", exact: true }).click();
  await expect
    .poll(() => mutations)
    .toEqual([
      { path: `/ssobackend/api/v1/admin/users/${other.id}/toggle`, body: { enabled: false } },
    ]);
  await expect(otherRow.getByText("Disabled", { exact: true })).toBeVisible();
});
