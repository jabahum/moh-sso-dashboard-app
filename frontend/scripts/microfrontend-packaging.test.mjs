import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { build, loadConfigFromFile } from "vite";
import { frontendRoot, workspaceModules } from "./module-source.mjs";
import cssInjectedByJsPlugin from "../build/css-injected-by-js.ts";
import { closeViteTestServer } from "./vite-test-server.mjs";

test("published CSS injection survives Vite rebundling into shell assets", async () => {
  const root = mkdtempSync(join(tmpdir(), "moh-npm-css-bundle-"));
  try {
    const plugin = cssInjectedByJsPlugin();
    const bundle = {
      "index.js": {
        type: "chunk",
        fileName: "index.js",
        isEntry: true,
        code: "export const value = 1;",
      },
      "assets/panel.css": {
        type: "asset",
        fileName: "assets/panel.css",
        source: ".panel { color: red; }",
      },
    };
    plugin.generateBundle.handler.call({}, {}, bundle);
    mkdirSync(join(root, "assets"));
    writeFileSync(join(root, "assets/panel.css"), bundle["assets/panel.css"].source);
    writeFileSync(join(root, "index.js"), bundle["index.js"].code);
    const output = await build({
      configFile: false,
      root,
      logLevel: "silent",
      base: "/portal/",
      build: {
        write: false,
        assetsInlineLimit: 0,
        minify: false,
        rollupOptions: { input: join(root, "index.js") },
      },
    });
    const css = output.output.find(
      (item) => item.type === "asset" && item.fileName.endsWith(".css"),
    );
    assert.ok(css, "The shell build must retain the package's CSS asset");
    const chunk = output.output.find((item) => item.type === "chunk");
    assert.ok(chunk.code.includes(`/portal/${css.fileName}`));
    assert.equal(chunk.code.includes("new URL(file, import.meta.url)"), false);
  } finally {
    await closeViteTestServer();
    rmSync(root, { recursive: true, force: true });
  }
});

test("app packages keep shared providers and state external", async () => {
  try {
    const result = await loadConfigFromFile(
      { command: "build", mode: "production" },
      join(frontendRoot, "apps/users/vite.config.ts"),
    );
    const external = result.config.build.rollupOptions.external;
    for (const { pkg } of workspaceModules().filter(({ group }) => group === "packages")) {
      assert.ok(
        external.includes(pkg.name),
        `${pkg.name} must remain shared instead of being bundled into app packages`,
      );
    }
  } finally {
    await closeViteTestServer();
  }
});

test("published lifecycle uses the host React renderer and JSX runtime", async () => {
  try {
    const result = await loadConfigFromFile(
      { command: "build", mode: "production" },
      join(frontendRoot, "packages/microfrontend/vite.config.ts"),
    );
    const output = await build({
      ...result.config,
      configFile: false,
      logLevel: "silent",
      build: { ...result.config.build, write: false },
    });
    const chunks = [output].flat().flatMap((result) => result.output)
      .filter((item) => item.type === "chunk");
    const imports = chunks.flatMap((chunk) => chunk.imports);
    assert.ok(imports.includes("react-dom/client"), "The host must supply the renderer");
    assert.ok(imports.includes("react/jsx-runtime"), "The host must supply the JSX runtime");
    for (const chunk of chunks) {
      assert.equal(chunk.code.includes("Incompatible React versions"), false);
      assert.equal(chunk.code.includes("checkDCE"), false);
    }
  } finally {
    await closeViteTestServer();
  }
});
