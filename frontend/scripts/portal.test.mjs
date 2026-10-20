/* global process */
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { root, loadProfile, readMap, execute, supervise } from "./portal-runtime.mjs";
import { migrationProblem, runStack, stackContext } from "./portal-stack.mjs";

function directory(t) {
  const path = mkdtempSync(join(tmpdir(), "portal-cli-"));
  t.after(() => rmSync(path, { recursive: true, force: true }));
  return path;
}

function fixture(t, options = {}) {
  const folder = directory(t);
  writeFileSync(join(folder, "index.html"), "shell");
  writeFileSync(join(folder, "map.json"), '{"imports":{}}');
  const profile = { ...loadProfile(), dist: folder, importMap: join(folder, "map.json") };
  const events = [];
  const ctx = stackContext(profile, undefined, {
    run: async (_file, args) => {
      events.push(["docker", ...args]);
      return "";
    },
    compose: async (args) => {
      events.push(args);
      if (args[0] === "ps") return "[]";
      if (args[0] === "exec") return options.version || "39|f\n";
      if (options.failAt === args[0]) throw new Error("Dependency failed");
      return "";
    },
    freePort: async () => {
      events.push(["port-check"]);
      if (options.occupied) throw new Error("Port occupied");
    },
    wait: async (name) => {
      events.push(["wait", name]);
      if (options.failReady === name) throw new Error("Not ready");
    },
  });
  return { ctx, events };
}

test("profiles inherit local defaults and resolve paths independently of cwd", (t) => {
  const folder = directory(t);
  const file = join(folder, "profiles.json");
  writeFileSync(
    file,
    JSON.stringify({
      demo: {
        port: 3100,
        publicOrigin: "http://localhost:3100",
        importMap: "public/import-map.json",
      },
    }),
  );
  const profile = loadProfile("demo", file);
  assert.equal(profile.port, 3100);
  assert.equal(profile.database, "sso_express_dev");
  assert.equal(profile.importMap, resolve(root, "public/import-map.json"));
  assert.ok(profile.composeFiles.every((path) => path.startsWith("/")));
  assert.throws(() => loadProfile("missing", file), /Unknown profile/);
  assert.throws(() => loadProfile("demo", join(folder, "missing.json")), /missing/);
});

test("unsafe database names and invalid profile URLs are rejected", (t) => {
  const file = join(directory(t), "profiles.json");
  writeFileSync(file, JSON.stringify({ local: { database: 'db";DROP' } }));
  assert.throws(() => loadProfile("local", file), /identifier/);
  writeFileSync(file, JSON.stringify({ local: { backend: "file:///tmp" } }));
  assert.throws(() => loadProfile("local", file), /HTTP/);
});

test("migration diagnostics distinguish newer schemas and dirty migrations", () => {
  assert.match(migrationProblem(47, false, 39), /migration 47.*39/);
  assert.match(migrationProblem(10, true, 39), /dirty/);
  assert.equal(migrationProblem(39, false, 39), undefined);
  assert.equal(migrationProblem(3, false, 39), undefined);
});

test("up waits for application readiness before starting Express", async (t) => {
  const { ctx, events } = fixture(t);
  await runStack("up", ctx, { register: async () => events.push(["callback"]) });
  const backendReady = events.findIndex((event) => event.join(" ") === "wait backend");
  const expressStart = events.findIndex((event) => event.includes("express") && event[0] === "up");
  assert.ok(backendReady > 0 && expressStart > backendReady);
  assert.ok(events.some((event) => event.join(" ") === "run --rm --no-deps portal-db-init"));
  assert.deepEqual(events.at(-1), ["wait", "API proxy"]);
});

test("failed readiness prevents Express from starting", async (t) => {
  const { ctx, events } = fixture(t, { failReady: "backend" });
  await assert.rejects(runStack("up", ctx, { register: async () => {} }), /Not ready/);
  assert.ok(!events.some((event) => event[0] === "up" && event.includes("express")));
});

test("newer/dirty databases stop up before backend startup and never get forced", async (t) => {
  for (const version of ["47|f", "10|t"]) {
    const { ctx, events } = fixture(t, { version });
    await assert.rejects(runStack("up", ctx), /migration|dirty/);
    assert.ok(!events.some((event) => event[0] === "up" && event.includes("backend")));
    assert.ok(!JSON.stringify(events).includes("force"));
  }
});

test("occupied host port prevents stack mutations", async (t) => {
  const { ctx, events } = fixture(t, { occupied: true });
  await assert.rejects(runStack("up", ctx), /Port occupied/);
  assert.ok(!events.some((event) => ["up", "run", "exec"].includes(event[0])));
});

