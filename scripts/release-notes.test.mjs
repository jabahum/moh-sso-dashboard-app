import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  changelogEntry,
  generateReleaseNotes,
  updateChangelog,
} from "./release-notes.mjs";

test("changelog updates preserve existing entries and authored summaries", () => {
  const original =
    "# Changelog\n\n## 0.1.1\n\n### Patch Changes\n\nFix session expiry.\n\n## 0.1.0\n\nInitial release.\n";
  assert.match(changelogEntry(original, "0.1.1"), /Fix session expiry/);
  const updated = updateChangelog(
    original,
    "0.1.1",
    "Fix session expiry.\n\n<!-- scoped-source-changes -->\nDetails.",
  );
  assert.equal(changelogEntry(updated, "0.1.1"), "Fix session expiry.");
  assert.match(updated, /## 0.1.0\n\nInitial release/);
  assert.equal(
    updateChangelog(updated, "0.1.1", "Replacement").match(/## 0.1.1/g).length,
    1,
  );
});

test("release notes isolate frontend modules and backend history, exclude current tags, and are repeatable", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-release-notes-"));
  try {
    const git = (...args) =>
      execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
    const commit = (message) => {
      git("add", ".");
      git("commit", "-qm", message);
    };
    for (const folder of [
      "backend",
      "frontend/apps/shell",
      "frontend/apps/users",
      "frontend/packages/api",
      "frontend/apps/empty",
    ])
      mkdirSync(join(root, folder), { recursive: true });
    writeFileSync(join(root, "backend/VERSION"), "0.0.2\n");
    for (const [folder, name] of [
      ["apps/shell", "shell"],
      ["apps/users", "users"],
      ["packages/api", "api"],
    ]) {
      writeFileSync(
        join(root, "frontend", folder, "package.json"),
        JSON.stringify({ name: `@moh-sso/${name}`, version: "0.1.1" }),
      );
    }
    git("init", "-q");
    git("config", "user.name", "Notes Test");
    git("config", "user.email", "notes@example.invalid");
    git("config", "commit.gpgsign", "false");
    commit("Initial baseline");
    git("tag", "frontend/v0.1.0");
    git("tag", "backend/v0.0.1");
    writeFileSync(join(root, "frontend/apps/users/index.ts"), "user fix");
    commit("Fix users search");
    writeFileSync(join(root, "backend/server.go"), "backend fix");
    commit("Fix backend sessions");
    writeFileSync(
      join(root, "frontend/packages/api/CHANGELOG.md"),
      "# Changelog\n\n## 0.1.1\n\nPreserve this authored API summary.\n",
    );
    commit("Prepare release: backend/v0.0.2, frontend/v0.1.1");
    git("tag", "frontend/v0.1.1");
    git("tag", "backend/v0.0.2");
    const result = generateReleaseNotes({ root });
    const notes = (path) =>
      readFileSync(join(root, ".release/notes", path), "utf8");
    assert.equal(result.metadata.components.frontend.modules.length, 3);
    assert.equal(
      result.metadata.components.frontend.previousTag,
      "frontend/v0.1.0",
    );
    assert.match(notes("frontend/apps/users/0.1.1.md"), /Fix users search/);
    assert.doesNotMatch(
      notes("frontend/apps/users/0.1.1.md"),
      /Fix backend sessions|Initial baseline|Prepare release:/,
    );
    assert.match(
      notes("frontend/packages/api/0.1.1.md"),
      /Preserve this authored API summary/,
    );
    assert.doesNotMatch(
      notes("frontend/packages/api/0.1.1.md"),
      /Fix users search/,
    );
    assert.match(
      notes("frontend/apps/shell/0.1.1.md"),
      /No direct source changes/,
    );
    assert.match(notes("backend.md"), /Fix backend sessions/);
    assert.doesNotMatch(
      notes("backend.md"),
      /Fix users search|Initial baseline/,
    );
    const first = notes("frontend.md");
    generateReleaseNotes({ root });
    assert.equal(notes("frontend.md"), first);
    assert.equal(
      readFileSync(join(root, "backend/CHANGELOG.md"), "utf8").match(
        /## 0.0.2/g,
      ).length,
      1,
    );
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
