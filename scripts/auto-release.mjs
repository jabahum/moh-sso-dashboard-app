import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import { createRequire } from "node:module";
import { delimiter, dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { releaseTags } from "./release.mjs";
import { generateReleaseNotes } from "./release-notes.mjs";

const projectRoot = fileURLToPath(new URL("..", import.meta.url));
const require = createRequire(
  new URL("../frontend/package.json", import.meta.url),
);
const semver = require("semver");

export function nextBackendVersion(current, tags, bump) {
  const versions = [
    current,
    ...tags
      .filter((tag) => tag.startsWith("backend/v"))
      .map((tag) => tag.slice(9)),
  ];
  if (!semver.valid(current))
    throw new Error("backend/VERSION must contain a semantic version");
  const latest = versions
    .filter((value) => semver.valid(value))
    .sort(semver.rcompare)[0];
  return semver.inc(latest, bump);
}

export function frontendChangeset(packages, bump) {
  return `---\n${packages.map((pkg) => `${JSON.stringify(pkg.name)}: ${bump}`).join("\n")}\n---\n\nAutomatic ${bump} release of all frontend apps and shared packages.\n`;
}

export function prepareRelease({
  root = projectRoot,
  component = "both",
  bump = "patch",
  remote = "upstream",
  npm = "npm",
  run = execFileSync,
}) {
  if (!["both", "backend", "frontend"].includes(component))
    throw new Error("Invalid release component");
  if (!["patch", "minor", "major"].includes(bump))
    throw new Error("BUMP must be patch, minor, or major");
  if (!remote || remote.startsWith("-"))
    throw new Error("Invalid release remote");
  const git = (...args) =>
    run("git", args, { cwd: root, encoding: "utf8" }).trim();
  if (git("status", "--porcelain"))
    throw new Error(
      "Commit or stash application changes before preparing an automatic release",
    );
  git("remote", "get-url", remote);
  // Refresh scoped tags before calculating versions, so another release cannot
  // silently leave this checkout's next version calculation behind.
  git("fetch", remote, "--tags");
  const tags = git("tag", "--list").split("\n");
  const sourceCommit = git("rev-parse", "HEAD");
  const planPath = join(root, ".release", "plan.json");
  const read = (path) => JSON.parse(readFileSync(path, "utf8"));
  const shellPath = join(root, "frontend/apps/shell/package.json");
  const backendPath = join(root, "backend/VERSION");

  if (existsSync(planPath)) {
    const plan = read(planPath);
    const planCommit = git(
      "log",
      "-1",
      "--format=%H",
      "--",
      ".release/plan.json",
    );
    if (
      planCommit === sourceCommit &&
      plan.component === component &&
      plan.bump === bump
    ) {
      const selectedTags = releaseTags(
        component,
        plan.backendVersion,
        plan.frontendVersion,
        read(shellPath).version,
      );
      if (selectedTags.every((tag) => !tags.includes(tag))) {
        if (
          component !== "frontend" &&
          readFileSync(backendPath, "utf8").trim() !== plan.backendVersion
        )
          throw new Error("Prepared backend version has drifted");
        console.log(`Reusing prepared release: ${selectedTags.join(", ")}`);
        return plan;
      }
      throw new Error(
        "Prepared release tags already exist; inspect local/remote tags and retry the pending atomic push or CI run instead of bumping versions again",
      );
    }
  }

  // Check Git author identity before changing any version files.
  git("var", "GIT_AUTHOR_IDENT");
  if (component !== "backend") {
    const published = tags
      .filter((tag) => tag.startsWith("frontend/v"))
      .map((tag) => tag.slice(10))
      .filter((value) => semver.valid(value))
      .sort(semver.rcompare)[0];
    if (published && semver.gt(published, read(shellPath).version))
      throw new Error(
        "Frontend versions are behind a published release; update the checkout before preparing versions",
      );
  }
  const plan = {
    component,
    bump,
    sourceCommit,
    backendVersion: readFileSync(backendPath, "utf8").trim(),
    frontendVersion: read(shellPath).version,
  };
  const paths = [".release/plan.json"];
  if (component !== "frontend") {
    plan.backendVersion = nextBackendVersion(plan.backendVersion, tags, bump);
    writeFileSync(backendPath, `${plan.backendVersion}\n`);
    paths.push("backend/VERSION");
  }
  if (component !== "backend") {
    const packages = [];
    for (const folder of ["apps", "packages"]) {
      for (const entry of readdirSync(join(root, "frontend", folder), {
        withFileTypes: true,
      })) {
        const path = join(root, "frontend", folder, entry.name, "package.json");
        if (entry.isDirectory() && existsSync(path)) packages.push(read(path));
      }
    }
    if (!packages.length) throw new Error("No frontend modules found");
    const changeset = join(
      root,
      "frontend/.changeset",
      `auto-release-${sourceCommit.slice(0, 12)}.md`,
    );
    if (existsSync(changeset))
      throw new Error(
        "Automatic changeset already exists; inspect the unfinished preparation",
      );
    writeFileSync(changeset, frontendChangeset(packages, bump));
    run(npm, ["run", "version:packages"], {
      cwd: join(root, "frontend"),
      stdio: "inherit",
    });
    plan.frontendVersion = read(shellPath).version;
    paths.push(
      "frontend/apps",
      "frontend/packages",
      "frontend/.changeset",
      "frontend/package-lock.json",
      "frontend/public/version-manifest.json",
    );
  }
  const selectedTags = releaseTags(
    component,
    plan.backendVersion,
    plan.frontendVersion,
    read(shellPath).version,
  );
  if (selectedTags.some((tag) => tags.includes(tag)))
    throw new Error(
      "Calculated release tag already exists; inspect version preparation",
    );
  mkdirSync(join(root, ".release"), { recursive: true });
  const notes = generateReleaseNotes({
    root,
    component,
    backendVersion: plan.backendVersion,
    sourceCommit,
    run,
  });
  paths.push(...notes.files.map((path) => relative(root, path)));
  writeFileSync(planPath, `${JSON.stringify(plan, null, 2)}\n`);
  git("add", "--", ...paths);
  git("commit", "-m", `Prepare release: ${selectedTags.join(", ")}`);
  console.log(`Prepared and committed ${selectedTags.join(", ")}`);
  return plan;
}

function main() {
  if (Number(process.versions.node.split(".")[0]) < 22)
    throw new Error(
      "Automatic release requires Node.js 22 or newer; select it before running Make",
    );
  process.env.PATH = `${dirname(process.execPath)}${delimiter}${process.env.PATH ?? ""}`;
  const [
    action = "prepare",
    component = "both",
    bump = "patch",
    remote = "upstream",
    npm = "npm",
    make = "make",
  ] = process.argv.slice(2);
  if (!["prepare", "release"].includes(action))
    throw new Error("Action must be prepare or release");
  const plan = prepareRelease({ component, bump, remote, npm });
  if (action === "prepare") return;
  const target =
    component === "both" ? "release-manual" : `release-${component}-manual`;
  execFileSync(
    make,
    [
      target,
      `BACKEND_VERSION=${plan.backendVersion}`,
      `FRONTEND_VERSION=${plan.frontendVersion}`,
      `RELEASE_REMOTE=${remote}`,
      `NPM=${npm}`,
      `NODE=${process.execPath}`,
    ],
    { cwd: projectRoot, stdio: "inherit" },
  );
}

if (
  process.argv[1] &&
  realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
