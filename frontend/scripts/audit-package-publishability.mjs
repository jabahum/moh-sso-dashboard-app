/* global console, process */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const appsDir = join(root, "apps");
const packagesDir = join(root, "packages");
const failures = [];

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function workspaceEntries(dir, ignored = new Set()) {
  return readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && !ignored.has(entry.name) && existsSync(join(dir, entry.name, "package.json")))
    .map((entry) => entry.name)
    .sort();
}

function assert(condition, path, message) {
  if (!condition) {
    failures.push(`${path}: ${message}`);
  }
}

function hasDistExport(pkg, key, file) {
  const exportEntry = pkg.exports?.[key];
  return exportEntry?.types === `./dist/${file}.d.ts` && exportEntry?.import === `./dist/${file}.js`;
}

function auditPackageJson(kind, name) {
  const baseDir = join(root, kind === "app" ? "apps" : "packages", name);
  const relativePath = `${kind === "app" ? "apps" : "packages"}/${name}/package.json`;
  const pkg = readJson(join(baseDir, "package.json"));

  assert(pkg.name?.startsWith("@moh-sso/"), relativePath, "name must start with @moh-sso/");
  assert(Boolean(pkg.version), relativePath, "version is required");
  assert(pkg.private === false || pkg.private === undefined, relativePath, "private must be false or absent");
  assert(pkg.main === "./dist/index.js", relativePath, "main must point to ./dist/index.js");
  assert(pkg.types === "./dist/index.d.ts", relativePath, "types must point to ./dist/index.d.ts");
  assert(hasDistExport(pkg, ".", "index"), relativePath, "exports[\".\"] must point to dist index files");
  assert(Array.isArray(pkg.files) && pkg.files.includes("dist"), relativePath, "files must include dist");
  assert(!pkg.files?.includes("src"), relativePath, "files must not include src");
  assert(JSON.stringify(pkg.exports ?? {}).includes("./src") === false, relativePath, "exports must not point to src");

  assert(existsSync(join(baseDir, "dist", "index.js")), relativePath, "dist/index.js is missing; run build first");
  assert(existsSync(join(baseDir, "dist", "index.d.ts")), relativePath, "dist/index.d.ts is missing; run build first");

  if (kind === "app") {
    assert(hasDistExport(pkg, "./single-spa", "single-spa"), relativePath, "apps must export ./single-spa from dist");
    assert(hasDistExport(pkg, "./routes", "routes"), relativePath, "apps must export ./routes from dist");
    assert(existsSync(join(baseDir, "dist", "single-spa.js")), relativePath, "dist/single-spa.js is missing");
    assert(existsSync(join(baseDir, "dist", "single-spa.d.ts")), relativePath, "dist/single-spa.d.ts is missing");
    assert(existsSync(join(baseDir, "dist", "routes.js")), relativePath, "dist/routes.js is missing");
    assert(existsSync(join(baseDir, "dist", "routes.d.ts")), relativePath, "dist/routes.d.ts is missing");

    for (const peer of ["react", "react-dom", "react-redux", "single-spa"]) {
      assert(Boolean(pkg.peerDependencies?.[peer]), relativePath, `apps must declare ${peer} as a peer dependency`);
    }
  }
}

for (const app of workspaceEntries(appsDir, new Set(["shell"]))) {
  auditPackageJson("app", app);
}

for (const packageName of workspaceEntries(packagesDir)) {
  auditPackageJson("package", packageName);
}

if (failures.length > 0) {
  console.error("Package publishability audit failed:");
  for (const failure of failures) {
    console.error(`  - ${failure}`);
  }
  process.exit(1);
}

console.log("All publishable frontend apps/packages are npm-ready.");
