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
    assert.equal(packages.length, 23);
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
    const config = { root, npm: "mock-npm", run };
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
