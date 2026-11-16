/* global process, URL */
import { existsSync, readFileSync, realpathSync, readdirSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import semver from "semver";

export const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
export function workspaceModules(root = frontendRoot) {
  return ["apps", "packages"].flatMap((group) =>
    readdirSync(join(root, group)).flatMap((name) => {
      const path = join(root, group, name, "package.json");
      if (!existsSync(path)) return [];
      const pkg = JSON.parse(readFileSync(path, "utf8"));
      return pkg.private ? [] : [{ group, name, pkg }];
    }),
  );
}
export function moduleSelection(
  environment = "production",
  env = process.env,
  root = frontendRoot,
) {
  environment = env.FRONTEND_MODULE_ENV || environment;
  if (!["development", "production"].includes(environment))
    throw new Error(`Invalid frontend module environment: ${environment}`);
  const configPath = resolve(root, env.FRONTEND_MODULE_CONFIG || "module-sources.json");
  const config = JSON.parse(readFileSync(configPath, "utf8"));
  const entry = config[environment];
  const source = env.FRONTEND_MODULE_SOURCE || entry?.source;
  if (!["workspace", "npm"].includes(source))
    throw new Error(`Invalid frontend module source for ${environment}: ${source}`);
  const modules = workspaceModules(root);
  const versions =
    source === "npm"
      ? entry.versions
      : Object.fromEntries(modules.map(({ pkg }) => [pkg.name, pkg.version]));
  if (!versions || typeof versions !== "object" || Array.isArray(versions))
    throw new Error(`Missing npm version pins for ${environment}`);
  const expected = new Set(modules.map(({ pkg }) => pkg.name));
  if (
    Object.keys(versions).length !== expected.size ||
    Object.keys(versions).some((name) => !expected.has(name))
  )
    throw new Error(
      `Version pins must include exactly all ${expected.size} publishable frontend modules`,
    );
  for (const name of expected) {
    if (typeof versions[name] !== "string" || semver.valid(versions[name]) !== versions[name])
      throw new Error(`Pin ${name} to an exact semantic version; received ${versions[name]}`);
  }
  return {
    environment,
    source,
    versions,
    modules,
    directory: resolve(root, env.FRONTEND_MODULE_DIRECTORY || join(".npm-modules", environment)),
  };
}
export function installedModules(selection, directory = selection.directory) {
  return selection.modules.map((module) => {
    const path = join(directory, "node_modules", module.pkg.name);
    const metadata = join(path, "package.json");
    if (!existsSync(metadata))
      throw new Error(
        `Missing ${module.pkg.name}; run npm run modules:prepare -- ${selection.environment}`,
      );
    // npm workspace links must never silently substitute unreleased source.
    if (relative(realpathSync(directory), realpathSync(path)).startsWith(".."))
      throw new Error(`Expected an isolated npm installation for ${module.pkg.name}`);
    const pkg = JSON.parse(readFileSync(metadata, "utf8"));
    if (pkg.name !== module.pkg.name || pkg.version !== selection.versions[pkg.name])
      throw new Error(
        `Installed version mismatch for ${module.pkg.name}; run npm run modules:prepare -- ${selection.environment}`,
      );
    for (const [name, range] of Object.entries({
      ...pkg.dependencies,
      ...pkg.optionalDependencies,
      ...pkg.peerDependencies,
    })) {
      if (selection.versions[name] && !semver.satisfies(selection.versions[name], range))
        throw new Error(
          `${pkg.name} requires ${name}@${range}, incompatible with pin ${selection.versions[name]}`,
        );
    }
    for (const entry of Object.values(pkg.exports || {})) {
      const target = typeof entry === "string" ? entry : entry.import;
      if (target && !existsSync(join(path, target)))
        throw new Error(`Missing npm artifact ${pkg.name}/${target}`);
    }
    return { ...module, pkg, path };
  });
}
// Exact public export resolution also prevents fallback to workspace symlinks.
export function npmModulePlugin(selection) {
  const installed = new Map(installedModules(selection).map((module) => [module.pkg.name, module]));
  let shellEntry;
  return {
    name: "moh-published-modules",
    enforce: "pre",
    configResolved(config) {
      shellEntry = join(config.root, "src/main.tsx");
    },
    transform(code, id) {
      if (id.split("?")[0] !== shellEntry) return null;
      // Source modules bring their SCSS imports with them. Published UI code
      // extracts those styles into its public CSS export instead.
      return { code: `import "@moh-sso/ui/styles.css";\n${code}`, map: null };
    },
    resolveId(id) {
      if (!id.startsWith("@moh-sso/")) return null;
      const [scope, name, ...parts] = id.split("/");
      const module = installed.get(`${scope}/${name}`);
      const entry = module?.pkg.exports?.[parts.length ? `./${parts.join("/")}` : "."];
      const target = typeof entry === "string" ? entry : entry?.import;
      if (!target) throw new Error(`Published module does not export ${id}`);
      return join(module.path, target);
    },
  };
}
