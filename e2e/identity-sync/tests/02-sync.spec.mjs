// Suite 2 — Sync. Reviewed batch runs through the dashboard CLI
// (`identity-sync run start|show|approve|reject|apply|resume`).
//
// Users and accounts this suite changes: CLIMS clims.unlinked (reuse, forced
// grant failure, resume) and POES poes.unlinked (reuse) plus poes.invitee
// (create/invitation). Every other row is only read.
import { test, expect } from "@playwright/test";
import {
  APPS, ISSUER, cli, climsId, directClientRoles, findUsers, kc, kcId, mailMessage, mailTo,
  serviceAccountUser, setClientRole, sql, waitFor,
} from "../lib/stack.mjs";

const REVIEWER = "e2e-reviewer@e2e.test";

function startRun(app) {
  const res = cli(["identity-sync", "run", "start", "--app", app]);
  expect(res.json.run.status).toBe("previewed");
  return res.json.run.id;
}

function showRun(runId) {
  const res = cli(["identity-sync", "run", "show", "--run", runId]);
  return { run: res.json.run, rows: Object.fromEntries(res.json.rows.map((r) => [r.accountId, r])) };
}

function expectProposals(rows, expected) {
  for (const [account, proposal] of Object.entries(expected)) {
    if (proposal === null) expect(rows[account], `account ${account} must not be listed`).toBeUndefined();
    else expect(rows[account]?.proposal, `proposal for account ${account}`).toBe(proposal);
  }
}

/** The app-side link row, including xmin so any UPDATE (or re-INSERT) is detected. */
function climsLink(accountId) {
  return sql(APPS.clims.db, `SELECT id, issuer, subject, user_id, created_at, xmin FROM user_oidc_identities WHERE user_id = '${accountId}'`);
}


test("CLIMS: proposals as seeded, reviewed approval, forced grant failure, resume without relinking", async () => {
  const runId = startRun("clims");
  let { rows } = showRun(runId);
  expectProposals(rows, {
    [climsId(1)]: "linked", // both.apps
    [climsId(2)]: "linked", // clims.linked
    [climsId(3)]: "reuse", // clims.unlinked
    [climsId(4)]: "linked_no_access", // clims.noaccess (batch run separates it)
    [climsId(5)]: null, // clims.disabled: inactive, not eligible
    [climsId(6)]: "create", // clims.invitee: no Keycloak user
    [climsId(7)]: "review", // clims.kcdisabled: only a disabled Keycloak user
    [climsId(10)]: null, // confinelig
    [climsId(11)]: "review", // filler1: no email
    [climsId(12)]: "review",
    [climsId(13)]: "review",
    [climsId(14)]: null, // clims.roleless: no local role
  });
  expect(rows[climsId(1)]).toMatchObject({ linkStatus: "linked", grantStatus: "granted" });
  expect(rows[climsId(4)]).toMatchObject({ linkStatus: "linked", grantStatus: "not_required" });
  expect(rows[climsId(3)]).toMatchObject({ decision: "pending", attempts: 0 });
  for (const r of Object.values(rows)) expect(r.decision).toBe("pending");

  // Only reuse/create rows can be approved; a review row is refused and stays pending.
  const refused = cli(["identity-sync", "run", "approve", "--run", runId, "--account", climsId(7), "--reviewer", REVIEWER], { allowFail: true });
  expect(refused.code).not.toBe(0);
  expect(showRun(runId).rows[climsId(7)].decision).toBe("pending");
  // A reviewer is required.
  expect(cli(["identity-sync", "run", "approve", "--run", runId, "--account", climsId(3), "--reviewer", " "], { allowFail: true }).code).not.toBe(0);

  cli(["identity-sync", "run", "approve", "--run", runId, "--account", climsId(3), "--reviewer", REVIEWER, "--note", "e2e: same email, verified"]);
  cli(["identity-sync", "run", "reject", "--run", runId, "--account", climsId(6), "--reviewer", REVIEWER, "--note", "e2e: not this run"]);
  ({ rows } = showRun(runId));
  expect(rows[climsId(3)]).toMatchObject({ decision: "approved", reviewedBy: REVIEWER, reviewNote: "e2e: same email, verified" });
  expect(rows[climsId(3)].reviewedAt).toBeTruthy();
  expect(rows[climsId(6)]).toMatchObject({ decision: "rejected", reviewedBy: REVIEWER });

  const subject = kcId(102); // clims.unlinked
  expect(climsLink(climsId(3))).toEqual([]);
  expect(await directClientRoles(subject, "clims")).not.toContain("clims_access");

  // Fault: the dashboard's Keycloak admin client loses the right to map roles.
  const sa = await serviceAccountUser("dashboard-admin");
  const removed = ["realm-admin", "manage-users"];
  for (const role of removed) await setClientRole(sa.id, "realm-management", role, false);
  let apply;
  try {
    apply = cli(["identity-sync", "run", "apply", "--run", runId], { allowFail: true });
  } finally {
    for (const role of removed) await setClientRole(sa.id, "realm-management", role, true);
  }
  expect(apply.code, `apply must report unfinished rows: ${apply.stderr}`).not.toBe(0);
  ({ rows } = showRun(runId));
  const failedRow = rows[climsId(3)];
  expect(failedRow).toMatchObject({ linkStatus: "linked", grantStatus: "failed", lastError: "grant_failed", attempts: 1 });
  expect(showRun(runId).run.status).toBe("failed");
  expect(await directClientRoles(subject, "clims")).not.toContain("clims_access");
  const linkAfterApply = climsLink(climsId(3));
  expect(linkAfterApply).toHaveLength(1);
  expect(linkAfterApply[0].slice(1, 4)).toEqual([ISSUER, subject, climsId(3)]);
  // Rejected and unapproved rows were not touched.
  expect(rows[climsId(6)]).toMatchObject({ attempts: 0, userStatus: "pending" });
  expect(await findUsers({ email: "clims.invitee@e2e.test" })).toEqual([]);

  // Resume with the fault fixed: only the grant runs.
  const resume = cli(["identity-sync", "run", "resume", "--run", runId], { allowFail: true });
  expect(resume.code, resume.stderr).toBe(0);
  ({ rows } = showRun(runId));
  // Each apply/resume call counts as one attempt (identityreconcile.processRow).
  expect(rows[climsId(3)]).toMatchObject({ linkStatus: "linked", grantStatus: "granted", attempts: 2 });
  expect(rows[climsId(3)].lastError ?? "").toBe("");
  expect(showRun(runId).run.status).toBe("completed");
  expect(await directClientRoles(subject, "clims")).toContain("clims_access");
  // The app-side link was not recreated or rewritten: same row id, created_at and xmin.
  expect(climsLink(climsId(3))).toEqual(linkAfterApply);

  // Resuming a completed run is a no-op.
  expect(cli(["identity-sync", "run", "resume", "--run", runId], { allowFail: true }).code).toBe(0);
  expect(showRun(runId).rows[climsId(3)].attempts).toBe(2);
  expect(climsLink(climsId(3))).toEqual(linkAfterApply);

  // The next run sees the account as linked and proposes nothing for it.
  const next = showRun(startRun("clims")).rows;
  expect(next[climsId(3)]).toMatchObject({ proposal: "linked", grantStatus: "granted" });
});

