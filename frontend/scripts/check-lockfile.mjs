/* global console, process */
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL, URL } from "node:url";

export function checkLockfile(lock) {
  const failures = [];
  if (lock.lockfileVersion !== 3 || !lock.packages) return ["Expected a v3 npm lockfile"];
  for (const [location, entry] of Object.entries(lock.packages)) {
    if (entry.link || !entry.resolved) continue;
    if (!entry.resolved.startsWith("https://")) {
      failures.push(`${location}: dependency must use an HTTPS download`);
      continue;
    }
    const url = new URL(entry.resolved);
    if (url.username || url.password)
      failures.push(`${location}: credentials must not appear in download URLs`);
    if (url.hostname !== "registry.npmjs.org") continue;
    const name = entry.name ?? location.split("node_modules/").at(-1);
    const filename = `${name.split("/").at(-1)}-${entry.version}.tgz`;
    if (decodeURIComponent(url.pathname) !== `/${name}/-/${filename}` || url.search || url.hash) {
      failures.push(`${location}: registry URL does not match its package name/version`);
    }
    if (!/^sha(?:256|384|512)-[A-Za-z0-9+/]+={0,2}$/.test(entry.integrity ?? "")) {
      failures.push(`${location}: missing or invalid download integrity`);
    }
  }
  return failures;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const failures = checkLockfile(JSON.parse(readFileSync("package-lock.json", "utf8")));
  if (failures.length) {
    console.error(failures.join("\n"));
    process.exitCode = 1;
  } else console.log("Lockfile download paths and integrity are valid.");
}
