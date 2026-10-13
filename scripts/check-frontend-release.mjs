import { execFileSync } from "node:child_process";
import { readFileSync, realpathSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

export function withPreservedFiles(paths, run) {
  const originals = paths.map((path) => [path, readFileSync(path)]);
  try {
    return run();
  } finally {
    for (const [path, contents] of originals) writeFileSync(path, contents);
  }
}

if (
  process.argv[1] &&
  realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const frontend = fileURLToPath(new URL("../frontend/", import.meta.url));
  const [npm = "npm", node = process.execPath] = process.argv.slice(2);
  try {
    withPreservedFiles(
      ["config.js", "import-map.json", "version-manifest.json"].map((name) =>
        resolve(frontend, "public", name),
      ),
      () => {
        execFileSync(node, ["--test", "scripts/test-versioning.mjs"], {
          cwd: frontend,
          stdio: "inherit",
        });
        execFileSync(npm, ["run", "release:check"], {
          cwd: frontend,
          stdio: "inherit",
        });
      },
    );
  } catch (error) {
    console.error(`Frontend release verification failed: ${error.message}`);
    process.exitCode = 1;
  }
}
