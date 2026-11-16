/* global process */
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { frontendRoot, installedModules, moduleSelection } from "./module-source.mjs";

const selection = moduleSelection(process.env.FRONTEND_MODULE_ENV || "production");
if (selection.source === "npm") {
  const paths = { "@/*": [join(frontendRoot, "apps/shell/src/*")] };
  for (const { pkg, path } of installedModules(selection)) {
    for (const [subpath, entry] of Object.entries(pkg.exports || {})) {
      if (!entry?.types) continue;
      paths[pkg.name + (subpath === "." ? "" : subpath.slice(1))] = [join(path, entry.types)];
    }
  }
  const fixture = mkdtempSync(join(tmpdir(), "moh-npm-shell-types-"));
  try {
    const config = join(fixture, "tsconfig.json");
    writeFileSync(
      config,
      JSON.stringify({
        extends: join(frontendRoot, "tsconfig.base.json"),
        compilerOptions: { paths, types: [], noEmit: true },
        files: [join(frontendRoot, "node_modules/vite/client.d.ts")],
        include: [
          join(frontendRoot, "apps/shell/src/**/*.ts"),
          join(frontendRoot, "apps/shell/src/**/*.tsx"),
        ],
      }),
    );
    execFileSync(
      process.execPath,
      [join(frontendRoot, "node_modules/typescript/bin/tsc"), "--project", config],
      { cwd: frontendRoot, env: process.env, stdio: "inherit" },
    );
  } finally {
    rmSync(fixture, { recursive: true, force: true });
  }
}