test("database inspection errors cannot be mistaken for an empty database", async (t) => {
  const { ctx, events } = fixture(t);
  const original = ctx.compose;
  ctx.compose = async (args) => {
    if (args[0] === "exec")
      throw Object.assign(new Error("database unavailable"), { stderr: "connection refused" });
    return original(args);
  };
  await assert.rejects(runStack("up", ctx), /database unavailable/);
  assert.ok(!events.some((event) => event[0] === "up" && event.includes("backend")));
});

test("empty database can proceed to backend migrations", async (t) => {
  const { ctx, events } = fixture(t);
  const original = ctx.compose;
  ctx.compose = async (args) => {
    if (args[0] === "exec")
      throw Object.assign(new Error("empty database"), {
        stderr: 'ERROR: relation "schema_migrations" does not exist',
      });
    return original(args);
  };
  await runStack("up", ctx, { register: async () => {} });
  assert.ok(events.some((event) => event[0] === "up" && event.includes("backend")));
});

test("down stops only managed services without deleting containers or volumes", async (t) => {
  const { ctx, events } = fixture(t);
  await runStack("down", ctx);
  assert.equal(events.length, 1);
  assert.equal(events[0][0], "stop");
  assert.ok(events[0].includes("express"));
  assert.ok(!events[0].includes("frontend"));
  assert.ok(!events[0].some((part) => ["down", "--volumes", "rm", "-v"].includes(part)));
});

test("down frontend stops Express alone", async (t) => {
  const { ctx, events } = fixture(t);
  await runStack("down", ctx, { service: "frontend" });
  assert.deepEqual(events, [["stop", "express"]]);
});

test("backend restart recreates the selected service with profile config", async (t) => {
  const { ctx, events } = fixture(t);
  await runStack("restart", ctx, { service: "backend" });
  assert.deepEqual(events[0], ["up", "-d", "--no-deps", "--force-recreate", "backend"]);
  assert.deepEqual(events[1], ["wait", "backend"]);
  await assert.rejects(runStack("logs", ctx, { service: "unknown" }), /Unknown service/);
});

test("dry-run performs no Docker calls or filesystem mutations", async (t) => {
  const { ctx, events } = fixture(t);
  for (const action of ["up", "down", "restart", "logs", "status", "doctor"])
    await runStack(action, ctx, { dryRun: true });
  assert.deepEqual(events, []);
});

test("mixed development map preserves all other module/vendor entries", async (t) => {
  const path = join(directory(t), "map.json");
  const original = {
    imports: {
      react: "https://example.com/react.js",
      "@moh-sso/users": "/mf/users.js",
      "@moh-sso/documents": "/mf/documents.js",
    },
  };
  writeFileSync(path, JSON.stringify(original));
  const map = await readMap(path);
  map.imports["@moh-sso/users"] = "http://localhost:4101/single-spa.js";
  assert.equal(map.imports.react, original.imports.react);
  assert.equal(map.imports["@moh-sso/documents"], original.imports["@moh-sso/documents"]);
  assert.deepEqual(JSON.parse(readFileSync(path)), original);
});

test("executor propagates a child failure exit code", async () => {
  await assert.rejects(
    execute(process.execPath, ["-e", "process.exit(7)"], { quiet: true }),
    (error) => error.exitCode === 7,
  );
});

test("development supervisor terminates siblings when one process fails", async (t) => {
  const pidFile = join(directory(t), "worker.pid");
  await assert.rejects(
    supervise(
      [
        [
          process.execPath,
          [
            "-e",
            `require('node:fs').writeFileSync(${JSON.stringify(pidFile)}, String(process.pid)); setInterval(()=>{},1000);`,
          ],
        ],
        [process.execPath, ["-e", "setTimeout(()=>process.exit(3),250);"]],
      ],
      process.env,
    ),
    /Development process stopped/,
  );
  const pid = Number(readFileSync(pidFile, "utf8"));
  assert.throws(() => process.kill(pid, 0), /ESRCH/);
});

test("CLI works from another cwd and rejects invalid commands/options", (t) => {
  const cwd = directory(t);
  const cli = resolve(root, "scripts/portal.mjs");
  for (const args of [
    ["up", "--dry-run"],
    ["down", "--dry-run"],
    ["dev", "users", "--port", "3100", "--dry-run"],
    ["serve", "users", "--profile", "local", "--dry-run"],
  ]) {
    const result = spawnSync(process.execPath, [cli, ...args], { cwd, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /[Pp]rofile/);
  }
  for (const args of [
    ["restart", "unknown"],
    ["up", "--timeout", "0"],
    ["status", "--port", "12"],
    ["dev", "users", "--module-port", "3000"],
  ]) {
    const result = spawnSync(process.execPath, [cli, ...args], { cwd, encoding: "utf8" });
    assert.equal(result.status, 1, `${args}: ${result.stdout}`);
  }
});
