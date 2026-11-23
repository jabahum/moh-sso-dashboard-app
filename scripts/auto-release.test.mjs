import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  frontendChangeset,
  nextBackendVersion,
  prepareRelease,
} from "./auto-release.mjs";

test("Make dry-runs never execute automatic version preparation or publication", () => {
  const result = spawnSync(
    "make",
    ["-n", "release", `NODE=${process.execPath}`],
    {
      cwd: new URL("../", import.meta.url),
      encoding: "utf8",
    },
  );
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /auto-release.mjs release both/);
  assert.doesNotMatch(
    result.stdout,
    /Prepared and committed|Reusing prepared release/,
  );
});

test("automatic backend versions use the highest prepared or published version", () => {
  assert.equal(
    nextBackendVersion("0.0.1", ["backend/v0.0.4", "frontend/v9.0.0"], "patch"),
    "0.0.5",
  );
  assert.equal(
    nextBackendVersion("1.2.0", ["backend/v1.1.0"], "minor"),
    "1.3.0",
  );
  assert.equal(nextBackendVersion("1.2.0", [], "major"), "2.0.0");
});

test("automatic changeset includes every app and shared package", () => {
  const result = frontendChangeset(
    [{ name: "@moh-sso/shell" }, { name: "@moh-sso/api" }],
    "minor",
  );
  assert.match(result, /"@moh-sso\/shell": minor/);
  assert.match(result, /"@moh-sso\/api": minor/);
});

