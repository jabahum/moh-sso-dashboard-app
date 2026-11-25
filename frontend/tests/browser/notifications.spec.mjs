import { test, expect } from "@playwright/test";
import { URL } from "node:url";

const account = {
  id: "00000000-0000-0000-0000-000000000001",
  username: "notification-reviewer",
  firstName: "Portal",
  lastName: "Reviewer",
  isAdmin: true,
  isUser: true,
  realmRoles: ["admin"],
  clientRoles: {},
  systems: [],
  accessibleSystems: [],
  permissions: ["portal:access", "users:read", "notifications:read", "notifications:write"],
};
const alerts = [
  {
    id: "notice-1",
    title: "Token refresh failed",
    message: "A token refresh attempt failed. Review the session before trying again.",
    severity: "warning",
    read: false,
    created_at: "2026-11-05T07:21:41Z",
    type: "AUTH_TOKEN_REFRESH_FAILED",
    target_role: "admin",
    metadata: { action: "auth.refresh_failed", detail: "long-event-value-".repeat(35) },
  },
  {
    id: "notice-2",
    title: "System started",
    message: "The portal service started successfully.",
    severity: "success",
    read: false,
    created_at: "2026-11-05T06:55:28Z",
  },
  {
    id: "notice-3",
    title: "Earlier update",
    message: "An earlier update that has already been read.",
    severity: "info",
    read: true,
    created_at: "2026-11-04T06:55:28Z",
  },
];

async function fixture(page, options = {}) {
  const state = {
    items: globalThis.structuredClone(alerts),
    failRead: false,
    failList: false,
    retries: 0,
    deleted: 0,
    ...options,
  };
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method();
    const fulfill = (data) => route.fulfill({ json: { success: true, data } });
    if (path.endsWith("/auth/me"))
      return fulfill({
        user: {
          ...account,
          permissions: account.permissions.filter((permission) =>
            options.readOnly ? permission !== "notifications:write" : true,
          ),
        },
      });
    if (path.endsWith("/notifications/count/unread"))
      return fulfill({ count: state.items.filter((item) => !item.read).length });
    if (path.endsWith("/notifications/deliveries/mail/retry")) {
      if (state.failRetry) return route.fulfill({ status: 500, json: { success: false } });
      state.retries++;
      return fulfill(null);
    }
    if (path.endsWith("/notice-1/deliveries"))
      return fulfill([
        {
          id: "app",
          channel: "in_app",
          status: "PENDING",
          attempts: 0,
          max_attempts: 1,
          updated_at: alerts[0].created_at,
        },
        {
          id: "mail",
          channel: "email",
          status: state.retries ? "RETRY" : "FAILED",
          attempts: 1,
          max_attempts: 5,
          updated_at: alerts[0].created_at,
          last_error: "Email delivery was rejected. Please try again.",
        },
      ]);
    if (path.endsWith("/deliveries")) return fulfill([]);
    if (path.endsWith("/notifications")) {
      if (state.failList) return route.fulfill({ status: 503, json: { success: false } });
      const items =
        url.searchParams.get("unread") === "true"
          ? state.items.filter((item) => !item.read)
          : state.items;
      return fulfill(items.slice(0, Number(url.searchParams.get("limit") ?? 20)));
    }
    const notificationId = path.match(/\/notifications\/([^/]+)/)?.[1];
    const item = state.items.find((item) => item.id === notificationId);
    if (item) {
      if (method === "PATCH") {
        if (state.failRead) return route.fulfill({ status: 500, json: { success: false } });
        item.read = true;
        return fulfill(null);
      }
      if (method === "DELETE") {
        state.items = state.items.filter((entry) => entry.id !== item.id);
        state.deleted++;
        return fulfill(null);
      }
      return fulfill(item);
    }
    return fulfill([]);
  });
  return { state, errors };
}

async function openFeed(page) {
  await page.goto("/admin/home");
  await page.getByRole("button", { name: /^Notifications(,|$)/ }).click();
  return page.getByRole("dialog", { name: "Notifications", exact: true });
}

