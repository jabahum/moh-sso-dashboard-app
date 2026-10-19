/* global URL, fetch, AbortSignal, console */
import express from "express";
import { createProxyMiddleware } from "http-proxy-middleware";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = fileURLToPath(new URL("../", import.meta.url));
const scriptJson = (value) => JSON.stringify(value).replace(/</g, "\\u003c");

export function hostOptions(env = {}) {
  const moduleName = env.STANDALONE_MODULE?.trim();
  const backend = new URL(env.BACKEND_URL || "http://127.0.0.1:9000");
  if (!["http:", "https:"].includes(backend.protocol) || backend.username || backend.password) {
    throw new Error("BACKEND_URL must be an HTTP(S) URL without credentials");
  }
  return {
    distDir: resolve(frontendRoot, env.FRONTEND_DIST_DIR || "dist"),
    importMapSource: env.IMPORT_MAP_URL || env.IMPORT_MAP_FILE || "public/import-map.json",
    moduleName: moduleName
      ? moduleName.startsWith("@moh-sso/")
        ? moduleName
        : `@moh-sso/${moduleName}`
      : undefined,
    backend: backend.toString().replace(/\/$/, ""),
  };
}

async function readImportMap(source) {
  let value;
  const remote = /^https?:\/\//.test(source);
  if (remote) {
    const response = await fetch(source, { signal: AbortSignal.timeout(10000), cache: "no-store" });
    if (!response.ok) throw new Error(`Import map returned HTTP ${response.status}`);
    value = await response.json();
  } else {
    value = JSON.parse(await readFile(resolve(frontendRoot, source), "utf8"));
  }
  if (!value.imports || typeof value.imports !== "object" || Array.isArray(value.imports)) {
    throw new Error("Import map must contain an imports object");
  }
  // Remote map-relative URLs must resolve against the central map, not this host.
  function normalizeEntries(entries) {
    return Object.fromEntries(
      Object.entries(entries).map(([key, target]) => {
        if (typeof target !== "string") throw new Error(`Invalid import map entry: ${key}`);
        return [key, remote ? new URL(target, source).href : target];
      }),
    );
  }
  return {
    ...value,
    imports: normalizeEntries(value.imports),
    ...(value.scopes && {
      scopes: Object.fromEntries(
        Object.entries(value.scopes).map(([scope, entries]) => [
          remote ? new URL(scope, source).href : scope,
          normalizeEntries(entries),
        ]),
      ),
    }),
  };
}

export async function createHost(options = hostOptions()) {
  const { distDir, importMapSource, moduleName, backend } = options;
  const template = await readFile(resolve(distDir, "index.html"), "utf8");
  if (!template.includes("import-map:generated") && !/type=["']importmap["']/.test(template)) {
    throw new Error("Shell HTML has no import map placeholder. Rebuild the shell.");
  }
  const runtimeConfig = {
    hostRuntime: true,
    API_BASE_URL: "/backend",
    microfrontendMode: "remote",
    singleSpaOrchestration: !moduleName,
    microfrontendMountMode: moduleName ? "hybrid" : "orchestrated",
    ...(moduleName && { standaloneModule: moduleName }),
    disablePwa: true,
  };
  const app = express();
  app.enable("strict routing");
  app.disable("x-powered-by");
  app.use((_req, res, next) => {
    res.set("X-Content-Type-Options", "nosniff");
    next();
  });
  app.get("/healthz", (_req, res) => res.json({ status: "ok" }));
  // Mounting strips /backend; the backend target may include a deployment prefix.
  app.use(
    "/backend",
    createProxyMiddleware({
      target: backend,
      changeOrigin: true,
      cookieDomainRewrite: "",
      cookiePathRewrite: "/",
      proxyTimeout: 30000,
      timeout: 30000,
      on: {
        error(error, _req, res) {
          console.error("Backend proxy failed:", error.message);
          if (!res.headersSent) res.writeHead(502, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: "Backend unavailable" }));
        },
      },
    }),
  );
  app.get(
    ["/portal/config.js", "/portal/config.development.js", "/portal/config.production.js"],
    (_req, res) => {
      res
        .set("Cache-Control", "no-store")
        .type("js")
        .send(`window.__APP_CONFIG__ = ${scriptJson(runtimeConfig)};`);
    },
  );
  app.get("/portal/import-map.json", async (_req, res) => {
    res.set("Cache-Control", "no-store").json(await readImportMap(importMapSource));
  });
  app.get(["/", "/portal"], (_req, res) => res.redirect(302, "/portal/"));
  // Prevent a previously installed portal worker from serving stale HTML/maps.
  app.get("/portal/sw.js", (_req, res) => {
    res
      .set("Cache-Control", "no-store")
      .type("js")
      .send(
        'self.addEventListener("install",()=>self.skipWaiting());self.addEventListener("activate",event=>event.waitUntil(self.registration.unregister().then(()=>self.clients.matchAll()).then(clients=>Promise.all(clients.map(client=>client.navigate(client.url))))));',
      );
  });
  const sendShell = async (_req, res) => {
    const map = await readImportMap(importMapSource);
    if (moduleName && !map.imports[moduleName]) {
      throw new Error(`Import map has no entry for ${moduleName}`);
    }
    const mapScript = `<script type="importmap">${scriptJson(map)}</script>`;
    let html = template.replace(
      /<script\b[^>]*type=["']importmap["'][^>]*>[\s\S]*?<\/script>/g,
      "<!-- import-map:generated -->",
    );
    html = html.replace("<!-- import-map:generated -->", () => mapScript);
    // Inject before hostname-based config selection; config routes above return the same values.
    html = html.replace(
      "<head>",
      () => `<head><script>window.__APP_CONFIG__ = ${scriptJson(runtimeConfig)};</script>`,
    );
    res.set("Cache-Control", "no-store").type("html").send(html);
  };
  app.get(["/portal/", "/portal/index.html"], sendShell);
  const staticOptions = { index: false, redirect: false, maxAge: 0 };
  app.use("/portal", express.static(distDir, staticOptions));
  // Non-versioned default map entries are rooted at /mf and /packages.
  app.use("/mf", express.static(resolve(distDir, "mf"), staticOptions));
  app.use("/packages", express.static(resolve(distDir, "packages"), staticOptions));
  app.use("/portal", (req, res, next) => {
    if (
      !["GET", "HEAD"].includes(req.method) ||
      extname(req.path) ||
      /^\/(assets|mf|packages)(\/|$)/.test(req.path) ||
      !req.accepts("html")
    )
      return next();
    return sendShell(req, res).catch(next);
  });
  app.use((_req, res) => res.status(404).type("text").send("Not found"));
  app.use((error, _req, res, _next) => {
    void _next; // Express identifies error middleware by its four-argument signature.
    console.error("Frontend host failed:", error.message);
    res
      .status(503)
      .set("Cache-Control", "no-store")
      .type("text")
      .send("Frontend unavailable. Check the host configuration and import map.");
  });
  return app;
}
