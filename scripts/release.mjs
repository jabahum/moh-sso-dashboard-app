import { execFileSync } from "node:child_process";
import { readFileSync, realpathSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const git = (...args) =>
  execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();

export function releaseTags(
  component,
  backendVersion,
  frontendVersion,
  shellVersion,
) {
  if (!["both", "backend", "frontend"].includes(component))
    throw new Error("Component must be both, backend, or frontend");
  const tags = [];
  for (const [name, value] of [
    ["backend", backendVersion],
    ["frontend", frontendVersion],
  ]) {
    if (component !== "both" && component !== name) continue;
    // SemVer without build metadata: these values also become Docker tags.
    if (
      !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?$/.test(
        value,
      )
    ) {
      throw new Error(
        `${name.toUpperCase()}_VERSION must be a semantic version without a prefix or build metadata`,
      );
    }
    if (name === "frontend" && value !== shellVersion)
      throw new Error(
        `FRONTEND_VERSION ${value} does not match shell version ${shellVersion}; run make frontend-version and commit the result first`,
      );
    tags.push(`${name}/v${value}`);
  }
  return tags;
}

function main() {
  const [action, component, backendVersion, frontendVersion, remote] =
    process.argv.slice(2);
  if (
    !["check", "publish"].includes(action) ||
    !remote ||
    remote.startsWith("-")
  )
    throw new Error(
      "Usage: release.mjs check|publish both|backend|frontend BACKEND_VERSION FRONTEND_VERSION REMOTE",
    );
  const shell = JSON.parse(
    readFileSync(
      new URL("../frontend/apps/shell/package.json", import.meta.url),
      "utf8",
    ),
  );
  const tags = releaseTags(
    component,
    backendVersion,
    frontendVersion,
    shell.version,
  );
  if (git("status", "--porcelain"))
    throw new Error(
      "Release requires a clean worktree; commit version bumps and changes first",
    );
  const existing = new Set(git("tag", "--list").split("\n"));
  for (const tag of tags)
    if (existing.has(tag))
      throw new Error(`Release tag already exists locally: ${tag}`);
  if (action === "check") {
    console.log(`Release preflight passed: ${tags.join(", ")}`);
    return;
  }
  // Inspect all remote tags before creating any local tag. A combined release
  // pushes both refs atomically so a rejected push cannot publish only one.
  git("remote", "get-url", remote);
  const refs = tags.map((tag) => `refs/tags/${tag}`);
  if (git("ls-remote", "--tags", remote, ...refs))
    throw new Error("One or more release tags already exist on the remote");
  for (const tag of tags) git("tag", "-a", tag, "-m", `Release ${tag}`);
  execFileSync("git", ["push", "--atomic", remote, ...refs], {
    cwd: root,
    stdio: "inherit",
  });
  console.log(
    `Published ${tags.join(", ")} to ${remote}. Check Build & Push Images and Backend Release in GitHub Actions.`,
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
