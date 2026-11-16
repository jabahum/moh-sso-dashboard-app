/* global process, URL */
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const root = fileURLToPath(new URL("../", import.meta.url));
function run(script) {
  return spawnSync(process.execPath, ["--input-type=module", "-e", script], {
    cwd: root,
    env: { ...process.env, ESBUILD_WORKER_THREADS: "0" },
    encoding: "utf8",
    timeout: 20000,
  });
}
for (const closeFails of [false, true]) {
  test(`standalone test exits naturally after esbuild cleanup (server close fails=${closeFails})`, () => {
    const result = run(`
      import assert from "node:assert/strict";
      import { context } from "esbuild";
      import { closeViteTestServer, failOnOpenTestResources } from "./scripts/vite-test-server.mjs";
      const build = await context({ stdin: { contents: "export const value: number = 1;", loader: "ts" }, write: false });
      await build.rebuild();
      let closed = false;
      const server = { async close() { closed = true; ${closeFails ? 'throw new Error("close failure");' : ""} } };
      ${closeFails ? "await assert.rejects(closeViteTestServer(server), /close failure/);" : "await closeViteTestServer(server);"}
      assert.equal(closed, true);
      failOnOpenTestResources(1000);
      console.log("Cleanup completed");
    `);
    assert.equal(result.error, undefined, result.error?.message);
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Cleanup completed/);
  });
}
test("a retained worker is reported and fails instead of hanging CI", () => {
  const result = run(`
    import { context } from "esbuild";
    import { failOnOpenTestResources } from "./scripts/vite-test-server.mjs";
    const build = await context({ stdin: { contents: "export const value = 1;" }, write: false });
    await build.rebuild();
    failOnOpenTestResources(100);
  `);
  assert.equal(result.error, undefined, result.error?.message);
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stderr, /resources remain open/);
});
