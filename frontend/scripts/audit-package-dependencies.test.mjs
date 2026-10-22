import assert from "node:assert/strict";
import test from "node:test";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { auditDependencies, sourceImports } from "./audit-package-dependencies.mjs";

test("includes stylesheet side effects, re-exports and dynamic imports", () => {
  assert.deepEqual(
    [
      ...sourceImports(`
    import "@carbon/charts/styles.css";
    import type { User } from "@moh-sso/types";
    export { value } from "shared";
    const load = () => import("lazy");
    // import "not-a-dependency";
    const label = 'from "also-not-a-dependency"';
  `),
    ].sort(),
    ["@carbon/charts/styles.css", "@moh-sso/types", "lazy", "shared"],
  );
});

test("rejects undeclared side-effect packages and missing stylesheet subpaths", () => {
  const root = mkdtempSync(join(tmpdir(), "moh-import-audit-"));
  try {
    const app = join(root, "apps", "demo");
    mkdirSync(join(app, "src"), { recursive: true });
    writeFileSync(
      join(app, "package.json"),
      JSON.stringify({ name: "demo", dependencies: { "@carbon/charts": "1.27.12" } }),
    );
    writeFileSync(join(app, "src", "main.ts"), 'import "@carbon/infra/helm/styles.css";');
    assert.ok(auditDependencies(root).some((failure) => failure.missing.includes("@carbon/infra")));
    writeFileSync(join(app, "src", "main.ts"), 'import "@carbon/charts/missing-style.css";');
    assert.ok(
      auditDependencies(root).some((failure) =>
        failure.missing.some((entry) => entry.includes("Unresolvable stylesheet")),
      ),
    );
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
