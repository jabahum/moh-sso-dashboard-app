// Browser helpers: a Chromium context that resolves *.e2e.test to the stack and
// trusts only Caddy's CA chain (see chromiumTrustArgs), plus portal sign-in.
import { chromium, expect } from "@playwright/test";
import { DASHBOARD, PASSWORD, chromiumTrustArgs } from "./stack.mjs";

let browserPromise;
export async function browser() {
  browserPromise ??= chromiumTrustArgs().then((args) => chromium.launch({ args }));
  return browserPromise;
}

export async function closeBrowser() {
  if (browserPromise) await (await browserPromise).close();
  browserPromise = undefined;
}

/** A fresh, isolated browser context (own cookies and storage). */
export async function newContext() {
  return (await browser()).newContext({ viewport: { width: 1400, height: 900 } });
}

/** Records every URL a page navigates to (main frame), to prove no second login happened. */
export function trackNavigations(page) {
  const urls = [];
  page.on("framenavigated", (frame) => {
    if (frame === page.mainFrame()) urls.push(frame.url());
  });
  return urls;
}

export const isKeycloakLoginPage = (url) =>
  /\/realms\/moh-realm\/(login-actions\/authenticate|protocol\/openid-connect\/auth)/.test(url);

/** Fills Keycloak's login form on the current page. */
export async function keycloakLogin(page, username) {
  await page.waitForSelector("#username", { timeout: 60_000 });
  await page.fill("#username", username);
  await page.fill("#password", PASSWORD);
  await page.click("#kc-login");
}

/** Signs in at the dashboard portal and waits until the portal home has loaded. */
export async function portalSignIn(page, username) {
  await page.goto(`${DASHBOARD}/portal/`);
  // The public portal landing page; its "Sign in" button starts the Keycloak flow.
  await page.getByRole("button", { name: "Sign in", exact: true }).first().click();
  await keycloakLogin(page, username);
  await page.waitForURL((u) => u.origin === DASHBOARD && u.pathname.startsWith("/portal"), { timeout: 60_000 });
  // page.request runs in Node without the browser's host mapping, so ask from the page.
  await expect
    .poll(async () => (await portalFetch(page, "GET", "/api/v1/auth/me")).status, { timeout: 30_000 })
    .toBe(200);
}

/** Calls the dashboard API from the portal page itself (same origin, portal session cookie). */
export async function portalFetch(page, method, path, body) {
  return page.evaluate(
    async ({ method, path, body }) => {
      const res = await fetch(path, {
        method,
        credentials: "include",
        headers: body === undefined ? {} : { "content-type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      const text = await res.text();
      let json;
      try {
        json = JSON.parse(text);
      } catch {
        json = undefined;
      }
      return { status: res.status, json, text };
    },
    { method, path, body },
  );
}