test("POES: invitation for an account without a Keycloak user, reuse, and idempotent re-runs", async () => {
  const runId = startRun("poesgo");
  let { rows } = showRun(runId);
  expectProposals(rows, {
    1: "linked", // both.apps
    2: "linked", // poes.linked
    3: "reuse", // poes.unlinked
    4: "linked_no_access", // poes.noaccess
    5: null, // poes.disabled (inactive)
    6: "create", // poes.invitee
    7: null, // poes.locked
    10: null, // confinelig
    11: "review",
    12: "review",
    13: "review",
    14: null, // poes.roleless
  });
  expect(await findUsers({ email: "poes.invitee@e2e.test" })).toEqual([]);
  expect(await mailTo("poes.invitee@e2e.test")).toEqual([]);

  cli(["identity-sync", "run", "approve", "--run", runId, "--account", "3,6", "--reviewer", REVIEWER, "--note", "e2e invitation"]);
  const apply = cli(["identity-sync", "run", "apply", "--run", runId], { allowFail: true });
  expect(apply.code, apply.stderr).toBe(0);
  ({ rows } = showRun(runId));
  expect(showRun(runId).run.status).toBe("completed");
  expect(rows["3"]).toMatchObject({ linkStatus: "linked", grantStatus: "granted", attempts: 1 });
  expect(rows["3"].proposedSubject).toBe(kcId(202));
  expect(rows["6"]).toMatchObject({ userStatus: "invited", linkStatus: "linked", grantStatus: "granted", attempts: 1 });

  // Keycloak: one new user, unverified, with the invitation actions and the origin marker.
  const invited = await findUsers({ email: "poes.invitee@e2e.test" });
  expect(invited).toHaveLength(1);
  const user = invited[0];
  expect(rows["6"].proposedSubject).toBe(user.id);
  expect(user).toMatchObject({ username: "poes.invitee@e2e.test", enabled: true, emailVerified: false });
  expect([...user.requiredActions].sort()).toEqual(["UPDATE_PASSWORD", "VERIFY_EMAIL"]);
  expect(user.attributes?.ihp_identity_sync_origin).toEqual(["poesgo:6"]);
  expect(await directClientRoles(user.id, "poesgo")).toEqual(["poesgo_access"]);
  const realmRoles = (await kc("GET", `/users/${user.id}/role-mappings/realm`)).map((r) => r.name);
  expect(realmRoles.filter((r) => !r.startsWith("default-roles-"))).toEqual([]);
  expect(await directClientRoles(kcId(202), "poesgo")).toContain("poesgo_access");

  // POES stored both links to exactly those subjects.
  const links = sql(APPS.poesgo.db, "SELECT user_id, issuer, subject FROM user_oidc_identities WHERE user_id IN (3, 6) ORDER BY user_id");
  expect(links).toEqual([["3", ISSUER, kcId(202)], ["6", ISSUER, user.id]]);

  // The execute-actions email arrives in mailpit with a Keycloak action link.
  const mails = await waitFor(async () => {
    const m = await mailTo("poes.invitee@e2e.test");
    return m.length > 0 && m;
  }, { what: "invitation email in mailpit" });
  expect(mails).toHaveLength(1);
  const message = await mailMessage(mails[0].ID);
  expect(message.Text + message.HTML).toContain("https://sso.e2e.test:18443/realms/moh-realm/login-actions/action-token");
  expect(await mailTo("poes.unlinked@e2e.test")).toEqual([]);

  // Re-running is idempotent: apply and resume change nothing and send nothing.
  for (const verb of ["apply", "resume"]) {
    expect(cli(["identity-sync", "run", verb, "--run", runId], { allowFail: true }).code, verb).toBe(0);
  }
  const again = showRun(runId).rows;
  expect(again["3"].attempts).toBe(1);
  expect(again["6"].attempts).toBe(1);
  // A new run shows both as linked and leaves nothing to approve or apply for them.
  const runId2 = startRun("poesgo");
  const rows2 = showRun(runId2).rows;
  expect(rows2["3"]).toMatchObject({ proposal: "linked", grantStatus: "granted" });
  expect(rows2["6"]).toMatchObject({ proposal: "linked", grantStatus: "granted" });
  const refused = cli(["identity-sync", "run", "approve", "--run", runId2, "--account", "6", "--reviewer", REVIEWER], { allowFail: true });
  expect(refused.code).not.toBe(0);
  expect(cli(["identity-sync", "run", "apply", "--run", runId2], { allowFail: true }).code).toBe(0);
  expect(await findUsers({ email: "poes.invitee@e2e.test" })).toHaveLength(1);
  expect(sql(APPS.poesgo.db, "SELECT count(*) FROM user_oidc_identities WHERE user_id IN (3, 6)")).toEqual([["2"]]);
  await new Promise((r) => setTimeout(r, 2000));
  expect(await mailTo("poes.invitee@e2e.test")).toHaveLength(1);
});

