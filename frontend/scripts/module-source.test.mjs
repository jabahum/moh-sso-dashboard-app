import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { installedModules, moduleSelection, npmModulePlugin } from "./module-source.mjs";

function fixture(run) {
  const root = mkdtempSync(join(tmpdir(), "moh-module-selection-"));
  const write = (path, value) => {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, typeof value === "string" ? value : JSON.stringify(value));
  };
  const pkg = (name, version) => ({
    name: `@moh-sso/${name}`,
    version,
    exports: { ".": { import: "./dist/index.js" } },
  });
  write(join(root, "apps/shell/package.json"), {
    name: "@moh-sso/shell",
    version: "1.0.0",
    private: true,
  });
  write(join(root, "apps/users/package.json"), pkg("users", "1.0.0"));
  write(join(root, "packages/api/package.json"), pkg("api", "1.0.0"));
  const versions = { "@moh-sso/users": "2.0.0", "@moh-sso/api": "3.0.0" };
  const config = {
    development: { source: "npm", versions },
    production: { source: "workspace", versions },
  };
  write(join(root, "module-sources.json"), config);
  const selection = () => moduleSelection("development", {}, root);
  const install = () => {
    const selected = selection();
    for (const name of ["users", "api"]) {
      const path = join(selected.directory, "node_modules/@moh-sso", name);
      write(join(path, "package.json"), pkg(name, versions[`@moh-sso/${name}`]));
      write(join(path, "dist/index.js"), "export const origin = 'npm';");
    }
    return selected;
  };
  try {
    run({ root, write, config, selection, install });
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}
test("development and production select sources independently and exclude the shell", () =>
  fixture(({ root, selection }) => {
    assert.equal(selection().source, "npm");
    assert.equal(selection().versions["@moh-sso/api"], "3.0.0");
    const production = moduleSelection("production", {}, root);
    assert.equal(production.source, "workspace");
    assert.equal(production.versions["@moh-sso/api"], "1.0.0");
    assert.equal(production.modules.length, 2);
    assert.equal(
      moduleSelection("development", { FRONTEND_MODULE_SOURCE: "workspace" }, root).source,
      "workspace",
    );
    assert.equal(
      moduleSelection("production", { FRONTEND_MODULE_SOURCE: "npm" }, root).source,
      "npm",
    );
  }));
for (const pin of ["latest", "^3.0.0", "~3.0.0", "3.x", "file:../api"]) {
  test(`reject floating npm pin ${pin}`, () =>
    fixture(({ root, config, write, selection }) => {
      config.development.versions["@moh-sso/api"] = pin;
      write(join(root, "module-sources.json"), config);
      assert.throws(selection, /exact semantic version/);
    }));
}
test("reject incomplete pins, invalid sources, and invalid environments", () =>
  fixture(({ root, config, write, selection }) => {
    delete config.development.versions["@moh-sso/api"];
    write(join(root, "module-sources.json"), config);
    assert.throws(selection, /exactly all/);
    assert.throws(
      () => moduleSelection("development", { FRONTEND_MODULE_SOURCE: "invalid" }, root),
      /Invalid frontend module source/,
    );
    assert.throws(
      () => moduleSelection("staging", {}, root),
      /Invalid frontend module environment/,
    );
  }));
test("npm resolution uses selected deployed artifacts and rejects private subpaths", () =>
  fixture(({ install }) => {
    const selected = install();
    assert.equal(
      installedModules(selected).find(({ name }) => name === "api").pkg.version,
      "3.0.0",
    );
    const plugin = npmModulePlugin(selected);
    assert.equal(
      plugin.resolveId("@moh-sso/api"),
      join(selected.directory, "node_modules/@moh-sso/api/dist/index.js"),
    );
    assert.equal(plugin.resolveId("react"), null);
    assert.throws(() => plugin.resolveId("@moh-sso/api/src/private"), /does not export/);
  }));
test("missing artifacts and stale installed versions fail rather than using source", () =>
  fixture(({ install, write }) => {
    const selected = install();
    const path = join(selected.directory, "node_modules/@moh-sso/api");
    rmSync(join(path, "dist/index.js"));
    assert.throws(() => installedModules(selected), /Missing npm artifact/);
    write(join(path, "dist/index.js"), "export {};");
    const metadata = JSON.parse(readFileSync(join(path, "package.json"), "utf8"));
    metadata.version = "1.0.0";
    write(join(path, "package.json"), metadata);
    assert.throws(() => installedModules(selected), /version mismatch/);
  }));
test("workspace symlinks cannot impersonate registry packages", () =>
  fixture(({ install, root }) => {
    const selected = install();
    const path = join(selected.directory, "node_modules/@moh-sso/api");
    rmSync(path, { recursive: true });
    symlinkSync(join(root, "packages/api"), path);
    assert.throws(() => installedModules(selected), /isolated npm installation/);
  }));
test("reject pins incompatible with a deployed package's internal dependencies", () =>
  fixture(({ install, write }) => {
    const selected = install();
    const path = join(selected.directory, "node_modules/@moh-sso/users/package.json");
    const pkg = JSON.parse(readFileSync(path, "utf8"));
    pkg.dependencies = { "@moh-sso/api": "^2.0.0" };
    write(path, pkg);
    assert.throws(() => installedModules(selected), /incompatible with pin/);
  }));

test("npm shell entry loads shared UI styles without changing other modules", () =>
  fixture(({ install, root }) => {
    const plugin = npmModulePlugin(install());
    const shell = join(root, "apps/shell");
    plugin.configResolved({ root: shell });
    const entry = plugin.transform("export {};", join(shell, "src/main.tsx"));
    assert.match(entry.code, /import "@moh-sso\/ui\/styles.css"/);
    assert.equal(plugin.transform("export {};", join(shell, "src/app/App.tsx")), null);
  }));
