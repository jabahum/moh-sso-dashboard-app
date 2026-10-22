/* global console, process */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { createRequire, isBuiltin } from "node:module";
import ts from "typescript";

const sourceRoots = ["apps", "packages"];
const ignoredSpecifiers = new Set(["react/jsx-runtime"]);

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

  for (const file of walkFiles(sourceDir)) {
    const source = readFileSync(file, "utf8");

    for (const specifier of sourceImports(source, file)) {
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

      if (!isBuiltin(packageName)) imports.add(specifier);
    }
  }

  return imports;
}

export function sourceImports(source, filename = "source.tsx") {
  const result = new Set();
  const tree = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    const value =
      ts.isImportDeclaration(node) || ts.isExportDeclaration(node)
        ? node.moduleSpecifier
        : ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword
          ? node.arguments[0]
          : undefined;
    if (value && ts.isStringLiteralLike(value)) result.add(value.text);
    ts.forEachChild(node, visit);
  }
  visit(tree);
  return result;
}

function declaredPackages(packageJson) {
  return new Set([
    ...Object.keys(packageJson.dependencies ?? {}),
    ...Object.keys(packageJson.peerDependencies ?? {}),
    ...Object.keys(packageJson.devDependencies ?? {}),
  ]);
}

export function auditDependencies(root) {
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
      const missing = [...new Set([...imports].map(packageNameFromSpecifier))]
        .filter((packageName) => packageName !== packageJson.name && !declared.has(packageName))
        .sort();

      if (missing.length > 0) {
        failures.push({
          path: packagePath.replace(`${root}/`, ""),
          name: packageJson.name,
          missing,
        });
      }
      const require = createRequire(packagePath);
      for (const specifier of imports) {
        if (!/\.(?:css|scss)(?:\?|$)/.test(specifier)) continue;
        try {
          require.resolve(specifier.split("?")[0]);
        } catch {
          failures.push({
            path: packagePath.replace(`${root}/`, ""),
            name: packageJson.name,
            missing: [`Unresolvable stylesheet: ${specifier}`],
          });
        }
      }
    }
  }

  return failures;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const failures = auditDependencies(process.cwd());
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
}
