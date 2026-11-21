import { test, expect } from '@playwright/test';
import { URL } from 'node:url';

const admin = { id: '00000000-0000-0000-0000-000000000001', username: 'admin', email: 'admin@example.invalid', isAdmin: true, enabled: true, realmRoles: ['admin'], clientRoles: {}, permissions: ['portal:access', 'clients:read', 'clients:write'], systems: [], accessibleSystems: [] };

for (const viewport of [{ width: 1280, height: 900 }, { width: 390, height: 844 }]) {
test(`admin reviews and provisions a disabled client at ${viewport.width}px`, async ({ page }) => {
  await page.setViewportSize(viewport);
  await page.emulateMedia({ reducedMotion: "reduce" });
  const writes = [];
  const failures = [];
  page.on('pageerror', error => failures.push(error.message));
  await page.route('http://127.0.0.1:5417/ssobackend/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let data = [];
    if (path.endsWith('/auth/me')) data = { user: admin };
    else if (path.includes('/onboarding/context')) data = { environment: 'staging', realm: 'moh-staging', issuer: 'https://auth.example.invalid/realms/moh-staging', profiles: [{ id: 'public-web', name: 'Public web application (PKCE)' }] };
    else if (path.endsWith('/onboarding/preview')) {
      const { spec } = route.request().postDataJSON();
      data = { spec, operation: 'create', realm: 'moh-staging', issuer: 'https://auth.example.invalid/realms/moh-staging', reviewToken: 'fixture-review', client: { ...spec, enabled: false, publicClient: true, standardFlowEnabled: true, defaultClientScopes: ['profile', 'email'], attributes: { 'pkce.code.challenge.method': 'S256' } } };
    } else if (path.endsWith('/onboarding/apply')) {
      const body = route.request().postDataJSON(); writes.push(body);
      data = { client: { ...body.spec, id: 'fixture', enabled: false }, issuer: 'https://auth.example.invalid/realms/moh-staging', message: 'Provisioned and verified. Client is disabled.' };
    } else if (path.includes('unread')) data = { count: 0 };
    await route.fulfill({ json: { success: true, data } });
  });
  await page.goto('/portal/admin/clients');
  const main = page.locator('#main-content');
  const openOnboarding = page.getByRole('button', { name: 'Onboard application', exact: true });
  await expect(openOnboarding).toBeVisible();
  const initialWidth = (await main.boundingBox()).width;
  await openOnboarding.click();
  const panel = page.getByRole('dialog', { name: 'Onboard application', exact: true });
  await expect(panel).toBeVisible();
  if (viewport.width >= 1280) {
    await expect(panel).not.toHaveAttribute('aria-modal', 'true');
    const content = await main.boundingBox();
    expect(content.width).toBeLessThan(initialWidth);
    expect(content.x + content.width).toBeLessThanOrEqual((await panel.boundingBox()).x);
  } else {
    await expect(panel).toHaveAttribute('aria-modal', 'true');
  }
  await page.getByLabel('Client ID', { exact: true }).fill('workforce-app');
  await page.getByLabel('Application name', { exact: true }).fill('Workforce');
  await page.getByLabel('Application owner or team').fill('Workforce team');
  await page.getByLabel('Support email', { exact: true }).fill('support@example.invalid');
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
  await page.getByLabel('Application URL', { exact: true }).fill('https://workforce.example.invalid');
  await page.getByLabel('Sign-in callback URLs').fill('https://workforce.example.invalid/callback');
  await page.getByLabel('Post-logout URLs').fill('https://workforce.example.invalid/login');
  await page.getByLabel('Allowed browser origins').fill('https://workforce.example.invalid');
  await page.getByRole('button', { name: 'Validate & preview' }).click();
  const apply = page.getByRole('button', { name: 'Provision disabled client' });
  await expect(apply).toBeDisabled();
  await page.getByText('I reviewed these URLs and settings for staging.', { exact: true }).click();
  await apply.click();
  await expect(page.getByRole('heading', { name: 'Application configured' })).toBeVisible();
  expect(writes).toHaveLength(1);
  expect(writes[0].spec.environment).toBe('staging');
  expect(writes[0].reviewToken).toBe('fixture-review');
  expect(writes[0]).not.toHaveProperty('secret');
  expect(failures).toEqual([]);
});

}

for (const scenario of ["missing-api", "wrong-realm"]) {
  test(`onboarding explains ${scenario} and can recover without reloading`, async ({ page }) => {
    let attempts = 0;
    const writes = [];
    await page.route("http://127.0.0.1:5417/ssobackend/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (route.request().method() !== "GET") writes.push(path);
      if (path.endsWith("/onboarding/context")) {
        attempts += 1;
        if (attempts === 1) {
          if (scenario === "missing-api")
            return route.fulfill({
              status: 404,
              contentType: "text/plain",
              body: "404 page not found",
            });
          return route.fulfill({
            status: 503,
            json: {
              success: false,
              error: {
                code: "ENVIRONMENT_MISMATCH",
                message: "identity environment/realm mismatch",
              },
            },
          });
        }
        return route.fulfill({
          json: {
            success: true,
            data: {
              environment: "development",
              realm: "moh-dev",
              issuer: "http://localhost:8081/realms/moh-dev",
              profiles: [{ id: "public-web", name: "Public web application (PKCE)" }],
            },
          },
        });
      }
      return route.fulfill({
        json: {
          success: true,
          data: path.endsWith("/auth/me")
            ? { user: admin }
            : path.includes("unread")
              ? { count: 0 }
              : [],
        },
      });
    });
    await page.goto("/portal/admin/clients");
    await page.getByRole("button", { name: "Onboard application", exact: true }).click();
    await expect(
      page.getByText(
        scenario === "missing-api"
          ? /backend does not provide the onboarding API/
          : /realm for a different environment/,
      ),
    ).toBeVisible();
    await page.getByRole("button", { name: "Retry onboarding" }).click();
    await expect(page.getByLabel("Client ID", { exact: true })).toBeVisible();
    expect(attempts).toBe(2);
    expect(writes).toEqual([]);
  });
}
