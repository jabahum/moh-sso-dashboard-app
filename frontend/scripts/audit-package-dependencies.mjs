/* global console, process */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const sourceRoots = ["apps", "packages"];
const ignoredSpecifiers = new Set(["react/jsx-runtime"]);
const ignoredPackages = new Set(["node:fs", "node:path", "node:url"]);

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function walkFiles(dir, files = []) {
  if (!existsSync(dir)) {
    return files;
  }

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);

    if (entry.isDirectory()) {
      if (!["dist", "node_modules"].includes(entry.name)) {
        walkFiles(path, files);
      }
      continue;
    }

    if (/\.(tsx?|jsx?)$/.test(entry.name)) {
      files.push(path);
    }
  }

  return files;
}

function packageNameFromSpecifier(specifier) {
  if (specifier === "geojson") {
    return "@types/geojson";
  }

  if (specifier.startsWith("@")) {
    return specifier.split("/").slice(0, 2).join("/");
  }

  return specifier.split("/")[0];
}

function collectImports(sourceDir) {
  const imports = new Set();
  const importPattern = /(?:from\s+["']([^"']+)["']|import\(["']([^"']+)["']\))/g;

  for (const file of walkFiles(sourceDir)) {
    const source = readFileSync(file, "utf8");

    for (const match of source.matchAll(importPattern)) {
      const specifier = match[1] ?? match[2];

      if (
        !specifier ||
        specifier.startsWith(".") ||
        specifier.startsWith("@/") ||
        specifier.startsWith("virtual:") ||
        ignoredSpecifiers.has(specifier)
      ) {
        continue;
      }

      const packageName = packageNameFromSpecifier(specifier);

      if (!ignoredPackages.has(packageName)) {
        imports.add(packageName);
      }
    }
  }

  return imports;
}

function declaredPackages(packageJson) {
  return new Set([
    ...Object.keys(packageJson.dependencies ?? {}),
    ...Object.keys(packageJson.peerDependencies ?? {}),
    ...Object.keys(packageJson.devDependencies ?? {}),
  ]);
}

const failures = [];

for (const sourceRoot of sourceRoots) {
  const rootPath = join(root, sourceRoot);

  if (!existsSync(rootPath)) {
    continue;
  }

  for (const entry of readdirSync(rootPath, { withFileTypes: true })) {
    if (!entry.isDirectory()) {
      continue;
    }

    const packagePath = join(rootPath, entry.name, "package.json");
    const sourcePath = join(rootPath, entry.name, "src");

    if (!existsSync(packagePath) || !existsSync(sourcePath)) {
      continue;
    }

    const packageJson = readJson(packagePath);
    const declared = declaredPackages(packageJson);
    const imports = collectImports(sourcePath);
    const missing = [...imports]
      .filter((packageName) => packageName !== packageJson.name && !declared.has(packageName))
      .sort();

    if (missing.length > 0) {
      failures.push({
        path: packagePath.replace(`${root}/`, ""),
        name: packageJson.name,
        missing,
      });
    }
  }
}

if (failures.length > 0) {
  console.error("Missing package dependency declarations:");

  for (const failure of failures) {
    console.error(`\n${failure.path} (${failure.name})`);
    for (const missing of failure.missing) {
      console.error(`  - ${missing}`);
    }
  }

  process.exit(1);
}

console.log("All app/package dependency declarations are complete.");