test("real Changesets bumps every frontend module and honors stronger recorded changes", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-changesets-release-"));
  const frontend = new URL("../frontend/", import.meta.url);
  try {
    writeFileSync(
      join(root, "package.json"),
      readFileSync(new URL("package.json", frontend)),
    );
    writeFileSync(
      join(root, "package-lock.json"),
      readFileSync(new URL("package-lock.json", frontend)),
    );
    mkdirSync(join(root, ".changeset"));
    writeFileSync(
      join(root, ".changeset/config.json"),
      readFileSync(new URL(".changeset/config.json", frontend)),
    );
    const packages = [];
    for (const folder of ["apps", "packages"]) {
      for (const entry of readdirSync(new URL(`${folder}/`, frontend), {
        withFileTypes: true,
      })) {
        const source = new URL(
          `${folder}/${entry.name}/package.json`,
          frontend,
        );
        if (!entry.isDirectory() || !existsSync(source)) continue;
        const contents = readFileSync(source);
        const pkg = JSON.parse(contents);
        packages.push({ ...pkg, folder, dir: entry.name });
        mkdirSync(join(root, folder, entry.name), { recursive: true });
        writeFileSync(join(root, folder, entry.name, "package.json"), contents);
      }
    }
    writeFileSync(
      join(root, ".changeset/automatic.md"),
      frontendChangeset(packages, "patch"),
    );
    writeFileSync(
      join(root, ".changeset/stronger-api-change.md"),
      '---\n"@moh-sso/api": minor\n---\n\nCompatible new API feature.\n',
    );
    execFileSync(
      process.execPath,
      [
        new URL("node_modules/@changesets/cli/bin.js", frontend).pathname,
        "version",
      ],
      { cwd: root, stdio: "pipe" },
    );
    assert.ok(packages.some((pkg) => pkg.name === "@moh-sso/shell"));
    assert.ok(packages.some((pkg) => pkg.name === "@moh-sso/api"));
    const sharedVersion = nextBackendVersion(
      packages.find((pkg) => pkg.name === "@moh-sso/api").version,
      [],
      "minor",
    );
    for (const pkg of packages) {
      const updated = JSON.parse(
        readFileSync(join(root, pkg.folder, pkg.dir, "package.json"), "utf8"),
      );
      assert.equal(
        updated.version,
        pkg.folder === "packages"
          ? sharedVersion
          : nextBackendVersion(pkg.version, [], "patch"),
        pkg.name,
      );
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("automatic preparation commits versions and reuses a prepared release on retry", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-auto-release-"));
  try {
    for (const path of [
      "backend",
      "frontend/apps/shell",
      "frontend/packages/api",
      "frontend/.changeset",
    ])
      mkdirSync(join(root, path), { recursive: true });
    writeFileSync(join(root, "backend/VERSION"), "0.0.1\n");
    for (const [path, name] of [
      ["apps/shell", "shell"],
      ["packages/api", "api"],
    ]) {
      writeFileSync(
        join(root, "frontend", path, "package.json"),
        JSON.stringify({ name: `@moh-sso/${name}`, version: "0.1.0" }),
      );
    }
    const git = (...args) =>
      execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
    git("init", "-q");
    git("config", "user.name", "Release Test");
    git("config", "user.email", "release@example.invalid");
    git("config", "commit.gpgsign", "false");
    git("add", ".");
    git("commit", "-qm", "fixture");
    let versionCalls = 0;
    const run = (command, args, options) => {
      if (command === "git" && ["fetch", "remote"].includes(args[0])) return "";
      if (command === "mock-go") {
        assert.deepEqual(args, ["run", "./cmd/cli", "openapi", "generate"]);
        assert.equal(options.cwd, join(root, "backend"));
        const version = readFileSync(
          join(root, "backend/VERSION"),
          "utf8",
        ).trim();
        mkdirSync(join(root, "backend/docs"), { recursive: true });
        writeFileSync(
          join(root, "backend/docs/openapi.yaml"),
          `info:\n  version: ${version}\n`,
        );
        return "";
      }
      if (command === "mock-npm") {
        versionCalls++;
        const changeset = execFileSync(
          "find",
          [join(root, "frontend/.changeset"), "-name", "auto-release-*.md"],
          { encoding: "utf8" },
        ).trim();
        assert.match(
          readFileSync(changeset, "utf8"),
          /"@moh-sso\/shell": patch/,
        );
        for (const path of ["apps/shell", "packages/api"]) {
          const file = join(root, "frontend", path, "package.json");
          const pkg = JSON.parse(readFileSync(file, "utf8"));
          pkg.version = "0.1.1";
          writeFileSync(file, JSON.stringify(pkg));
        }
        writeFileSync(join(root, "frontend/package-lock.json"), "{}\n");
        mkdirSync(join(root, "frontend/public"));
        writeFileSync(
          join(root, "frontend/public/version-manifest.json"),
          "{}\n",
        );
        return "";
      }
      return execFileSync(command, args, options);
    };
    const config = { root: `${root}/`, npm: "mock-npm", go: "mock-go", run };
    const plan = prepareRelease(config);
    assert.equal(plan.backendVersion, "0.0.2");
    assert.equal(plan.frontendVersion, "0.1.1");
    assert.ok(
      existsSync(join(root, ".release/notes/frontend/apps/shell/0.1.1.md")),
    );
    assert.ok(
      existsSync(join(root, ".release/notes/frontend/packages/api/0.1.1.md")),
    );
    assert.ok(existsSync(join(root, "backend/CHANGELOG.md")));
    // The regenerated API contract carries the new version in the same commit.
    assert.equal(
      git("show", "HEAD:backend/docs/openapi.yaml"),
      "info:\n  version: 0.0.2",
    );
    assert.equal(git("status", "--porcelain"), "");
    assert.match(
      git("log", "-1", "--format=%s"),
      /Prepare release: backend\/v0.0.2, frontend\/v0.1.1/,
    );
    assert.deepEqual(prepareRelease(config), plan);
    assert.equal(versionCalls, 1);
    assert.equal(git("tag", "--list"), "");
    git("tag", "backend/v0.0.2");
    assert.throws(
      () => prepareRelease(config),
      /Prepared release tags already exist/,
    );
    writeFileSync(join(root, "dirty.txt"), "uncommitted");
    assert.throws(() => prepareRelease(config), /Commit or stash/);
    assert.ok(existsSync(join(root, ".release/plan.json")));
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("a merged prepare PR is published as prepared, never bumped past", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-auto-release-merge-"));
  try {
    for (const path of [
      "backend",
      "frontend/apps/shell",
      "frontend/.changeset",
    ])
      mkdirSync(join(root, path), { recursive: true });
    writeFileSync(join(root, "backend/VERSION"), "1.5.1\n");
    writeFileSync(
      join(root, "frontend/apps/shell/package.json"),
      JSON.stringify({ name: "@moh-sso/shell", version: "1.5.1" }),
    );
    const git = (...args) =>
      execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
    git("init", "-q", "-b", "main");
    git("config", "user.name", "Release Test");
    git("config", "user.email", "release@example.invalid");
    git("config", "commit.gpgsign", "false");
    git("add", ".");
    git("commit", "-qm", "fixture");
    git("tag", "backend/v1.5.1");
    const run = (command, args, options) => {
      if (command === "git" && ["fetch", "remote"].includes(args[0])) return "";
      if (command === "mock-go") {
        mkdirSync(join(root, "backend/docs"), { recursive: true });
        writeFileSync(join(root, "backend/docs/openapi.yaml"), "info: {}\n");
        return "";
      }
      return execFileSync(command, args, options);
    };
    const config = {
      root: `${root}/`,
      component: "backend",
      bump: "minor",
      go: "mock-go",
      run,
    };

    // Prepare on a branch, then merge it like a pull request (a merge commit).
    git("switch", "-qc", "release");
    const plan = prepareRelease(config);
    assert.equal(plan.backendVersion, "1.6.0");
    git("switch", "-q", "main");
    git(
      "merge",
      "-q",
      "--no-ff",
      "-m",
      "Merge pull request: prepare 1.6.0",
      "release",
    );
    assert.notEqual(
      git("log", "-1", "--format=%H", "--", ".release/plan.json"),
      git("rev-parse", "HEAD"),
    );

    // Releasing from the merge commit reuses the plan instead of preparing 1.7.0.
    assert.deepEqual(prepareRelease(config), plan);
    assert.equal(
      readFileSync(join(root, "backend/VERSION"), "utf8"),
      "1.6.0\n",
    );

    // Once anything else lands, the untagged 1.6.0 is not silently skipped.
    writeFileSync(join(root, "later.txt"), "a later change");
    git("add", "later.txt");
    git("commit", "-qm", "later change");
    assert.throws(
      () => prepareRelease(config),
      /backend\/v1\.6\.0 was prepared .* but never published.*BACKEND_VERSION=1\.6\.0/,
    );
    // A different BUMP does not bypass it either.
    assert.throws(
      () => prepareRelease({ ...config, bump: "patch" }),
      /never published/,
    );

    // After 1.6.0 is published the next release prepares normally.
    git("tag", "backend/v1.6.0");
    assert.equal(prepareRelease(config).backendVersion, "1.7.0");
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
