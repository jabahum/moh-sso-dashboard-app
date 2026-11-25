// Suite 5 — Portal screens. A portal admin uses the RBAC workspace's
// Identity sync section against the real backend, CLIMS and Keycloak: checks
// the integration, starts a run, approves and applies one row, and clears a
// revocation.
//
// User owned by this suite: screens.unlinked (Keycloak n=110, CLIMS account
// c1...15), unlinked with a verified email, so a run proposes reuse.
import { test, expect } from "@playwright/test";
import { APPS, DASHBOARD, ISSUER, cli, climsId, dashboardDB, directClientRoles, kcId, serviceAccountUser, sql } from "../lib/stack.mjs";
import { closeBrowser, newContext, portalSignIn } from "../lib/browser.mjs";

const SUBJECT = kcId(110);
const ACCOUNT = climsId(0x15);
const ADMIN_ACTOR = "portal:e2e.admin@e2e.test";

test.afterAll(closeBrowser);

/**
 * Opens the RBAC workspace's Identity sync section for an application: the
 * "This application" scope, the application selector, then the section tab.
 */
async function openIdentitySync(page, clientId = "clims") {
  await page.goto(`${DASHBOARD}/admin/rbac`);
  const scope = page.getByRole("button", { name: "This application", exact: true });
  await expect(scope).toBeVisible({ timeout: 60_000 });
  if ((await scope.getAttribute("aria-pressed")) !== "true") await scope.click();
  const app = page.getByRole("combobox", { name: "Selected application" });
  await app.fill(clientId);
  await page.getByRole("option", { name: new RegExp(`\\(${clientId}\\)$`) }).click();
  await expect(app).toHaveValue(new RegExp(`\\(${clientId}\\)$`));
  await page
    .getByRole("tablist", { name: "Application sections" })
    .getByRole("tab", { name: "Identity sync", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "Integration" })).toBeVisible();
}

function rowFor(page, accountId) {
  return page.getByRole("table", { name: "Run rows" }).getByRole("row").filter({ hasText: accountId });
}

test("an admin reviews and applies a run, and clears a revocation, from the RBAC workspace", async () => {
  const context = await newContext();
  const page = await context.newPage();
  page.on("dialog", (dialog) => dialog.accept());
  await portalSignIn(page, "e2e.admin");
  await openIdentitySync(page);

  // Integration: registered, enabled, contract 1.1, and the live descriptor answers.
  const integration = page.getByRole("region", { name: "Identity sync integration" });
  await expect(integration.getByText("Enabled", { exact: true }).first()).toBeVisible();
  await expect(integration.getByText("Contract 1.1")).toBeVisible();
  await expect(integration.getByLabel("Application origin")).toHaveValue(APPS.clims.origin);
  await integration.getByRole("button", { name: "Check connection" }).click();
  await expect(integration.getByText(/Live descriptor: contract 1\.1, application clims/)).toBeVisible();
  await expect(integration.getByText(/sessions\.revoke/)).toBeVisible();

  // Start a run: it runs in the background and opens when it finishes.
  const runs = page.getByRole("region", { name: "Identity sync runs" });
  await runs.getByRole("button", { name: "Start run" }).click();
  await expect(page.getByRole("heading", { name: /Run started .* by portal:e2e\.admin@e2e\.test/ })).toBeVisible({
    timeout: 60_000,
  });
  const row = rowFor(page, ACCOUNT);
  await expect(row).toContainText("Reuse existing user");
  await expect(row).toContainText(SUBJECT);
  // A review row cannot be selected.
  await expect(rowFor(page, climsId(0x12)).getByRole("checkbox")).toBeDisabled();

  // Approve only this row; the reviewer is the signed-in admin.
  await row.getByRole("checkbox").check({ force: true });
  await page.getByLabel("Review note (optional)").fill("e2e: approved in the portal");
  await page.getByRole("button", { name: "Approve selected (1)" }).click();
  await expect(page.getByText("1 row approved.")).toBeVisible();
  await expect(row).toContainText(`approved by ${ADMIN_ACTOR}`);
  expect(await directClientRoles(SUBJECT, "clims")).not.toContain("clims_access");

  // Apply in the background; the row updates when it finishes.
  await page.getByRole("button", { name: "Apply approved rows" }).click();
  await expect(row).toContainText("linked / granted", { timeout: 60_000 });
  await expect(page.getByRole("button", { name: "Apply approved rows" })).toBeDisabled();
  expect(await directClientRoles(SUBJECT, "clims")).toContain("clims_access");
  expect(
    sql(APPS.clims.db, `SELECT issuer, subject FROM user_oidc_identities WHERE user_id = '${ACCOUNT}'`),
  ).toEqual([[ISSUER, SUBJECT]]);
  // Only the approved row was applied.
  expect(sql(APPS.clims.db, `SELECT count(*) FROM user_oidc_identities WHERE user_id = '${climsId(0x13)}'`)).toEqual([["0"]]);

  // The stored run records the admin as creator and reviewer.
  const runId = cli(["identity-sync", "run", "show", "--app", "clims"]).json.runs[0].id;
  const shown = cli(["identity-sync", "run", "show", "--run", runId]).json;
  expect(shown.run).toMatchObject({ createdBy: ADMIN_ACTOR, status: "completed" });
  expect(shown.rows.find((r) => r.accountId === ACCOUNT)).toMatchObject({
    reviewedBy: ADMIN_ACTOR,
    reviewNote: "e2e: approved in the portal",
    linkStatus: "linked",
    grantStatus: "granted",
  });

  // Revoke with the CLI, then clear the revocation from the portal.
  cli(["identity-sync", "revoke", "--app", "clims", "--subject", SUBJECT, "--revoker", "e2e-operator", "--reason", "e2e: screens"]);
  await page.reload();
  await openIdentitySync(page);
  const removals = page.getByRole("region", { name: "Identity sync revocations" });
  const revocation = removals.getByRole("row").filter({ hasText: SUBJECT });
  await expect(revocation).toContainText("Role done");
  await expect(revocation).toContainText("Sessions done");
  await expect(revocation).toContainText("Done");
  await expect(revocation.getByRole("button", { name: "Clear revocation" })).toBeDisabled();
  await removals.getByLabel("Reason for clearing a revocation").fill("e2e: cleared in the portal");
  await revocation.getByRole("button", { name: "Clear revocation" }).click();
  await expect(removals.getByText("Revocation cleared. Access was not granted again.")).toBeVisible();
  await removals.getByText("Active only").click();
  await expect(removals.getByRole("row").filter({ hasText: SUBJECT })).toContainText(`Restored by ${ADMIN_ACTOR}`);
  const listed = cli(["identity-sync", "revocations", "--app", "clims", "--all"]).json;
  expect(listed.find((r) => r.subject === SUBJECT)).toMatchObject({
    restoredBy: ADMIN_ACTOR,
    restoreReason: "e2e: cleared in the portal",
  });
  // Clearing never grants access again.
  expect(await directClientRoles(SUBJECT, "clims")).not.toContain("clims_access");
  await context.close();
});

