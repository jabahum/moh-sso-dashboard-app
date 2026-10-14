import { join } from "node:path";
import { isDeepStrictEqual } from "node:util";
import semver from "semver";
import {
  check,
  fileExists,
  finish,
  listApps,
  listPackages,
  readJson,
  root,
} from "./workspace-lib.mjs";

const failures = [];
const modules = new Map();
for (const [folder, names] of [
  ["apps", listApps()],
  ["packages", listPackages()],
]) {
  for (const name of names) {
    const path = `${folder}/${name}/package.json`;
    const pkg = readJson(join(root, path));
    check(pkg.name === `@moh-sso/${name}`, `${path} has unexpected package name`, failures);
    check(
      Boolean(semver.valid(pkg.version)),
      `${path} has invalid version ${pkg.version ?? ""}`,
      failures,
    );
    modules.set(pkg.name, { pkg, folder, path });
    if (name === "shell" && folder === "apps") {
      check(pkg.private === true, `${path} should remain private`, failures);
      continue;
    }
    check(pkg.private === false, `${path} should have private=false for npm deployment`, failures);
    check(
      Array.isArray(pkg.files) && pkg.files.includes("dist"),
      `${path} should publish dist`,
      failures,
    );
    check(
      Boolean(pkg.publishConfig?.access),
      `${path} should define publishConfig.access`,
      failures,
    );
  }
}

for (const { pkg, path } of modules.values()) {
  for (const group of [
    "dependencies",
    "devDependencies",
    "peerDependencies",
    "optionalDependencies",
  ]) {
    for (const [name, range] of Object.entries(pkg[group] ?? {})) {
      if (!name.startsWith("@moh-sso/")) continue;
      const target = modules.get(name);
      check(Boolean(target), `${path}: unknown internal dependency ${name}`, failures);
      if (!target) continue;
      check(
        Boolean(semver.validRange(range)) && semver.satisfies(target.pkg.version, range),
        `${path}: ${name}@${range} does not accept workspace version ${target.pkg.version}`,
        failures,
      );
    }
  }
}

const changesetPath = join(root, ".changeset", "config.json");
check(fileExists(changesetPath), ".changeset/config.json is missing", failures);
if (fileExists(changesetPath)) {
  const config = readJson(changesetPath);
  const sharedNames = [...modules]
    .filter(([, entry]) => entry.folder === "packages")
    .map(([name]) => name);
  const fixed = config.fixed?.find((group) => sharedNames.every((name) => group.includes(name)));
  check(
    Boolean(fixed),
    "All shared packages must belong to the same Changesets fixed group",
    failures,
  );
  for (const group of config.fixed ?? []) {
    const versions = new Set();
    for (const name of group) {
      const entry = modules.get(name);
      check(Boolean(entry), `Changesets fixed group references unknown module ${name}`, failures);
      if (entry) versions.add(entry.pkg.version);
    }
    check(
      versions.size === 1,
      `Changesets fixed group has mismatched versions: ${group.join(", ")}`,
      failures,
    );
  }
}

const lockPath = join(root, "package-lock.json");
check(fileExists(lockPath), "package-lock.json is missing", failures);
if (fileExists(lockPath)) {
  const lock = readJson(lockPath);
  for (const { pkg, path } of modules.values()) {
    const entry = lock.packages?.[path.replace(/\/package\.json$/, "")];
    check(entry?.version === pkg.version, `${path}: lockfile version is stale`, failures);
    for (const group of [
      "dependencies",
      "devDependencies",
      "peerDependencies",
      "optionalDependencies",
    ]) {
      check(
        isDeepStrictEqual(entry?.[group] ?? {}, pkg[group] ?? {}),
        `${path}: lockfile ${group} are stale`,
        failures,
      );
    }
  }
}

const manifestPath = join(root, "public", "version-manifest.json");
check(
  fileExists(manifestPath),
  "public/version-manifest.json is missing; run npm run generate:versions",
  failures,
);
if (fileExists(manifestPath)) {
  const manifest = readJson(manifestPath);
  for (const { pkg, folder, path } of modules.values()) {
    const name = path.split("/")[1];
    const entry = manifest[folder]?.[name];
    check(
      entry?.name === pkg.name && entry?.version === pkg.version,
      `${path}: version manifest is stale; run npm run generate:versions`,
      failures,
    );
  }
  for (const folder of ["apps", "packages"]) {
    const expected = [...modules.values()].filter((entry) => entry.folder === folder).length;
    check(
      Object.keys(manifest[folder] ?? {}).length === expected,
      `${folder}: version manifest module count is stale`,
      failures,
    );
  }
}
finish("Version tooling check", failures);
