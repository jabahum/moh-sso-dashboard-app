// Suite 1 — Contract. Runs the dashboard's conformance suite
// (`moh-sso identity-sync conformance`) against CLIMS and POES with real
// tokens minted from the e2e Keycloak, plus garbage/tampered tokens.
//
// Users and accounts owned by this suite: Keycloak `contract.user` (user token),
// CLIMS accounts conf1/conf2/confinelig, POES accounts 8/9/10. The suite links
// conf1/conf2 to random synthetic subjects.
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import {
  APPS, ISSUER, SYNC_BASE, claims, cli, clientToken, clientUUID, climsId, kc, passwordToken,
  request, serviceAccountUser, setClientRole, syncToken,
} from "../lib/stack.mjs";

const FIXTURES = {
  clims: { linkableAccounts: [climsId(8), climsId(9)], ineligibleAccount: climsId(10) },
  poesgo: { linkableAccounts: ["8", "9"], ineligibleAccount: "10" },
};

/** A real sync-client token whose `identity_sync` role was removed for the mint, then restored. */
async function missingRoleToken(app) {
  const { syncClient, syncSecret } = APPS[app];
  const sa = await serviceAccountUser(syncClient);
  await setClientRole(sa.id, app, "identity_sync", false);
  try {
    return await clientToken(syncClient, syncSecret);
  } finally {
    await setClientRole(sa.id, app, "identity_sync", true);
  }
}

/** A real sync-client token that is already expired: minted with a 2 s lifespan. */
async function expiredToken(app) {
  const { syncClient, syncSecret } = APPS[app];
  const uuid = await clientUUID(syncClient);
  const rep = await kc("GET", `/clients/${uuid}`);
  const before = rep.attributes?.["access.token.lifespan"] ?? "";
  await kc("PUT", `/clients/${uuid}`, { ...rep, attributes: { ...rep.attributes, "access.token.lifespan": "2" } });
  let token;
  try {
    token = await clientToken(syncClient, syncSecret);
  } finally {
    const now = await kc("GET", `/clients/${uuid}`);
    await kc("PUT", `/clients/${uuid}`, { ...now, attributes: { ...now.attributes, "access.token.lifespan": before } });
  }
  const { exp, iat } = claims(token);
  expect(exp - iat, "expired-token fixture lifespan").toBeLessThanOrEqual(2);
  // The lifespan override is gone again: a fresh token lives the realm default.
  const fresh = claims(await clientToken(syncClient, syncSecret));
  expect(fresh.exp - fresh.iat).toBeGreaterThan(60);
  const waitMs = exp * 1000 - Date.now() + 3000;
  if (waitMs > 0) await new Promise((r) => setTimeout(r, waitMs));
  return token;
}

