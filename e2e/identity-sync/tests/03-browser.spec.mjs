// Suite 3 — Browser (Chromium via Playwright 1.64.1). Real portal sign-in,
// real tile launches, real Keycloak SSO and the apps' own OIDC callbacks.
//
// Users owned by this suite: both.apps (linked in both apps), browser.unlinked
// (access roles, unlinked local accounts), browser.noaccess (linked, no roles).
import { test, expect } from "@playwright/test";
import { CLIMS, DASHBOARD, POES } from "../lib/stack.mjs";
import { closeBrowser, isKeycloakLoginPage, keycloakLogin, newContext, portalFetch, portalSignIn, trackNavigations } from "../lib/browser.mjs";

test.afterAll(closeBrowser);

async function openAppMenu(page) {
  const open = page.getByRole("button", { name: "Open application menu" });
  const close = page.getByRole("button", { name: "Close application menu" }).first();
  await expect(open.or(close).first()).toBeVisible({ timeout: 30_000 });
  if (await open.isVisible()) await open.click();
  await expect(page.getByRole("dialog", { name: "Application menu" })).toBeVisible();
}

/** Client IDs the dashboard reports as accessible for the signed-in portal user. */
async function accessibleSystems(page) {
  const me = await portalFetch(page, "GET", "/api/v1/auth/me");
  expect(me.status).toBe(200);
  return [...new Set((JSON.stringify(me.json).match(/"clientId":"[^"]+"/g) ?? []).map((m) => m.slice(12, -1)))];
}

/** The tile for a system in the portal's application menu, found through the menu's own search. */
async function tile(page, title) {
  await openAppMenu(page);
  const dialog = page.getByRole("dialog", { name: "Application menu" });
  await dialog.getByPlaceholder("Search applications").fill(title);
  return dialog.getByText(title, { exact: true });
}

/** Clicks a portal tile (it opens a new tab) and returns the new page with its navigation log. */
async function launchTile(context, page, title) {
  const t = await tile(page, title);
  await expect(t).toBeVisible();
  const [popup] = await Promise.all([context.waitForEvent("page"), t.click()]);
  const navs = trackNavigations(popup);
  await popup.waitForLoadState("domcontentloaded");
  return { popup, navs };
}

test("one portal sign-in opens CLIMS and POES in the right local accounts with their own permissions", async () => {
  const context = await newContext();
  const page = await context.newPage();
  await portalSignIn(page, "both.apps");
  expect(await accessibleSystems(page)).toEqual(expect.arrayContaining(["clims", "poesgo"]));

  // CLIMS tile: Keycloak SSO (no second login), handoff, the account's role home.
  const clims = await launchTile(context, page, "CLIMS");
  await clims.popup.waitForURL(`${CLIMS}/moh`, { timeout: 60_000 });
  await expect(clims.popup.getByRole("heading", { name: "MOH dashboard" })).toBeVisible();
  // The account menu names the local CLIMS account and its existing role.
  await expect(clims.popup.getByRole("button", { name: /Both Apps.*moh.*account menu/s })).toBeVisible();
  const climsSession = await clims.popup.evaluate(() => JSON.parse(localStorage.getItem("clims.session")));
  expect(climsSession.user).toMatchObject({ id: "c1000000-0000-4000-8000-000000000001", username: "both.apps", role: "moh" });
  await expect(clims.popup.locator("#username")).toHaveCount(0);

  // POES tile: same SSO session, POES's landing page, then the account page.
  const poes = await launchTile(context, page, "Poesgo");
  await poes.popup.waitForURL(`${POES}/api/v1/auth/sso/session`, { timeout: 60_000 });
  await expect(poes.popup.getByText("Signed in to Point-of-Entry Screening.")).toBeVisible();
  await poes.popup.getByRole("link", { name: "View my account and assignments" }).click();
  await poes.popup.waitForURL(`${POES}/api/v1/auth/me`);
  const me = JSON.parse(await poes.popup.locator("body").innerText()).data;
  expect(me.user).toMatchObject({ id: 1, username: "both.apps", role_key: "poe_officer", is_active: true });
  expect(me.assignments.map((a) => a.poe_code)).toEqual(["POE-E2E-ALPHA"]);
  await expect(poes.popup.getByText('"poe_code":"POE-E2E-ALPHA"')).toBeVisible();

  // Neither tab ever showed the Keycloak login form: the portal sign-in was the only one.
  for (const navs of [clims.navs, poes.navs]) {
    expect(navs.some((u) => u.includes("/login-actions/authenticate"))).toBe(false);
  }
  await context.close();
});

test("a user with the access role but no link is refused by both apps", async () => {
  const context = await newContext();
  const page = await context.newPage();
  await portalSignIn(page, "browser.unlinked");

  const clims = await launchTile(context, page, "CLIMS");
  await clims.popup.waitForURL((u) => u.pathname === "/sign-in", { timeout: 60_000 });
  expect(new URL(clims.popup.url()).searchParams.get("sso_error")).toBe("unavailable");
  await expect(clims.popup.getByText("Your portal account is not linked to an active CLIMS account.")).toBeVisible();
  expect(await clims.popup.evaluate(() => localStorage.getItem("clims.session"))).toBeNull();

  const poes = await launchTile(context, page, "Poesgo");
  await expect(poes.popup.getByText("sso_account_unavailable")).toBeVisible({ timeout: 60_000 });
  const body = JSON.parse(await poes.popup.locator("body").innerText());
  expect(JSON.stringify(body)).toContain("An active linked Poesgo account is required");
  // No POES session was created.
  const check = await poes.popup.goto(`${POES}/api/v1/auth/me`);
  expect(check.status()).toBe(401);
  await context.close();
});

test("a linked user without the access role gets no tiles and is refused by both apps", async () => {
  const context = await newContext();
  const page = await context.newPage();
  await portalSignIn(page, "browser.noaccess");
  const systems = await accessibleSystems(page);
  expect(systems).toContain("dashboard-web");
  expect(systems).not.toContain("clims");
  expect(systems).not.toContain("poesgo");
  // The menu works for this user (the portal's own tile is there) but offers neither app.
  await expect(await tile(page, "Dashboards")).toBeVisible();
  await expect(await tile(page, "CLIMS")).toHaveCount(0);
  await expect(await tile(page, "Poesgo")).toHaveCount(0);

  // Going to the apps directly (bookmark): Keycloak SSO succeeds, the apps refuse.
  const clims = await context.newPage();
  await clims.goto(`${CLIMS}/api/v1/auth/sso/start`);
  await clims.waitForURL((u) => u.pathname === "/sign-in", { timeout: 60_000 });
  expect(new URL(clims.url()).searchParams.get("sso_error")).toBe("denied");
  await expect(clims.getByText("Your portal account does not have access to CLIMS.")).toBeVisible();
  expect(await clims.evaluate(() => localStorage.getItem("clims.session"))).toBeNull();

  const poes = await context.newPage();
  await poes.goto(`${POES}/api/v1/auth/sso/start`);
  await poes.waitForURL((u) => u.pathname === "/api/v1/auth/sso/callback", { timeout: 60_000 });
  await expect(poes.getByText("sso_access_denied")).toBeVisible();
  expect((await poes.goto(`${POES}/api/v1/auth/me`)).status()).toBe(401);
  expect(page.url().startsWith(DASHBOARD)).toBe(true);
  await context.close();
});

for (const [label, clUser, poUser] of [
  ["a disabled local account", "clims.disabled", "poes.disabled"],
  ["an orphan (access role, no local account)", "clims.orphan", "poes.orphan"],
]) {
  test(`${label} is refused by the app without a session`, async () => {
    for (const [username, app] of [[clUser, "clims"], [poUser, "poes"]]) {
      const context = await newContext();
      const page = await context.newPage();
      if (app === "clims") {
        await page.goto(`${CLIMS}/api/v1/auth/sso/start`);
        await keycloakLogin(page, username);
        await page.waitForURL((u) => u.origin === CLIMS && u.pathname === "/sign-in", { timeout: 60_000 });
        expect(new URL(page.url()).searchParams.get("sso_error"), `${username} CLIMS sso_error`).toBe("unavailable");
        expect(await page.evaluate(() => localStorage.getItem("clims.session"))).toBeNull();
      } else {
        await page.goto(`${POES}/api/v1/auth/sso/start`);
        await keycloakLogin(page, username);
        await page.waitForURL((u) => u.origin === POES && u.pathname === "/api/v1/auth/sso/callback", { timeout: 60_000 });
        await expect(page.getByText("sso_account_unavailable")).toBeVisible();
        expect((await page.goto(`${POES}/api/v1/auth/me`)).status()).toBe(401);
      }
      await context.close();
    }
  });
}

test("a user disabled in Keycloak cannot sign in at all", async () => {
  const context = await newContext();
  const page = await context.newPage();
  await page.goto(`${CLIMS}/api/v1/auth/sso/start`);
  await keycloakLogin(page, "clims.kcdisabled");
  await expect(page.locator("#input-error, .kc-feedback-text, [class*='alert-error']").first()).toBeVisible({ timeout: 30_000 });
  expect(isKeycloakLoginPage(page.url())).toBe(true);
  expect(page.url().startsWith(CLIMS)).toBe(false);
  await context.close();
});
