/* global console, process */
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

export const root = process.cwd();

export function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

export function listDirs(path) {
  if (!existsSync(path)) {
    return [];
  }
  return readdirSync(path, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort((a, b) => a.localeCompare(b));
}

export function listApps() {
  return listDirs(join(root, "apps")).filter((name) =>
    fileExists(join(root, "apps", name, "package.json")),
  );
}

export function listStandaloneApps() {
  return listApps().filter((app) => app !== "shell");
}

export function listPackages() {
  return listDirs(join(root, "packages")).filter((name) =>
    fileExists(join(root, "packages", name, "package.json")),
  );
}

export function walkFiles(path, predicate = () => true, files = []) {
  if (!existsSync(path)) {
    return files;
  }

  for (const entry of readdirSync(path, { withFileTypes: true })) {
    const fullPath = join(path, entry.name);
    if (entry.isDirectory()) {
      if (["node_modules", "dist", ".git", ".vite", "coverage"].includes(entry.name)) {
        continue;
      }
      walkFiles(fullPath, predicate, files);
      continue;
    }

    if (entry.isFile() && predicate(fullPath)) {
      files.push(fullPath);
    }
  }
  return files;
}

export function sourceFiles(paths) {
  return paths.flatMap((path) =>
    walkFiles(path, (file) => /\.(ts|tsx|js|jsx|mjs|css|scss)$/.test(file)),
  );
}

export function rel(path) {
  return relative(root, path);
}

export function fileExists(path) {
  return existsSync(path) && statSync(path).isFile();
}

export function dirExists(path) {
  return existsSync(path) && statSync(path).isDirectory();
}

export function check(condition, message, failures) {
  if (!condition) {
    failures.push(message);
  }
}

export function finish(label, failures) {
  if (failures.length > 0) {
    console.error(`${label} failed:`);
    for (const failure of failures) {
      console.error(`- ${failure}`);
    }
    process.exit(1);
  }
  console.log(`${label} passed.`);
}