test("an admin re-enables the sync client's service identity, with the subject read from Keycloak", async () => {
  const binding = () =>
    sql(dashboardDB, `SELECT subject, enabled FROM ihp_application_identities WHERE oauth_client_id = 'poesgo-identity-sync'`);
  const serviceAccount = await serviceAccountUser("poesgo-identity-sync");
  expect(binding()).toEqual([[serviceAccount.id, "t"]]);
  // An operator disabled the binding (for example after a suspected leak).
  sql(dashboardDB, `UPDATE ihp_application_identities SET enabled = false WHERE oauth_client_id = 'poesgo-identity-sync'`);

  const context = await newContext();
  const page = await context.newPage();
  await portalSignIn(page, "e2e.admin");
  await openIdentitySync(page, "poesgo");
  const integration = page.getByRole("region", { name: "Identity sync integration" });
  const status = integration.locator(".rbac-identity-sync__binding");
  await expect(status).toContainText("poesgo-identity-sync");
  await expect(status).toContainText("Disabled");
  const enable = integration.getByRole("button", { name: "Enable service identity" });
  // A reason is required before anything is written.
  await expect(enable).toBeDisabled();
  await integration.getByLabel("Reason for this change").fill("e2e: re-enable the sync client");
  await enable.click();
  await expect(integration.getByText(`Keycloak service account ${serviceAccount.id}`)).toBeVisible();
  await expect(status).toContainText("Registered");
  await expect(enable).toHaveCount(0);

  expect(binding()).toEqual([[serviceAccount.id, "t"]]);
  const audit = sql(
    dashboardDB,
    `SELECT details->>'reason', details->>'subject' FROM ihp_rbac_audit_events
     WHERE action = 'application.identity.updated' AND resource_id = 'poesgo-identity-sync' ORDER BY created_at DESC LIMIT 1`,
  );
  expect(audit).toEqual([["e2e: re-enable the sync client", serviceAccount.id]]);
  // The integration works again: the live descriptor answers.
  await integration.getByRole("button", { name: "Check connection" }).click();
  await expect(integration.getByText(/Live descriptor: contract 1\.1, application poesgo/)).toBeVisible();
  await context.close();
});