for (const width of [390, 1536]) {
  test(`notifications resize, open details and preserve recent activity at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { state, errors } = await fixture(page);
    const panel = await openFeed(page);
    await expect(panel.getByRole("heading", { name: "Notifications", exact: true })).toHaveCount(1);
    await expect(panel.getByRole("article")).toHaveCount(3);
    if (width > 672)
      await panel.getByRole("button", { name: "Maximize panel", exact: true }).click();
    const item = panel.getByRole("button", { name: "View Token refresh failed", exact: true });
    await item.focus();
    await page.keyboard.press("Enter");
    const back = panel.getByRole("button", { name: "Back to notifications", exact: true });
    await expect(back).toBeFocused();
    await expect(
      panel.getByRole("heading", { name: "Token refresh failed", exact: true }),
    ).toBeVisible();
    await expect(panel.getByText("AUTH_TOKEN_REFRESH_FAILED", { exact: true })).toBeHidden();
    await panel.getByText("Technical details", { exact: true }).click();
    await expect(panel.getByText("AUTH_TOKEN_REFRESH_FAILED", { exact: true })).toBeVisible();
    expect(await panel.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(
      true,
    );
    await panel.getByRole("button", { name: "Mark as read", exact: true }).click();
    await expect(
      panel.getByRole("region", { name: "Notification summary", exact: true }),
    ).toContainText("Read");
    await expect(panel.getByRole("button", { name: "Mark as read", exact: true })).toHaveCount(0);
    await panel.getByRole("button", { name: "Retry Email delivery", exact: true }).click();
    await expect.poll(() => state.retries).toBe(1);
    await back.click();
    await expect(item).toBeFocused();
    await expect(panel.getByRole("article")).toHaveCount(3);
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: /^Notifications(,|$)/ }).click();
    await expect(panel.getByRole("article")).toHaveCount(3);
    await expect(
      panel.getByRole("button", { name: "Back to notifications", exact: true }),
    ).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test("mark-read failure is visible and a successful retry refreshes the feed", async ({ page }) => {
  const { state } = await fixture(page, { failRead: true });
  const panel = await openFeed(page);
  const mark = panel.getByRole("button", {
    name: "Mark Token refresh failed as read",
    exact: true,
  });
  await mark.click();
  await expect(
    panel.getByText("Could not mark the notification as read. Please try again."),
  ).toBeVisible();
  await expect(panel.getByRole("article")).toHaveCount(3);
  state.failRead = false;
  await mark.click();
  await expect(mark).toHaveCount(0);
  await expect(panel.getByRole("article")).toHaveCount(3);
  await expect(
    page.getByRole("button", { name: "Notifications, 1 unread", exact: true }),
  ).toBeVisible();
});

test("delivery retry failures remain visible beside the channel", async ({ page }) => {
  const { state } = await fixture(page, { failRetry: true });
  const panel = await openFeed(page);
  await panel.getByRole("button", { name: "View Token refresh failed", exact: true }).click();
  const delivery = panel.getByRole("article", { name: "Email delivery", exact: true });
  const retry = delivery.getByRole("button", { name: "Retry Email delivery", exact: true });
  await retry.click();
  await expect(delivery.getByText("Could not retry the delivery. Please try again.")).toBeVisible();
  state.failRetry = false;
  await retry.click();
  await expect.poll(() => state.retries).toBe(1);
  await expect(delivery.getByText("Could not retry the delivery. Please try again.")).toHaveCount(
    0,
  );
});

test("deletion requires confirmation and does not open the detail view", async ({ page }) => {
  const { state } = await fixture(page);
  const panel = await openFeed(page);
  await panel.getByRole("button", { name: "Delete System started", exact: true }).click();
  await expect(panel.getByText("Delete this notification?", { exact: true })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Keep notification", exact: true })).toBeFocused();
  expect(state.deleted).toBe(0);
  await panel.getByRole("button", { name: "Keep notification", exact: true }).click();
  await expect(
    panel.getByRole("button", { name: "Delete System started", exact: true }),
  ).toBeFocused();
  await panel.getByRole("button", { name: "Delete System started", exact: true }).click();
  await panel.getByRole("button", { name: "Confirm delete", exact: true }).click();
  await expect(panel.getByRole("article")).toHaveCount(2);
  expect(state.deleted).toBe(1);
  await expect(
    panel.getByRole("button", { name: "Back to notifications", exact: true }),
  ).toHaveCount(0);
});

test("notification loading errors can be retried and empty views stay navigable", async ({
  page,
}) => {
  const { state } = await fixture(page, { failList: true });
  const panel = await openFeed(page);
  await expect(panel.getByText("Unable to load notifications", { exact: true })).toBeVisible();
  state.failList = false;
  state.items = [];
  await panel.getByRole("button", { name: "Try again", exact: true }).click();
  await expect(panel.getByText("No recent notifications", { exact: true })).toBeVisible();
});

test("overview table opens details and delivery retries respect permissions", async ({ page }) => {
  await fixture(page, { readOnly: true });
  await page.goto("/admin/home");
  await page.getByRole("button", { name: "View Token refresh failed", exact: true }).click();
  const panel = page.getByRole("dialog", { name: "Notifications", exact: true });
  await expect(
    panel.getByRole("heading", { name: "Token refresh failed", exact: true }),
  ).toBeVisible();
  await expect(panel.getByRole("article", { name: "Email delivery", exact: true })).toBeVisible();
  await expect(
    panel.getByRole("button", { name: "Retry Email delivery", exact: true }),
  ).toHaveCount(0);
});

async function chooseFilter(page, name, value) {
  await page.getByRole("combobox", { name: new RegExp(name) }).click();
  await page.getByRole("option", { name: value, exact: true }).click();
}

test("overview notifications table combines search, status and severity filters", async ({
  page,
}) => {
  await fixture(page);
  await page.goto("/admin/home");
  const table = page.getByRole("table", { name: "Notifications", exact: true });
  await expect(table.getByRole("row")).toHaveCount(3);
  await chooseFilter(page, "Status", "All notifications");
  await expect(table.getByRole("row")).toHaveCount(4);
  await chooseFilter(page, "Severity", "Warning");
  await expect(table.getByRole("row")).toHaveCount(2);
  await page
    .getByRole("searchbox", { name: "Search notifications", exact: true })
    .fill("no matching alert");
  await expect(
    page.getByText("No notifications match your filters.", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Reset filters", exact: true }).first().click();
  await expect(table.getByRole("row")).toHaveCount(3);
  await chooseFilter(page, "Status", "Read");
  await expect(
    table.getByRole("button", { name: "View Earlier update", exact: true }),
  ).toBeVisible();
  await expect(table.getByRole("row")).toHaveCount(2);
});

test("overview table row actions open details and refresh unread rows", async ({ page }) => {
  await fixture(page);
  await page.goto("/admin/home");
  const table = page.getByRole("table", { name: "Notifications", exact: true });
  await table
    .getByRole("button", { name: "Actions for Token refresh failed", exact: true })
    .click();
  await page.getByRole("menuitem", { name: "View details", exact: true }).click();
  const panel = page.getByRole("dialog", { name: "Notifications", exact: true });
  await expect(
    panel.getByRole("heading", { name: "Token refresh failed", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await table.getByRole("button", { name: "Actions for System started", exact: true }).click();
  await page.getByRole("menuitem", { name: "Mark as read", exact: true }).click();
  await expect(table.getByRole("row")).toHaveCount(2);
  await expect(
    page.getByRole("button", { name: "Notifications, 1 unread", exact: true }),
  ).toBeVisible();
});

for (const width of [390, 1536]) {
  test(`overview table paginates and fits its container at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const { errors } = await fixture(page, {
      items: Array.from({ length: 18 }, (_, index) => ({
        ...alerts[0],
        id: `paged-${index}`,
        title: `Notice ${index + 1}`,
        severity: index % 2 ? "warning" : "info",
      })),
    });
    await page.goto("/admin/home");
    const table = page.getByRole("table", { name: "Notifications", exact: true });
    await expect(table.getByRole("row")).toHaveCount(6);
    await page.getByRole("button", { name: "Next page", exact: true }).click();
    await expect(table.getByRole("button", { name: "View Notice 6", exact: true })).toBeVisible();
    await page
      .getByRole("searchbox", { name: "Search notifications", exact: true })
      .fill("Notice 18");
    await expect(table.getByRole("row")).toHaveCount(2);
    await expect(table.getByRole("button", { name: "View Notice 18", exact: true })).toBeVisible();
    expect(
      await page.evaluate(
        () => globalThis.document.documentElement.scrollWidth <= globalThis.innerWidth,
      ),
    ).toBe(true);
    await table.getByRole("button", { name: "View Notice 18", exact: true }).click();
    await expect(
      page
        .getByRole("dialog", { name: "Notifications", exact: true })
        .getByRole("heading", { name: "Notice 18", exact: true }),
    ).toBeVisible();
    expect(errors).toEqual([]);
  });
}

