/* global console, process, URL, fetch, setTimeout, AbortSignal */
import { existsSync, readdirSync, readFileSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const frontend = fileURLToPath(new URL("../", import.meta.url));
export function releaseModules(root = frontend) {
  const shell = JSON.parse(readFileSync(join(root, "apps/shell/package.json"), "utf8"));
  if (shell.private !== true) throw new Error("The shell must remain private");
  const modules = ["apps", "packages"].flatMap((group) =>
    readdirSync(join(root, group)).flatMap((name) => {
      const path = join(root, group, name, "package.json");
      if (!existsSync(path) || (group === "apps" && name === "shell")) return [];
      const pkg = JSON.parse(readFileSync(path, "utf8"));
      if (pkg.private !== false || pkg.publishConfig?.access !== "public")
        throw new Error(`${pkg.name} must be publishable with public access`);
      if (
        pkg.repository?.url !== "git+https://github.com/mohuganda/moh-sso-dashboard.git" ||
        pkg.repository.directory !== `frontend/${group}/${name}`
      )
        throw new Error(
          `${pkg.name} must identify its upstream repository and workspace directory`,
        );
      if (
        pkg.publishConfig.registry &&
        pkg.publishConfig.registry.replace(/\/$/, "") !== "https://registry.npmjs.org"
      )
        throw new Error(`${pkg.name} must publish to npmjs.org`);
      return [{ name: pkg.name, version: pkg.version }];
    }),
  );
  if (!modules.length) throw new Error("No frontend modules to release");
  return modules;
}
export async function verifyNpmRelease(
  modules,
  {
    lookup = (module) =>
      fetch(
        `https://registry.npmjs.org/${encodeURIComponent(module.name)}/${encodeURIComponent(module.version)}`,
        { signal: AbortSignal.timeout(15000), headers: { Accept: "application/json" } },
      ),
    attempts = 4,
    wait = () => new Promise((resolve) => setTimeout(resolve, 2000)),
  } = {},
) {
  let pending = modules;
  for (let attempt = 0; attempt < attempts; attempt++) {
    const missing = [];
    for (const module of pending) {
      const response = await lookup(module);
      if (response.status === 404 || response.status >= 500) {
        missing.push(module);
        continue;
      }
      if (!response.ok)
        throw new Error(
          `npm lookup failed for ${module.name}@${module.version}: HTTP ${response.status}`,
        );
      const published = await response.json();
      if (
        published.name !== module.name ||
        published.version !== module.version ||
        !published.dist?.tarball
      )
        throw new Error(`Unexpected npm metadata for ${module.name}@${module.version}`);
    }
    if (!missing.length) return;
    pending = missing;
    if (attempt + 1 < attempts) await wait();
  }
  throw new Error(
    `Frontend release incomplete; missing npm versions: ${pending.map((module) => `${module.name}@${module.version}`).join(", ")}`,
  );
}
if (process.argv[1] && realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const modules = releaseModules();
    if (process.argv[2] === "--check")
      console.log(`Validated ${modules.length} public npm modules; shell excluded.`);
    else {
      await verifyNpmRelease(modules);
      console.log(`Confirmed all ${modules.length} frontend module versions on npm.`);
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
