import { readFileSync } from "node:fs";
import { join } from "node:path";
import { finish, listApps, listPackages, rel, root, sourceFiles } from "./workspace-lib.mjs";

const failures = [];
const oldAliasPattern = /@\/(features|store|shared|lib|ui|utils)\b/;
const shellImportPattern = /@\/app\b|apps\/shell\b|\.\.\/\.\.\/shell\b/;
const crossAppInternalPattern = /apps\/(announcements|audit|clients|data-validation|data-visualizer|documents|e-services|email|issue-tracker|report-browser|report-scheduler|surveillance|users|utilities)\/src\b/;
const roots = [
  ...listApps().map((app) => join(root, "apps", app, "src")),
  ...listPackages().map((pkg) => join(root, "packages", pkg, "src")),
];

for (const file of sourceFiles(roots)) {
  const source = readFileSync(file, "utf8");
  const relativePath = rel(file);
  if (oldAliasPattern.test(source)) {
    failures.push(`${relativePath} imports through retired @/ compatibility aliases`);
  }

  if (relativePath.startsWith("packages/") && shellImportPattern.test(source)) {
    failures.push(`${relativePath} package source imports shell/app code`);
  }

  if (relativePath.startsWith("apps/") && !relativePath.startsWith("apps/shell/") && shellImportPattern.test(source)) {
    failures.push(`${relativePath} feature app imports shell/app code`);
  }

  if (crossAppInternalPattern.test(source)) {
    failures.push(`${relativePath} imports another app's internal src path`);
  }
}

finish("Frontend architecture check", failures);
