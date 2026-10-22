import assert from "node:assert/strict";
import test from "node:test";
import { checkLockfile } from "./check-lockfile.mjs";

const lock = () => ({
  lockfileVersion: 3,
  packages: {
    "node_modules/@carbon/charts": {
      version: "1.27.12",
      resolved: "https://registry.npmjs.org/@carbon/charts/-/charts-1.27.12.tgz",
      integrity: "sha512-YWJjZA==",
    },
    "node_modules/local": { resolved: "packages/local", link: true },
  },
});
test("accepts scoped packages, npm aliases and workspace links", () => {
  const data = lock();
  data.packages["node_modules/alias"] = {
    ...data.packages["node_modules/@carbon/charts"],
    name: "@carbon/charts",
  };
  assert.deepEqual(checkLockfile(data), []);
});
test("rejects the infrastructure rename corruption before installing packages", () => {
  const data = lock();
  data.packages["node_modules/@carbon/charts"].resolved =
    "https://registry.npmjs.org/@carbon/infra/helm/-/charts-1.27.12.tgz";
  assert.match(checkLockfile(data).join(), /package name\/version/);
});
test("rejects plaintext downloads, embedded credentials and missing integrity", () => {
  for (const changes of [
    { resolved: "http://registry.npmjs.org/pkg" },
    { resolved: "https://user:secret@registry.npmjs.org/pkg" },
    { integrity: "" },
  ]) {
    const data = lock();
    Object.assign(data.packages["node_modules/@carbon/charts"], changes);
    assert.ok(checkLockfile(data).length);
  }
});