for (const app of ["clims", "poesgo"]) {
  test.describe(`contract: ${app}`, () => {
    test(`conformance suite passes against ${app} with real Keycloak tokens`, async ({}, testInfo) => {
      const tokens = {
        IDENTITY_SYNC_TOKEN: await syncToken(app),
        // dashboard-admin's service token: a valid realm token without the app in aud.
        IDENTITY_SYNC_WRONG_AUDIENCE_TOKEN: await clientToken("dashboard-admin", "e2e-fake-dashboard-admin-secret"),
        IDENTITY_SYNC_USER_TOKEN: await passwordToken("contract.user"),
        IDENTITY_SYNC_OTHER_CLIENT_TOKEN: await clientToken("e2e-other-service", "e2e-fake-other-service-secret"),
        IDENTITY_SYNC_MISSING_ROLE_TOKEN: await missingRoleToken(app),
        IDENTITY_SYNC_EXPIRED_TOKEN: await expiredToken(app),
      };
      // Each token really is the case it stands for.
      const aud = (t) => [claims(t).aud].flat();
      const roles = (t) => claims(t).resource_access?.[app]?.roles ?? [];
      expect(claims(tokens.IDENTITY_SYNC_TOKEN).azp).toBe(APPS[app].syncClient);
      expect(aud(tokens.IDENTITY_SYNC_TOKEN)).toContain(app);
      expect(roles(tokens.IDENTITY_SYNC_TOKEN)).toContain("identity_sync");
      expect(aud(tokens.IDENTITY_SYNC_WRONG_AUDIENCE_TOKEN)).not.toContain(app);
      expect(claims(tokens.IDENTITY_SYNC_USER_TOKEN).azp).toBe("e2e-user-token");
      expect(aud(tokens.IDENTITY_SYNC_USER_TOKEN)).toContain(app);
      expect(claims(tokens.IDENTITY_SYNC_OTHER_CLIENT_TOKEN).azp).toBe("e2e-other-service");
      expect(aud(tokens.IDENTITY_SYNC_OTHER_CLIENT_TOKEN)).toContain(app);
      expect(claims(tokens.IDENTITY_SYNC_MISSING_ROLE_TOKEN).azp).toBe(APPS[app].syncClient);
      expect(roles(tokens.IDENTITY_SYNC_MISSING_ROLE_TOKEN)).not.toContain("identity_sync");
      expect(claims(tokens.IDENTITY_SYNC_EXPIRED_TOKEN).exp * 1000).toBeLessThan(Date.now());

      const dir = mkdtempSync(join(tmpdir(), "idsync-conformance-"));
      writeFileSync(join(dir, "fixtures.json"), JSON.stringify({ application: app, issuer: ISSUER, ...FIXTURES[app] }));
      const res = cli(
        ["identity-sync", "conformance", "--app", app, "--origin", APPS[app].origin, "--fixtures", "/conformance/fixtures.json",
          "--probe", "--ca-file", "/e2e-ca/root.crt", "--json"],
        { env: tokens, volumes: [`${dir}:/conformance:ro`], allowFail: true },
      );
      await testInfo.attach(`conformance-${app}.json`, { body: res.stdout, contentType: "application/json" });
      expect(res.json, `conformance printed no JSON report (exit ${res.code}): ${res.stderr}`).toBeTruthy();
      const results = res.json.results;
      for (const r of results) console.log(`[${app}] ${r.status.toUpperCase().padEnd(4)} ${r.id} ${r.detail ?? ""}`);
      const failed = results.filter((r) => r.status === "fail");
      const skipped = results.filter((r) => r.status === "skip");
      expect(failed, "failed conformance checks").toEqual([]);
      // Every token and the probe are supplied, so the only allowed skip is the one
      // the design assigns elsewhere ("Retries | Covered by connector unit tests
      // with a fault-injecting fake server"), and it must state that reason.
      expect(skipped.map((r) => r.id), "skipped conformance checks").toEqual(["transport.retries"]);
      expect(skipped[0].detail).toMatch(/unit tests/);
      expect(results.length).toBeGreaterThan(20);
      expect(res.code).toBe(0);
      for (const area of ["authorization", "descriptor", "pagination", "linking", "sessions", "transport"]) {
        expect(results.some((r) => r.area.toLowerCase() === area), `area ${area} ran`).toBe(true);
      }
    });

    test(`${app} refuses garbage and tampered tokens with 401`, async () => {
      const url = APPS[app].origin + SYNC_BASE;
      const valid = await syncToken(app);
      const [h, p, s] = valid.split(".");
      const flipped = s.slice(0, -2) + (s.at(-2) === "A" ? "B" : "A") + s.at(-1);
      const forgedPayload = Buffer.from(JSON.stringify({ ...claims(valid), azp: "someone-else" })).toString("base64url");
      const cases = {
        garbage: "not-a-jwt",
        "tampered signature": `${h}.${p}.${flipped}`,
        "edited payload, original signature": `${h}.${forgedPayload}.${s}`,
        "alg none": `${Buffer.from('{"alg":"none","typ":"JWT"}').toString("base64url")}.${p}.`,
      };
      for (const [name, token] of Object.entries(cases)) {
        const res = await request("GET", url, { headers: { authorization: `Bearer ${token}` } });
        expect(res.status, name).toBe(401);
        expect(res.headers["cache-control"], `${name}: Cache-Control`).toContain("no-store");
      }
      const ok = await request("GET", url, { headers: { authorization: `Bearer ${valid}` } });
      expect(ok.status).toBe(200);
    });
  });
}