for (const width of [390, 1536]) {
  test(`drawer shows ten compact recent updates and uses the panel height at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1000 });
    await fixture(page, {
      items: Array.from({ length: 25 }, (_, index) => ({
        ...alerts[0],
        id: `recent-${index}`,
        title: `Recent alert ${index + 1}`,
      })),
    });
    const panel = await openFeed(page);
    await expect(panel.getByRole("article")).toHaveCount(10);
    await expect(panel.getByRole("tab")).toHaveCount(0);
    await expect(panel.getByText("Recent", { exact: true })).toHaveCount(0);
    await expect(panel.locator(".cds--content-switcher")).toHaveCount(0);
    await expect(
      panel.getByRole("button", { name: "View Recent alert 11", exact: true }),
    ).toHaveCount(0);
    await expect(panel.getByText("25 unread", { exact: true })).toBeVisible();
    const list = panel.getByRole("list", { name: "Recent notifications", exact: true });
    const bodyBox = await panel.locator(".app-side-panel__body").boundingBox();
    const feedBox = await panel.locator(".notifications-panel").boundingBox();
    expect(feedBox.height).toBeGreaterThan(bodyBox.height - 50);
    expect(await list.evaluate((element) => element.scrollHeight > element.clientHeight)).toBe(
      true,
    );
    expect((await panel.getByRole("article").first().boundingBox()).height).toBeLessThan(200);
    await panel
      .getByRole("button", { name: "View Recent alert 10", exact: true })
      .scrollIntoViewIfNeeded();
    await panel.getByRole("button", { name: "View Recent alert 10", exact: true }).click();
    await expect(
      panel.getByRole("heading", { name: "Recent alert 10", exact: true }),
    ).toBeVisible();
  });
}

test("recent feed still shows read notifications when none are unread", async ({ page }) => {
  await fixture(page, { items: [globalThis.structuredClone(alerts[2])] });
  const panel = await openFeed(page);
  await expect(panel.getByRole("article")).toHaveCount(1);
  await expect(panel.getByRole("button", { name: "View Earlier update", exact: true })).toBeVisible();
  await expect(panel.getByText("No unread notifications", { exact: true })).toHaveCount(0);
});
