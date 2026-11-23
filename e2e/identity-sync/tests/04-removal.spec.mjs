// Suite 4 — Removal. A portal admin removes CLIMS access from a user who has
// live sessions in CLIMS and POES, through the portal's RBAC user-access API.
//
// User owned by this suite: removal.both (Keycloak n=11, CLIMS account
// c1...0f, POES account 15), linked and with access in both apps.
import { test, expect } from "@playwright/test";
import {
  APPS, CLIMS, ISSUER, POES, SYNC_BASE, cli, climsId, dashboardDB, directClientRoles, kc, kcId,
  request, sql, syncToken,
} from "../lib/stack.mjs";
import { closeBrowser, newContext, portalFetch, portalSignIn, trackNavigations } from "../lib/browser.mjs";

const SUBJECT = kcId(11);
const CLIMS_ACCOUNT = climsId(15);
const WINDOW_MS = 5000;

test.afterAll(closeBrowser);

/** Creates a real local session through the test-only probe and returns its token. */
async function probeSession(app) {
  const res = await request("POST", `${APPS[app].origin}${SYNC_BASE}/test/sessions`, {
    headers: { authorization: `Bearer ${await syncToken(app)}` },
    json: { issuer: ISSUER, subject: SUBJECT },
  });
  expect(res.status, `${app} probe session: ${res.text}`).toBe(201);
  return res.json().sessionToken;
}

async function probeCheck(app, sessionToken, token) {
  const res = await request("GET", `${APPS[app].origin}${SYNC_BASE}/test/sessions/check`, {
    headers: { authorization: `Bearer ${token}`, "x-test-session": sessionToken },
  });
  return res.status;
}

/** The CLIMS API, called from the CLIMS tab with the SPA's own stored access token. */
async function climsMe(page) {
  return page.evaluate(async () => {
    const { token } = JSON.parse(localStorage.getItem("clims.session"));
    const res = await fetch("/api/v1/auth/me", { headers: { authorization: `Bearer ${token}` } });
    return res.status;
  });
}

/** POES's account API from the POES tab (session cookie). */
async function poesMe(page) {
  return page.evaluate(async () => (await fetch("/api/v1/auth/me", { credentials: "include" })).status);
}

async function openTile(context, portal, title) {
  const open = portal.getByRole("button", { name: "Open application menu" });
  const dialog = portal.getByRole("dialog", { name: "Application menu" });
  if (!(await dialog.isVisible())) await open.click();
  await dialog.getByPlaceholder("Search applications").fill(title);
  const [tab] = await Promise.all([context.waitForEvent("page"), dialog.getByText(title, { exact: true }).click()]);
  return tab;
}

/** The access a correct portal form would submit: Keycloak's direct mappings for every assignable system. */
async function currentDirectAccess(assignable) {
  const realm = (await kc("GET", `/users/${SUBJECT}/role-mappings/realm`)).map((r) => r.name);
  const assignableRealm = new Set(assignable.realmRoles.map((r) => r.name));
  const clientRoles = {};
  for (const system of assignable.systems) {
    const roles = await directClientRoles(SUBJECT, system.clientId).catch(() => []);
    if (roles.length) clientRoles[system.clientId] = roles;
  }
  return { realmRoles: realm.filter((r) => assignableRealm.has(r)), clientRoles };
}

test("the RBAC user-access profile shows the user's current Keycloak access", async () => {
  // Precondition for an admin removing access through the portal UI: the form is
  // filled from this profile. See product_bugs: KeyAdminClient.GetUser reads
  // GET /users/{id}, which carries no role mappings, so this comes back empty.
  const context = await newContext();
  const admin = await context.newPage();
  await portalSignIn(admin, "e2e.admin");
  const profile = await portalFetch(admin, "GET", `/api/v1/admin/rbac/user-access/users/${SUBJECT}`);
  expect(profile.status).toBe(200);
  const direct = profile.json.data.directAccess;
  expect(await directClientRoles(SUBJECT, "clims")).toEqual(["clims_access"]);
  expect(direct.clientRoles.clims ?? [], "directAccess.clientRoles.clims").toEqual(["clims_access"]);
  expect(direct.clientRoles.poesgo ?? [], "directAccess.clientRoles.poesgo").toEqual(["poesgo_access"]);
  expect(direct.realmRoles, "directAccess.realmRoles").toContain("user");
  await context.close();
});

