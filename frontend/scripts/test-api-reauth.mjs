import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import test from "node:test";
import { URL } from "node:url";
import { runInNewContext } from "node:vm";
import ts from "typescript";

const require = createRequire(import.meta.url);

// Exercise the real handler and reducer with a controlled HTTP transport.
function loadSource(relativePath, imports = {}) {
  const source = readFileSync(new URL(relativePath, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  });
  const exports = {};
  runInNewContext(outputText, {
    exports,
    require: (name) => imports[name] ?? require(name),
  });
  return exports;
}

const auth = loadSource("../packages/auth/src/auth/auth.slice.ts");
const unauthorized = { error: { status: 401, data: { message: "Unauthorized" } } };
const success = { data: { success: true } };

function setup(responses) {
  const calls = [];
  let state = auth.default(undefined, auth.loginSuccess({ user: { id: "user-1" } }));
  const { baseApi } = loadSource("../packages/api/src/api/baseApi.ts", {
    "@reduxjs/toolkit/query/react": {
      createApi: (config) => config,
      fetchBaseQuery: () => async (args) => {
        calls.push(args);
        assert.ok(responses.length, "Unexpected extra HTTP request");
        const response = responses.shift();
        return typeof response === "function" ? response() : response;
      },
    },
    "@moh-sso/config": { API: { base: "http://localhost/api/v1" } },
    "../../../auth/src/auth/auth.slice": auth,
  });
  return {
    calls,
    state: () => state,
    query: (args) =>
      baseApi.baseQuery(args, { dispatch: (action) => (state = auth.default(state, action)) }, {}),
  };
}

test("persistent 401 after refresh clears auth for the login route guard", async () => {
  const app = setup([unauthorized, success, unauthorized]);
  assert.equal(await app.query("/documents"), unauthorized);
  assert.equal(app.state().isAuthenticated, false);
  assert.equal(app.state().user, null);
  assert.equal(app.state().loaded, true);
  assert.equal(app.state().loading, false);
  assert.equal(app.calls.length, 3);
});

test("successful refresh and retry keep the user signed in", async () => {
  const app = setup([unauthorized, success, success]);
  assert.equal(await app.query("/documents"), success);
  assert.equal(app.state().isAuthenticated, true);
  assert.equal(app.calls[1].url, "/auth/refresh");
});

test("failed refresh clears auth without retrying the original endpoint", async () => {
  const app = setup([unauthorized, unauthorized]);
  await app.query("/documents");
  assert.equal(app.state().isAuthenticated, false);
  assert.equal(app.calls.length, 2);
});

test("403 permission errors do not expire the session", async () => {
  const forbidden = { error: { status: 403, data: {} } };
  const app = setup([forbidden]);
  assert.equal(await app.query("/documents"), forbidden);
  assert.equal(app.state().isAuthenticated, true);
  assert.equal(app.calls.length, 1);
});

test("auth endpoints never recursively refresh on 401", async () => {
  for (const endpoint of ["refresh", "login", "logout", "callback"]) {
    const app = setup([unauthorized]);
    await app.query(`/auth/${endpoint}`);
    assert.equal(app.state().isAuthenticated, false);
    assert.equal(app.calls.length, 1);
  }
});

test("concurrent 401 responses share one refresh request", async () => {
  let finishRefresh;
  const refresh = new Promise((resolve) => (finishRefresh = resolve));
  const app = setup([unauthorized, unauthorized, () => refresh, success, unauthorized]);
  const results = [app.query("/documents"), app.query("/users")];
  await Promise.resolve();
  await Promise.resolve();
  finishRefresh(success);
  await Promise.all(results);
  assert.equal(app.calls.filter((args) => args.url === "/auth/refresh").length, 1);
  assert.equal(app.state().isAuthenticated, false);
});
