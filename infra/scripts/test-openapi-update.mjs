import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRequire } from "node:module";
import { test } from "node:test";

const require = createRequire(
  new URL("../../frontend/package.json", import.meta.url),
);
const workflow = require("yaml").parse(
  readFileSync(
    new URL("../../.github/workflows/update-openapi.yml", import.meta.url),
    "utf8",
  ),
);
const script = workflow.jobs["openapi-update"].steps.at(-1).run;
const platformContract = "backend/core/platform/contract/platform.openapi.yaml";

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "openapi-update-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const repo = join(root, "repo"),
    remote = join(root, "remote.git"),
    bin = join(root, "bin");
  mkdirSync(repo);
  mkdirSync(bin);
  const git = (...args) =>
    execFileSync("git", args, {
      cwd: repo,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    }).trim();
  git("init", "-b", "contract-fixture");
  git("config", "user.name", "Test");
  git("config", "user.email", "test@example.invalid");
  mkdirSync(join(repo, "backend/docs"), { recursive: true });
  mkdirSync(join(repo, "backend/core/platform/contract"), { recursive: true });
  writeFileSync(join(repo, "backend/docs/openapi.yaml"), "original contract\n");
  writeFileSync(join(repo, platformContract), "original platform contract\n");
  writeFileSync(join(repo, "other.txt"), "original\n");
  git("add", ".");
  git("commit", "-m", "fixture");
  execFileSync("git", ["init", "--bare", remote], { stdio: "ignore" });
  git("remote", "add", "origin", remote);
  git("push", "origin", "HEAD");
  const source = git("rev-parse", "HEAD");
  // The artifact keeps both files relative to backend/.
  const generated = join(root, "generated"),
    log = join(root, "dispatch.log");
  const writeGenerated = (openapi, platform) => {
    mkdirSync(join(generated, "docs"), { recursive: true });
    mkdirSync(join(generated, "core/platform/contract"), { recursive: true });
    writeFileSync(join(generated, "docs/openapi.yaml"), openapi);
    writeFileSync(
      join(generated, "core/platform/contract/platform.openapi.yaml"),
      platform,
    );
  };
  writeGenerated("updated contract\n", "updated platform contract\n");
  writeFileSync(
    join(bin, "gh"),
    `#!/bin/sh
case "$1" in
  api) printf '%s\\n' "$TEST_CURRENT_SHA" ;;
  auth) exit 0 ;;
  workflow) printf '%s\\n' "$*" >> "$TEST_DISPATCH_LOG" ;;
  *) exit 1 ;;
esac
`,
    { mode: 0o755 },
  );
  const env = {
    ...process.env,
    PATH: `${bin}:${process.env.PATH}`,
    GH_REPO: "example/repo",
    PR_NUMBER: "1",
    BRANCH: "contract-fixture",
    SOURCE_SHA: source,
    GENERATED_DIR: generated,
    TEST_CURRENT_SHA: source,
    TEST_DISPATCH_LOG: log,
  };
  return {
    git,
    source,
    remote,
    repo,
    writeGenerated,
    log,
    run: (extra = {}) =>
      spawnSync("bash", ["-c", script], {
        cwd: repo,
        env: { ...env, ...extra },
        encoding: "utf8",
      }),
  };
}

test("bot commits only the generated contract, pushes it and dispatches quality on that branch", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.repo, "other.txt"), "unrelated edit\n");
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(
    f.git("show", "--format=", "--name-only", "HEAD"),
    `${platformContract}\nbackend/docs/openapi.yaml`,
  );
  assert.equal(
    f.git("ls-remote", "origin", "refs/heads/contract-fixture").split(/\s/)[0],
    f.git("rev-parse", "HEAD"),
  );
  assert.match(
    readFileSync(f.log, "utf8"),
    /workflow run pr-check.yml --repo example\/repo --ref contract-fixture/,
  );
  assert.equal(f.git("status", "--porcelain"), "M other.txt");
});

test("bot refuses to commit when the PR head changed during generation", (t) => {
  const f = fixture(t);
  const result = f.run({ TEST_CURRENT_SHA: "newer-commit" });
  assert.notEqual(result.status, 0);
  assert.match(result.stdout, /PR changed during generation/);
  assert.equal(f.git("rev-parse", "HEAD"), f.source);
  assert.equal(
    readFileSync(join(f.repo, "backend/docs/openapi.yaml"), "utf8"),
    "original contract\n",
  );
});

test("non-fast-forward pushes preserve concurrent changes and do not dispatch checks", (t) => {
  const f = fixture(t);
  writeFileSync(join(f.repo, "other.txt"), "concurrent change\n");
  f.git("add", "other.txt");
  f.git("commit", "-m", "concurrent change");
  const newer = f.git("rev-parse", "HEAD");
  f.git("push", "origin", "HEAD");
  f.git("checkout", "--detach", f.source);
  const result = f.run();
  assert.notEqual(result.status, 0);
  assert.equal(
    f.git("ls-remote", "origin", "refs/heads/contract-fixture").split(/\s/)[0],
    newer,
  );
  assert.throws(() => readFileSync(f.log));
});

test("bot commits a platform contract change on its own", (t) => {
  const f = fixture(t);
  f.writeGenerated("original contract\n", "updated platform contract\n");
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(
    f.git("show", "--format=", "--name-only", "HEAD"),
    platformContract,
  );
});

test("unchanged generated contract is a no-op", (t) => {
  const f = fixture(t);
  f.writeGenerated("original contract\n", "original platform contract\n");
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.git("rev-parse", "HEAD"), f.source);
  assert.throws(() => readFileSync(f.log));
});
