import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { releaseTags } from "./release.mjs";
import { withPreservedFiles } from "./check-frontend-release.mjs";

test("combined release retains independent component versions", () => {
  assert.deepEqual(releaseTags("both", "0.0.2", "0.1.1", "0.1.1"), [
    "backend/v0.0.2",
    "frontend/v0.1.1",
  ]);
  assert.deepEqual(releaseTags("backend", "1.2.3", "", "0.1.1"), [
    "backend/v1.2.3",
  ]);
  assert.deepEqual(releaseTags("frontend", "", "0.1.1", "0.1.1"), [
    "frontend/v0.1.1",
  ]);
});

test("Make verifies both components before publishing a combined release", () => {
  const result = spawnSync(
    "make",
    [
      "-n",
      "release-manual",
      "BACKEND_VERSION=0.0.2",
      "FRONTEND_VERSION=0.1.1",
      `NODE=${process.execPath}`,
    ],
    {
      cwd: new URL("../", import.meta.url),
      encoding: "utf8",
    },
  );
  assert.equal(result.status, 0, result.stderr);
  const plan = result.stdout;
  assert.match(plan, /check both "0\.0\.2" "0\.1\.1"/);
  const backend = plan.indexOf('-C backend release-check VERSION="0.0.2"');
  const frontend = plan.indexOf("scripts/check-frontend-release.mjs");
  const publication = plan.indexOf('publish both "0.0.2" "0.1.1"');
  assert.ok(backend >= 0 && frontend > backend && publication > frontend);
});

test("releases reject mismatched shell versions and invalid Docker-compatible SemVer", () => {
  assert.throws(
    () => releaseTags("frontend", "", "0.1.2", "0.1.1"),
    /does not match shell/,
  );
  for (const invalid of [
    "",
    "dev",
    "v1.2.3",
    "01.2.3",
    "1.2.3-01",
    "1.2.3+build",
    "1.2.3;echo bad",
  ]) {
    assert.throws(
      () => releaseTags("backend", invalid, "", ""),
      /semantic version/,
    );
  }
  assert.deepEqual(releaseTags("backend", "1.2.3-rc.1", "", ""), [
    "backend/v1.2.3-rc.1",
  ]);
});

test("frontend checks preserve generated source files after success and failure", () => {
  const dir = mkdtempSync(join(tmpdir(), "moh-release-check-"));
  const path = join(dir, "config.js");
  try {
    writeFileSync(path, "original");
    withPreservedFiles([path], () => writeFileSync(path, "generated"));
    assert.equal(readFileSync(path, "utf8"), "original");
    assert.throws(
      () =>
        withPreservedFiles([path], () => {
          writeFileSync(path, "failed build");
          throw new Error("build failed");
        }),
      /build failed/,
    );
    assert.equal(readFileSync(path, "utf8"), "original");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("combined publication preflights both tags and pushes them atomically", () => {
  const dir = mkdtempSync(join(tmpdir(), "moh-release-publish-"));
  try {
    mkdirSync(join(dir, "scripts"));
    mkdirSync(join(dir, "frontend/apps/shell"), { recursive: true });
    mkdirSync(join(dir, "mock-bin"));
    writeFileSync(
      join(dir, "scripts/release.mjs"),
      readFileSync(new URL("./release.mjs", import.meta.url)),
    );
    writeFileSync(
      join(dir, "frontend/apps/shell/package.json"),
      '{"version":"0.1.1"}',
    );
    const log = join(dir, "git.log");
    writeFileSync(
      join(dir, "mock-bin/git"),
      `#!/bin/sh
printf '%s\\n' "$*" >> "$RELEASE_TEST_LOG"
case "$1" in
  ls-remote) if [ "$RELEASE_TEST_EXISTING" = yes ]; then printf 'sha\\trefs/tags/frontend/v0.1.1\\n'; fi ;;
esac
`,
      { mode: 0o755 },
    );
    const run = (existing) =>
      spawnSync(
        process.execPath,
        [
          join(dir, "scripts/release.mjs"),
          "publish",
          "both",
          "0.0.2",
          "0.1.1",
          "upstream",
        ],
        {
          encoding: "utf8",
          env: {
            ...process.env,
            PATH: `${join(dir, "mock-bin")}:${process.env.PATH}`,
            RELEASE_TEST_LOG: log,
            RELEASE_TEST_EXISTING: existing,
          },
        },
      );
    const success = run("no");
    assert.equal(success.status, 0, success.stderr);
    const calls = readFileSync(log, "utf8");
    assert.match(
      calls,
      /push --atomic upstream refs\/tags\/backend\/v0\.0\.2 refs\/tags\/frontend\/v0\.1\.1/,
    );
    assert.ok(calls.indexOf("ls-remote") < calls.indexOf("tag -a"));
    writeFileSync(log, "");
    const conflict = run("yes");
    assert.equal(conflict.status, 1);
    assert.match(conflict.stderr, /already exist on the remote/);
    assert.doesNotMatch(readFileSync(log, "utf8"), /tag -a|push /);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
