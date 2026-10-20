/* global process, console */
import { mkdirSync, writeFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import {
  root,
  repo,
  httpUrl,
  readMap,
  assertFreePort,
  execute,
  npmArgs,
  supervise,
} from "./portal-runtime.mjs";

export async function develop(profile, selected, values) {
  const validatePort = (value) => {
    if (!/^\d+$/.test(String(value)) || Number(value) < 1 || Number(value) > 65535)
      throw new Error("Ports must be between 1 and 65535.");
    return Number(value);
  };
  const port = validatePort(values.port || process.env.PORT || profile.port);
  const modulePort = validatePort(values["module-port"] || selected.port);
  if (port === modulePort) throw new Error("Express and module ports must differ.");
  const source =
    values["import-map"] ||
    process.env.IMPORT_MAP_URL ||
    process.env.IMPORT_MAP_FILE ||
    profile.importMap;
  const mapSource = /^https?:\/\//.test(source)
    ? httpUrl(source, "import map").href
    : resolve(source);
  const env = {
    ...process.env,
    BACKEND_URL: values.backend || process.env.BACKEND_URL || profile.backend,
    HOST: values.host || profile.host,
    PORT: String(port),
    STANDALONE_MODULE: selected.name,
    FRONTEND_DIST_DIR: values.dist ? resolve(values.dist) : profile.dist,
  };
  httpUrl(env.BACKEND_URL, "backend");
  if (values["dry-run"]) {
    console.log(
      `Profile ${profile.name}: build/watch ${selected.name}; preview localhost:${modulePort}; standalone Express ${env.HOST}:${port}; backend ${env.BACKEND_URL}`,
    );
    console.log(
      `Copy ${mapSource} to an ignored mixed map, override only @moh-sso/${selected.name}, and supervise watch/preview/Express. Refresh after rebuild.`,
    );
    return;
  }
  if (Number(process.versions.node.split(".")[0]) < 22) throw new Error("Use Node 22 or later.");
  if (!existsSync(resolve(env.FRONTEND_DIST_DIR, "index.html")))
    throw new Error("Shell build missing. Run 'portal build'.");
  if (!existsSync(resolve(root, "server/node_modules/express")))
    throw new Error("Host dependencies missing. Run 'portal setup'.");
  await assertFreePort(port, env.HOST);
  await assertFreePort(modulePort, "127.0.0.1");
  const map = await readMap(mapSource);
  map.imports[`@moh-sso/${selected.name}`] = `http://localhost:${modulePort}/single-spa.js`;
  const folder = resolve(repo, ".local-sso");
  mkdirSync(folder, { recursive: true, mode: 0o700 });
  const filename = resolve(
    folder,
    `import-map.${profile.name.replace(/[^a-zA-Z0-9_-]/g, "_")}.${selected.name}.json`,
  );
  writeFileSync(filename, `${JSON.stringify(map, null, 2)}\n`, { mode: 0o600 });
  delete env.IMPORT_MAP_URL;
  env.IMPORT_MAP_FILE = filename;
  await execute(
    process.execPath,
    npmArgs(["run", "build", "--workspace", `@moh-sso/${selected.name}`]),
    { env },
  );
  console.log(`Development host: http://localhost:${port}/portal/ (refresh after rebuild)`);
  return supervise(
    [
      [
        process.execPath,
        [
          resolve(root, "node_modules/vite/bin/vite.js"),
          "build",
          "--watch",
          "--config",
          resolve(root, "apps", selected.name, "vite.config.ts"),
        ],
      ],
      [
        process.execPath,
        [
          resolve(root, "node_modules/vite/bin/vite.js"),
          "preview",
          "--config",
          resolve(root, "apps", selected.name, "vite.config.ts"),
          "--host",
          "127.0.0.1",
          "--port",
          String(modulePort),
          "--strictPort",
        ],
      ],
      [process.execPath, [resolve(root, "server/index.mjs")]],
    ],
    env,
  );
}