test("CLIMS: an unverified Keycloak email needs review, and apply sends changed rows back to review", async () => {
  const runId = startRun("clims");
  let { rows } = showRun(runId);
  // clims.unverified: one Keycloak user with this email, but the email is not verified.
  // (duplicateEmailsAllowed is false in the realm, so two users can never share an email.)
  expectProposals(rows, {
    [climsId(0x12)]: "review",
    [climsId(0x13)]: "reuse",
    [climsId(0x14)]: "reuse",
  });
  expect(rows[climsId(0x12)].proposedSubject ?? "").toBe("");
  const refused = cli(["identity-sync", "run", "approve", "--run", runId, "--account", climsId(0x12), "--reviewer", REVIEWER], { allowFail: true });
  expect(refused.code).not.toBe(0);
  expect(rows[climsId(0x13)].proposedSubject).toBe(kcId(108));
  expect(rows[climsId(0x14)].proposedSubject).toBe(kcId(109));

  cli(["identity-sync", "run", "approve", "--run", runId, "--account", `${climsId(0x13)},${climsId(0x14)}`, "--reviewer", REVIEWER, "--note", "e2e recheck"]);
  ({ rows } = showRun(runId));
  expect(rows[climsId(0x13)].decision).toBe("approved");
  expect(rows[climsId(0x14)].decision).toBe("approved");

  // Between approval and apply: the directory match stops qualifying (email no
  // longer verified), and the application account's email changes.
  const user = await kc("GET", `/users/${kcId(108)}`);
  await kc("PUT", `/users/${kcId(108)}`, { ...user, emailVerified: false });
  expect((await kc("GET", `/users/${kcId(108)}`)).emailVerified).toBe(false);
  sql(APPS.clims.db, `UPDATE users SET email = 'clims.recheckacct.moved@e2e.test' WHERE id = '${climsId(0x14)}'`);

  const apply = cli(["identity-sync", "run", "apply", "--run", runId], { allowFail: true });
  ({ rows } = showRun(runId));
  expect(rows[climsId(0x13)], apply.stderr).toMatchObject({ proposal: "review", decision: "pending", lastError: "directory_changed" });
  expect(rows[climsId(0x14)], apply.stderr).toMatchObject({ proposal: "review", decision: "pending", lastError: "account_changed" });
  // Nothing was linked or granted for either account.
  expect(climsLink(climsId(0x13))).toEqual([]);
  expect(climsLink(climsId(0x14))).toEqual([]);
  expect(await directClientRoles(kcId(108), "clims")).not.toContain("clims_access");
  expect(await directClientRoles(kcId(109), "clims")).not.toContain("clims_access");
  // A row sent back to review cannot be applied by re-running without a new decision.
  const again = cli(["identity-sync", "run", "resume", "--run", runId], { allowFail: true });
  expect(again.code, again.stderr).toBe(0);
  expect(climsLink(climsId(0x13))).toEqual([]);
  expect(climsLink(climsId(0x14))).toEqual([]);
  expect(apply.code, `apply exit (stderr: ${apply.stderr})`).toBe(0);
});
