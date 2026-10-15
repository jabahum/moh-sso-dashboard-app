import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { URL } from "node:url";
import { runInNewContext } from "node:vm";
import ts from "typescript";

const source = readFileSync(
  new URL("../apps/report-scheduler/src/utils/report-parameters.ts", import.meta.url),
  "utf8",
);
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const exports = {};
runInNewContext(outputText, { exports });
const { populateReportParameters, parameterInputValue } = exports;
const values = (schema, overrides = {}) =>
  JSON.parse(JSON.stringify(populateReportParameters(schema, overrides)));

test("prefills configured defaults without inventing values for other filters", () => {
  assert.deepEqual(values([{ name: "basket", defaultValue: "EMHS" }, { name: "region" }]), {
    basket: "EMHS",
  });
});

test("saved and edited values win over defaults, including intentional blanks and falsy values", () => {
  const schema = [
    { name: "basket", defaultValue: "EMHS" },
    { name: "enabled", defaultValue: true },
    { name: "limit", defaultValue: 5 },
  ];
  assert.deepEqual(values(schema, { basket: "", enabled: false, limit: 0 }), {
    basket: "",
    enabled: false,
    limit: 0,
  });
  const saved = { basket: "edited", locations: ["A", "B"] };
  assert.deepEqual(values(schema, saved), {
    basket: "edited",
    enabled: true,
    limit: 5,
    locations: ["A", "B"],
  });
  assert.deepEqual(saved, { basket: "edited", locations: ["A", "B"] });
});

test("refetched defaults never overwrite the user's changes", () => {
  const edited = { basket: "user choice" };
  assert.equal(values([{ name: "basket", defaultValue: "initial" }], edited).basket, "user choice");
  assert.equal(
    values([{ name: "basket", defaultValue: "refreshed" }], edited).basket,
    "user choice",
  );
  assert.deepEqual(values([{ name: "another_report", defaultValue: "new" }], {}), {
    another_report: "new",
  });
});

test("respects Health BI noDefault settings and legacy metadata defaults", () => {
  assert.deepEqual(
    values([{ name: "basket", defaultValue: "EMHS", metadata: { noDefault: true } }]),
    {},
  );
  assert.deepEqual(
    values([
      { name: "basket", defaultValue: "EMHS", metadata: { noDefault: true, hideNone: true } },
    ]),
    { basket: "EMHS" },
  );
  assert.deepEqual(values([{ name: "basket", metadata: { defaultValue: "EMHS" } }]), {
    basket: "EMHS",
  });
  assert.deepEqual(
    values([{ name: "basket", defaultValue: "EMHS", metadata: { noDefault: true } }], {
      basket: "saved",
    }),
    { basket: "saved" },
  );
});

test("displays typed saved parameters without changing their stored values", () => {
  assert.equal(parameterInputValue(false), "false");
  assert.equal(parameterInputValue(0), "0");
  assert.equal(parameterInputValue(["A", "B"]), "A, B");
  assert.equal(parameterInputValue(null), "");
});
