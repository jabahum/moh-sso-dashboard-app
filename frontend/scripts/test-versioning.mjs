/* global URL, process */
import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const frontend = fileURLToPath(new URL("..", import.meta.url));
const audit = join(frontend, "scripts/version-check.mjs");
const read = (path) => JSON.parse(readFileSync(path, "utf8"));
function write(path, value) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, JSON.stringify(value));
}

function fixture(run) {
  const root = mkdtempSync(join(tmpdir(), "moh-versioning-"));
  try {
    const lock = read(join(frontend, "package-lock.json"));
    for (const path of Object.keys(lock.packages)) {
      if (/^(apps|packages)\/[^/]+$/.test(path)) {
        write(join(root, path, "package.json"), read(join(frontend, path, "package.json")));
      }
    }
    write(join(root, "package-lock.json"), lock);
    write(
      join(root, "public/version-manifest.json"),
      read(join(frontend, "public/version-manifest.json")),
    );
    write(join(root, ".changeset/config.json"), read(join(frontend, ".changeset/config.json")));
    return run(root);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test("version audit includes all modules and ignores directories without package.json", () => {
  fixture((root) => {
    mkdirSync(join(root, "apps/dashboards"));
    execFileSync(process.execPath, [audit], { cwd: root });
  });
});

for (const [title, path, mutate, expected] of [
  [
    "missing shell version",
    "apps/shell/package.json",
    (pkg) => delete pkg.version,
    /shell.*invalid version/,
  ],
  [
    "incompatible dependency",
    "apps/users/package.json",
    (pkg) => (pkg.dependencies["@moh-sso/api"] = "^99.0.0"),
    /does not accept workspace version/,
  ],
  [
    "shared train drift",
    "packages/api/package.json",
    (pkg) => (pkg.version = "0.2.0"),
    /fixed group has mismatched versions/,
  ],
  [
    "stale lockfile",
    "package-lock.json",
    (lock) => (lock.packages["apps/shell"].version = "0.0.0"),
    /shell.*lockfile version is stale/,
  ],
  [
    "stale manifest",
    "public/version-manifest.json",
    (manifest) => (manifest.apps.shell.version = "0.0.0"),
    /shell.*version manifest is stale/,
  ],
]) {
  test(`version audit rejects ${title}`, () => {
    fixture((root) => {
      const target = join(root, path);
      const value = read(target);
      mutate(value);
      write(target, value);
      const result = spawnSync(process.execPath, [audit], { cwd: root, encoding: "utf8" });
      assert.equal(result.status, 1);
      assert.match(result.stderr, expected);
    });
  });
}

test("backend build versions only identify clean, exact backend tags as releases", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-backend-versioning-"));
  try {
    const script = join(root, "backend/scripts/build-version.sh");
    mkdirSync(dirname(script), { recursive: true });
    writeFileSync(
      script,
      readFileSync(new URL("../../backend/scripts/build-version.sh", import.meta.url)),
    );
    const git = (...args) => execFileSync("git", args, { cwd: root, stdio: "pipe" });
    const version = () => execFileSync("sh", [script], { encoding: "utf8" }).trim();
    git("init", "-q");
    git("add", ".");
    git(
      "-c",
      "user.name=Version Test",
      "-c",
      "user.email=version@example.invalid",
      "-c",
      "commit.gpgsign=false",
      "commit",
      "-qm",
      "fixture",
    );
    git("tag", "frontend/v9.0.0");
    assert.match(version(), /^0\.0\.0-dev\.[0-9a-f]+$/);
    git("tag", "backend/v1.2.3");
    assert.equal(version(), "1.2.3");
    writeFileSync(join(root, "dirty.txt"), "unreleased change");
    assert.match(version(), /^0\.0\.0-dev\.[0-9a-f]+$/);
    git("add", ".");
    git(
      "-c",
      "user.name=Version Test",
      "-c",
      "user.email=version@example.invalid",
      "-c",
      "commit.gpgsign=false",
      "commit",
      "-qm",
      "after release",
    );
    assert.match(version(), /^0\.0\.0-dev\.[0-9a-f]+$/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
