/* global URL */
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import { releaseModules, verifyNpmRelease } from "./verify-npm-release.mjs";

const module = { name: "@moh-sso/users", version: "1.3.1" };
const response = (
  status,
  metadata = { ...module, dist: { tarball: "https://registry.npmjs.org/users.tgz" } },
) => ({ status, ok: status === 200, json: async () => metadata });

test("all published versions succeed without waiting or publishing", async () => {
  await verifyNpmRelease([module], {
    lookup: async () => response(200),
    wait: () => assert.fail("No retry expected"),
  });
});
test("retry eventual consistency and only query versions still missing", async () => {
  const second = { name: "@moh-sso/api", version: "1.3.1" };
  const calls = [];
  let waits = 0;
  await verifyNpmRelease([module, second], {
    lookup: async (item) => {
      calls.push(item.name);
      return item.name === second.name && calls.length === 2
        ? response(404)
        : response(200, { ...item, dist: { tarball: "https://registry.npmjs.org/fixture.tgz" } });
    },
    wait: async () => {
      waits++;
    },
  });
  assert.deepEqual(calls, [module.name, second.name, second.name]);
  assert.equal(waits, 1);
});
test("partially published releases fail and identify missing versions", async () => {
  await assert.rejects(
    verifyNpmRelease([module], {
      lookup: async () => response(404),
      wait: async () => {},
      attempts: 2,
    }),
    /incomplete.*@moh-sso\/users@1.3.1/,
  );
});
test("transient registry failures are retried", async () => {
  let count = 0;
  await verifyNpmRelease([module], {
    lookup: async () => response(++count === 1 ? 503 : 200),
    wait: async () => {},
  });
  assert.equal(count, 2);
});
for (const metadata of [
  { ...module, version: "1.3.0", dist: { tarball: "fixture" } },
  { ...module, name: "@moh-sso/other", dist: { tarball: "fixture" } },
  { ...module },
]) {
  test(`reject incorrect registry metadata ${JSON.stringify(metadata)}`, async () => {
    await assert.rejects(
      verifyNpmRelease([module], { lookup: async () => response(200, metadata) }),
      /Unexpected npm metadata/,
    );
  });
}
test("authentication failures are not mistaken for unpublished versions", async () => {
  await assert.rejects(
    verifyNpmRelease([module], {
      lookup: async () => response(401),
      wait: () => assert.fail("Must fail immediately"),
    }),
    /HTTP 401/,
  );
});
test("public module discovery excludes the shell and rejects accidental private publishing", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-npm-release-"));
  const write = (path, value) => {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, JSON.stringify(value));
  };
  try {
    mkdirSync(join(root, "packages"));
    write(join(root, "apps/shell/package.json"), { private: true });
    const path = join(root, "apps/users/package.json");
    const pkg = {
      ...module,
      private: false,
      publishConfig: { access: "public" },
      repository: {
        url: "git+https://github.com/mohuganda/moh-sso-dashboard.git",
        directory: "frontend/apps/users",
      },
    };
    write(path, pkg);
    assert.deepEqual(releaseModules(root), [module]);
    write(path, { ...pkg, private: true });
    assert.throws(() => releaseModules(root), /public access/);
    write(path, { ...pkg, repository: {} });
    assert.throws(() => releaseModules(root), /upstream repository/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
test("tag releases build and publish npm modules before publishing GitHub release notes", () => {
  const root = fileURLToPath(new URL("../", import.meta.url));
  const workflow = parse(
    readFileSync(join(root, "../.github/workflows/frontend-release.yml"), "utf8"),
  );
  assert.deepEqual(workflow.on.push.tags, ["frontend/v*.*.*"]);
  assert.equal(workflow.on.pull_request, undefined);
  const job = workflow.jobs.release;
  assert.match(job.if, /github.repository == 'mohuganda\/moh-sso-dashboard'/);
  assert.equal(job.permissions["id-token"], "write");
  const steps = job.steps;
  const index = (run) => steps.findIndex((step) => step.run === run);
  const check = index("npm run release:verify-npm -- --check");
  const build = index("npm run release:dry-run");
  const publish = index("npm run release:packages -- --no-git-tag");
  const confirm = index("npm run release:verify-npm");
  const notes = steps.findIndex((step) => step.uses?.startsWith("softprops/action-gh-release@"));
  assert.ok(check >= 0 && build > check && publish > build && confirm > publish && notes > confirm);
  assert.equal(steps[publish].env.NODE_AUTH_TOKEN, "${{ secrets.NPM_TOKEN }}");
});