test("removing CLIMS access in the portal ends the CLIMS session at once and leaves POES working", async ({}, testInfo) => {
  // ---- Live sessions for removal.both in both apps.
  const userContext = await newContext();
  const portal = await userContext.newPage();
  await portalSignIn(portal, "removal.both");
  const climsTab = await openTile(userContext, portal, "CLIMS");
  await climsTab.waitForURL((u) => u.origin === CLIMS && !u.pathname.startsWith("/sso") && u.pathname !== "/sign-in", { timeout: 60_000 });
  expect(await climsTab.evaluate(() => JSON.parse(localStorage.getItem("clims.session")).user.username)).toBe("removal.both");
  expect(await climsMe(climsTab)).toBe(200);
  const poesTab = await openTile(userContext, portal, "Poesgo");
  await poesTab.waitForURL(`${POES}/api/v1/auth/sso/session`, { timeout: 60_000 });
  expect(await poesMe(poesTab)).toBe(200);
  const climsProbe = await probeSession("clims");
  const poesProbe = await probeSession("poesgo");
  const climsToken = await syncToken("clims");
  const poesToken = await syncToken("poesgo");
  expect(await probeCheck("clims", climsProbe, climsToken)).toBe(200);
  expect(await probeCheck("poesgo", poesProbe, poesToken)).toBe(200);
  expect(sql(dashboardDB, `SELECT count(*) FROM ihp_identity_sync_revocations WHERE subject = '${SUBJECT}'`)).toEqual([["0"]]);

  // ---- A portal admin removes clims_access through the RBAC user-access API.
  const adminContext = await newContext();
  const admin = await adminContext.newPage();
  await portalSignIn(admin, "e2e.admin");
  const assignable = (await portalFetch(admin, "GET", "/api/v1/admin/rbac/user-access/assignable")).json.data;
  const before = await currentDirectAccess(assignable);
  expect(before.clientRoles.clims).toEqual(["clims_access"]);
  expect(before.clientRoles.poesgo).toEqual(["poesgo_access"]);
  const body = { ...before, clientRoles: { ...before.clientRoles } };
  delete body.clientRoles.clims;

  const t0 = Date.now();
  const put = await portalFetch(admin, "PUT", `/api/v1/admin/rbac/user-access/users/${SUBJECT}`, body);
  const tResponse = Date.now() - t0;
  await testInfo.attach("user-access-put.json", { body: JSON.stringify({ status: put.status, warnings: put.json?.data?.warnings, ms: tResponse }, null, 2), contentType: "application/json" });
  expect(put.status, put.text.slice(0, 500)).toBe(200);
  // Nothing incomplete is reported: no identity-sync warnings.
  expect(put.json.data.warnings ?? []).toEqual([]);

  // ---- Within the window: every observable effect has happened.
  // 1. The existing CLIMS session is rejected, in the browser and through the probe.
  expect(await climsMe(climsTab), "CLIMS API with the pre-removal access token").toBe(401);
  expect(await probeCheck("clims", climsProbe, climsToken), "CLIMS probe session").toBe(401);
  // 2. A new CLIMS sign-in is refused (the Keycloak SSO session itself is still live).
  const again = await userContext.newPage();
  const navs = trackNavigations(again);
  await again.goto(`${CLIMS}/api/v1/auth/sso/start`);
  await again.waitForURL((u) => u.pathname === "/sign-in", { timeout: WINDOW_MS });
  expect(new URL(again.url()).searchParams.get("sso_error")).toBe("denied");
  expect(navs.some((u) => u.includes("/login-actions/authenticate"))).toBe(false);
  // 3. POES still works: the browser session and the probe session.
  expect(await poesMe(poesTab), "POES session").toBe(200);
  expect(await probeCheck("poesgo", poesProbe, poesToken), "POES probe session").toBe(200);
  // 4. The revocation record shows every step done.
  const record = sql(dashboardDB, `SELECT application_client_id, issuer, role_removed, consents_revoked, sessions_revoked, coalesce(last_error, ''), restored_at IS NULL, revoked_by FROM ihp_identity_sync_revocations WHERE subject = '${SUBJECT}'`);
  expect(record).toHaveLength(1);
  expect(record[0].slice(0, 7)).toEqual(["clims", ISSUER, "done", "done", "done", "", "t"]);
  const elapsed = Date.now() - t0;
  await testInfo.attach("timing.json", { body: JSON.stringify({ responseMs: tResponse, allChecksMs: elapsed }), contentType: "application/json" });
  expect(elapsed, "all removal effects observed within 5 s of the request").toBeLessThan(WINDOW_MS);

  // ---- After the window: state in Keycloak, the audit trail and the CLI agree.
  expect(await directClientRoles(SUBJECT, "clims")).toEqual([]);
  expect(await directClientRoles(SUBJECT, "poesgo")).toEqual(["poesgo_access"]);
  const listing = cli(["identity-sync", "revocations", "--app", "clims"]).json;
  const view = [listing].flat().find((r) => r.subject === SUBJECT);
  expect(view).toMatchObject({ applicationId: "clims", roleRemoved: "done", consentsRevoked: "done", sessionsRevoked: "done" });
  expect(view.lastError ?? "").toBe("");
  expect(cli(["identity-sync", "revocations", "--app", "clims", "--unfinished"]).json ?? []).not.toContainEqual(expect.objectContaining({ subject: SUBJECT }));
  const audit = sql(dashboardDB, `SELECT action FROM ihp_rbac_audit_events WHERE resource_type = 'identity_sync_revocation' AND details::text LIKE '%${SUBJECT}%' ORDER BY created_at`).flat();
  expect(audit).toEqual(expect.arrayContaining(["application.identity_sync.revocation_recorded", "application.identity_sync.revocation_attempted"]));
  // A new POES sign-in still works.
  const poesAgain = await userContext.newPage();
  await poesAgain.goto(`${POES}/api/v1/auth/sso/start`);
  await poesAgain.waitForURL(`${POES}/api/v1/auth/sso/session`, { timeout: 30_000 });
  await expect(poesAgain.getByText("Signed in to Point-of-Entry Screening.")).toBeVisible();
  // The CLIMS tab itself is signed out on its next API call or reload.
  await climsTab.reload();
  await climsTab.waitForURL((u) => u.pathname === "/sign-in", { timeout: 30_000 });

  // ---- The next batch run proposes "revoked", and apply does not re-grant.
  const runId = cli(["identity-sync", "run", "start", "--app", "clims"]).json.run.id;
  const rows = Object.fromEntries(cli(["identity-sync", "run", "show", "--run", runId]).json.rows.map((r) => [r.accountId, r]));
  expect(rows[CLIMS_ACCOUNT]).toMatchObject({ proposal: "revoked", grantStatus: "not_required", decision: "pending" });
  const approve = cli(["identity-sync", "run", "approve", "--run", runId, "--account", CLIMS_ACCOUNT, "--reviewer", "e2e-reviewer"], { allowFail: true });
  expect(approve.code, "a revoked row cannot be approved").not.toBe(0);
  const apply = cli(["identity-sync", "run", "apply", "--run", runId], { allowFail: true });
  expect(apply.code, apply.stderr).toBe(0);
  const after = cli(["identity-sync", "run", "show", "--run", runId]).json.rows.find((r) => r.accountId === CLIMS_ACCOUNT);
  expect(after).toMatchObject({ proposal: "revoked", decision: "pending", grantStatus: "not_required", attempts: 0 });
  expect(await directClientRoles(SUBJECT, "clims")).toEqual([]);
  // The CLIMS link itself is kept (removal never deletes links or local roles).
  expect(sql(APPS.clims.db, `SELECT subject FROM user_oidc_identities WHERE user_id = '${CLIMS_ACCOUNT}'`)).toEqual([[SUBJECT]]);

  await userContext.close();
  await adminContext.close();
});
