import { pathToFileURL } from "node:url";

const requiredApps = [
  "announcements", "audit", "clients", "data-validation", "data-visualizer",
  "documents", "e-services", "email", "issue-tracker", "report-browser",
  "rbac", "surveillance", "users", "utilities",
];
const requiredPackages = ["api", "auth", "config", "microfrontend", "state", "types", "ui", "utils"];

export async function verifyFrontend(baseURL, fetchImpl = fetch) {
  const base = new URL(baseURL);
  if (!base.pathname.endsWith("/")) base.pathname += "/";
  const checked = new Set();
  async function get(path, kind) {
    const url = new URL(path, base);
    if (url.origin !== base.origin) throw new Error("Expected a same-origin deployment asset: " + url);
    const response = await fetchImpl(url, { signal: AbortSignal.timeout(10000), redirect: "error" });
    if (!response.ok) throw new Error("HTTP " + response.status + " for " + url.pathname);
    const type = response.headers.get("content-type") ?? "";
    const body = await response.text();
    if (!body.trim()) throw new Error("Empty " + kind + " response: " + url.pathname);
    const expected = {
      html: /text\/html/i,
      json: /application\/json/i,
      js: /(?:java|ecma)script/i,
      css: /text\/css/i,
    }[kind];
    if (!expected.test(type) || (kind !== "html" && /^\s*(?:<!doctype\s+html|<html)/i.test(body))) {
      throw new Error("Unexpected content for " + url.pathname + ": " + type);
    }
    checked.add(url.href);
    return body;
  }

  const html = await get("./", "html");
  if (!/id=["']root["']/.test(html)) throw new Error("Portal root element is missing");
  const config = await get("config.js", "js");
  if (!config.includes("__APP_CONFIG__")) throw new Error("Runtime config is missing");
  const map = JSON.parse(await get("import-map.json", "json"));
  const imports = map.imports ?? {};
  for (const name of [...requiredApps, ...requiredPackages]) {
    const specifier = "@moh-sso/" + name;
    if (typeof imports[specifier] !== "string") throw new Error("Missing import: " + specifier);
    await get(imports[specifier], "js");
  }

  const scripts = [...html.matchAll(/<script\b[^>]*\bsrc=["']([^"']+)["'][^>]*>/gi)];
  if (!scripts.some((match) => match[1].includes("/assets/"))) {
    throw new Error("Built shell entry script is missing");
  }
  for (const match of scripts) {
    if (!checked.has(new URL(match[1], base).href)) await get(match[1], "js");
  }
  for (const match of html.matchAll(/<link\b[^>]*>/gi)) {
    if (!/\brel=["']stylesheet["']/i.test(match[0])) continue;
    const href = match[0].match(/\bhref=["']([^"']+)["']/i)?.[1];
    if (href) await get(href, "css");
  }
  return checked.size;
}

if (process.argv[1] === "-" || (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)) {
  try {
    const count = await verifyFrontend(process.env.FRONTEND_SMOKE_URL ?? "http://127.0.0.1:3000/portal/");
    console.log("Frontend deployment verified: " + count + " resources");
  } catch (error) {
    console.error("Frontend deployment verification failed:", error.message);
    process.exitCode = 1;
  }
}
