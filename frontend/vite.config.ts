import { createHash } from "node:crypto";
import react from "@vitejs/plugin-react";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, loadEnv } from "vite";
import { VitePWA } from "vite-plugin-pwa";
import { moduleSelection, npmModulePlugin } from "./scripts/module-source.mjs";

const pathFromRoot = (path: string) => fileURLToPath(new URL(path, import.meta.url));

function readEnvLocalOverrides(localEnvPath: string): Record<string, string> {
  if (!existsSync(localEnvPath)) {
    return {};
  }
  const overrides: Record<string, string> = {};
  for (const rawLine of readFileSync(localEnvPath, "utf8").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }
    const separator = line.indexOf("=");
    if (separator <= 0) {
      continue;
    }
    const key = line.slice(0, separator).trim();
    let value = line.slice(separator + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    overrides[key] = value;
  }
  return overrides;
}

function normalizeVisualizerProxyCookie(rawCookie?: string): string | undefined {
  const trimmed = rawCookie?.trim();
  if (!trimmed) {
    return undefined;
  }
  if (/^bearer\s+/i.test(trimmed)) {
    return `access_token=${trimmed.replace(/^bearer\s+/i, "").trim()}`;
  }
  if (trimmed.includes("=")) {
    return trimmed;
  }
  if (trimmed.startsWith("eyJ") && trimmed.split(".").length === 3) {
    return `access_token=${trimmed}`;
  }
  return `sso_session=${trimmed}`;
}

function parseCookieMap(cookieHeader?: string): Map<string, string> {
  const map = new Map<string, string>();
  if (!cookieHeader) {
    return map;
  }
  for (const part of cookieHeader.split(";")) {
    const segment = part.trim();
    const eq = segment.indexOf("=");
    if (eq > 0) {
      map.set(segment.slice(0, eq).trim(), segment.slice(eq + 1).trim());
    }
  }
  return map;
}

function mergeCookies(existingCookie?: string, extraCookie?: string): string | undefined {
  if (!existingCookie && !extraCookie) {
    return undefined;
  }
  if (!existingCookie) {
    return extraCookie;
  }
  if (!extraCookie) {
    return existingCookie;
  }
  const merged = new Map<string, string>();
  for (const source of [extraCookie, existingCookie]) {
    for (const [key, value] of parseCookieMap(source).entries()) {
      merged.set(key, value);
    }
  }
  return Array.from(merged.entries())
    .map(([key, value]) => `${key}=${value}`)
    .join("; ");
}

const sessionAccessTokenCache = new Map<string, { token: string; expiresAt: number }>();

async function resolveLocalSessionAccessToken(sessionId: string): Promise<string | undefined> {
  const cached = sessionAccessTokenCache.get(sessionId);
  if (cached && cached.expiresAt > Date.now()) {
    return cached.token;
  }

  const localBackends = ["http://backend:9000", "http://127.0.0.1:9000", "http://localhost:9000"];
  for (const base of localBackends) {
    try {
      let res = await fetch(`${base}/api/v1/auth/me`, {
        method: "GET",
        headers: { Cookie: `sso_session=${sessionId}` },
      });
      if (res.status === 401) {
        res = await fetch(`${base}/api/v1/auth/refresh`, {
          method: "POST",
          headers: { Cookie: `sso_session=${sessionId}` },
        });
      }
      const setCookies =
        typeof res.headers.getSetCookie === "function"
          ? res.headers.getSetCookie()
          : [res.headers.get("set-cookie") ?? ""];
      for (const header of setCookies) {
        const match = header.match(/(?:^|;\s*)access_token=([^;]+)/);
        if (match?.[1]) {
          const token = decodeURIComponent(match[1].trim());
          sessionAccessTokenCache.set(sessionId, {
            token,
            expiresAt: Date.now() + 60_000,
          });
          return token;
        }
      }
      if (res.ok) {
        break;
      }
    } catch {
      // Try next local backend candidate
    }
  }
  return undefined;
}

export function rewriteVisualizerProxyPath(path: string, target: string): string {
  const stripped = path.replace(/^\/visualizer[-_]proxy/, "") || "/";
  const normalizedPath = stripped.startsWith("/") ? stripped : `/${stripped}`;
  const cleanTarget = target.trim().replace(/\/+$/, "");

  if (cleanTarget.endsWith("/api/v1")) {
    if (
      normalizedPath === "/api/v1" ||
      normalizedPath.startsWith("/api/v1/") ||
      normalizedPath.startsWith("/api/v1?")
    ) {
      return normalizedPath.slice("/api/v1".length) || "/";
    }
    return normalizedPath;
  }

  if (cleanTarget.endsWith("/api")) {
    if (
      normalizedPath === "/api/v1" ||
      normalizedPath.startsWith("/api/v1/") ||
      normalizedPath.startsWith("/api/v1?")
    ) {
      return normalizedPath.slice("/api".length);
    }
    if (
      normalizedPath === "/v1" ||
      normalizedPath.startsWith("/v1/") ||
      normalizedPath.startsWith("/v1?")
    ) {
      return normalizedPath;
    }
    return `/v1${normalizedPath}`;
  }

  if (normalizedPath === "/api" || normalizedPath.startsWith("/api/")) {
    return normalizedPath;
  }

  return `/api/v1${normalizedPath}`;
}

const getDirectories = (source: string) =>
  readdirSync(pathFromRoot(source), { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name);

const apps = getDirectories("./apps");
const packages = getDirectories("./packages");
const shell = JSON.parse(readFileSync(pathFromRoot("./apps/shell/package.json"), "utf8"));

const dynamicAliases = [
  ...packages.map((pkg) => ({
    find: new RegExp(`^@moh-sso/${pkg}/(.+)$`),
    replacement: pathFromRoot(`./packages/${pkg}/src/$1`),
  })),

  ...packages.map((pkg) => ({
    find: `@moh-sso/${pkg}`,
    replacement: pathFromRoot(`./packages/${pkg}/src/index.ts`),
  })),

  ...apps.map((app) => ({
    find: `@moh-sso/${app}/single-spa`,
    replacement: pathFromRoot(`./apps/${app}/src/single-spa.tsx`),
  })),

  ...apps.map((app) => ({
    find: new RegExp(`^@moh-sso/${app}/(.+)$`),
    replacement: pathFromRoot(`./apps/${app}/src/$1`),
  })),

  ...apps.map((app) => ({
    find: `@moh-sso/${app}`,
    replacement: pathFromRoot(`./apps/${app}/src/index.ts`),
  })),
];

export default defineConfig(({ mode }) => {
  const envDir = pathFromRoot(".");
  const env = {
    ...loadEnv(mode, envDir, ""),
    ...loadEnv(mode, process.cwd(), ""),
    ...readEnvLocalOverrides(pathFromRoot("./.env")),
    ...readEnvLocalOverrides(pathFromRoot("./.env.local")),
  };
  const modules = moduleSelection(
    mode === "production" ? "production" : "development",
    { ...env, ...process.env },
  );
  const visualizerProxyTarget =
    env.VITE_VISUALIZER_PROXY_TARGET?.trim() ||
    (env.VITE_VISUALIZER_API_BASE?.trim()
      ? env.VITE_API_BASE_URL?.trim() || "http://localhost:9000"
      : undefined);
  const normalizedVisualizerCookie = normalizeVisualizerProxyCookie(
    env.VITE_VISUALIZER_PROXY_COOKIE,
  );
  const cleanVisualizerTarget = visualizerProxyTarget?.replace(/\/+$/, "");
  const visualizerProxyEntry = cleanVisualizerTarget
    ? {
        target: cleanVisualizerTarget,
        changeOrigin: true,
        secure: false,
        rewrite: (path: string) => rewriteVisualizerProxyPath(path, cleanVisualizerTarget),
        cookieDomainRewrite: {
          "dashboards.health.go.ug": "localhost",
        },
      }
    : undefined;

  return {
    cacheDir: pathFromRoot(`./node_modules/.vite/${modules.environment}-${modules.source}-${createHash("sha256").update(JSON.stringify(modules.versions)).digest("hex").slice(0, 12)}`),
    base: "/portal/",
    root: pathFromRoot("./apps/shell"),
    envDir,
    publicDir: pathFromRoot("./public"),

  plugins: [
    ...(modules.source === "npm" ? [npmModulePlugin(modules)] : []),
    {
      name: "portal-base-redirect",
      configureServer(server) {
        server.middlewares.use(async (request, response, next) => {
          const match = request.url?.match(/^\/portal(\?.*)?$/);
          if (match) {
            response.writeHead(302, { Location: `/portal/${match[1] ?? ""}` });
            response.end();
            return;
          }

          if (request.url && /^\/visualizer[-_]proxy(?:\/|\?|$)/.test(request.url)) {
            const browserSessionId = parseCookieMap(request.headers.cookie).get("sso_session");
            const fallbackSessionId = parseCookieMap(normalizedVisualizerCookie).get("sso_session");
            const mergedCookie = mergeCookies(
              request.headers.cookie,
              normalizedVisualizerCookie,
            );
            if (mergedCookie) {
              request.headers.cookie = mergedCookie;
            }
            const cookies = parseCookieMap(request.headers.cookie);
            let accessToken = cookies.get("access_token");
            const sessionCandidates = Array.from(
              new Set([browserSessionId, fallbackSessionId, cookies.get("sso_session")].filter(Boolean)),
            ) as string[];
            for (const sessionId of sessionCandidates) {
              if (accessToken) {
                break;
              }
              accessToken = await resolveLocalSessionAccessToken(sessionId);
              if (accessToken) {
                request.headers.cookie = mergeCookies(
                  request.headers.cookie,
                  `access_token=${accessToken}`,
                );
              }
            }
            if (accessToken && !request.headers.authorization) {
              request.headers.authorization = `Bearer ${accessToken}`;
            }
          }

          next();
        });
      },
    },
    react(),
    VitePWA({
      base: "/portal/",
      scope: "/portal/",
      registerType: "prompt",
      manifest: false,
      devOptions: {
        enabled: false,
      },
      includeAssets: [
        "logo.png",
        "manifest.webmanifest",
        "offline.html",
        "config.js",
        "config.production.js",
        "config.development.js",
        "config.local-remote.js",
        "import-map.json",
        "import-map.local.json",
        "version-manifest.json",
        "icons/icon-192.png",
        "icons/icon-512.png",
        "icons/maskable-192.png",
        "icons/maskable-512.png",
      ],
      workbox: {
        navigateFallback: "/portal/index.html",
        globPatterns: ["**/*.{js,css,html,ico,png,svg,webmanifest,json}"],
        navigateFallbackDenylist: [
          /^\/api\//,
          /^\/realms\//,
          /^\/auth\//,
          /^\/visualizer[-_]proxy\//,
        ],
        maximumFileSizeToCacheInBytes: 6 * 1024 * 1024,
        runtimeCaching: [
          {
            urlPattern: ({ request, url }) =>
              request.destination === "script" && url.pathname.startsWith("/portal/assets/"),
            handler: "StaleWhileRevalidate",
            options: {
              cacheName: "portal-static-assets",
              expiration: {
                maxEntries: 80,
                maxAgeSeconds: 60 * 60 * 24 * 30,
              },
            },
          },
          {
            urlPattern: ({ url }) =>
              url.pathname.startsWith("/portal/mf/") ||
              url.pathname.startsWith("/portal/packages/"),
            handler: "NetworkFirst",
            options: {
              cacheName: "portal-microfrontends",
              networkTimeoutSeconds: 5,
              expiration: {
                maxEntries: 120,
                maxAgeSeconds: 60 * 60 * 24,
              },
            },
          },
        ],
      },
    }),
  ],

    define: {
      "import.meta.env.VITE_APP_VERSION": JSON.stringify(env.VITE_APP_VERSION || shell.version),
      "import.meta.env.VITE_BUILD_TIME": JSON.stringify(env.VITE_BUILD_TIME || new Date().toISOString()),
      "process.env.NODE_ENV": JSON.stringify(mode === "production" ? "production" : "development"),
    },

    resolve: {
      dedupe: [
        "react",
        "react-dom",
        "react-router-dom",
        "react-redux",
        "@reduxjs/toolkit",
        "@carbon/react",
        "single-spa",
        "single-spa-react",
      ],

      alias: [
        ...(modules.source === "workspace" ? [
          { find: "@/config", replacement: pathFromRoot("./packages/config/src") },
          { find: "@/types", replacement: pathFromRoot("./packages/types/src/global") },
          ...dynamicAliases,
        ] : []),
        {
          find: "@",
          replacement: pathFromRoot("./apps/shell/src"),
        },
      ],
    },

    server: {
      host: true,
      port: 3000,
      proxy: visualizerProxyEntry
        ? {
            "/visualizer-proxy": visualizerProxyEntry,
            "/visualizer_proxy": visualizerProxyEntry,
          }
        : undefined,
    },

    optimizeDeps: {
      include: [
        "react",
        "react-dom",
        "react-dom/client",
        "react/jsx-runtime",
        "react/jsx-dev-runtime",
        "react-redux",
        "react-router-dom",
        "@reduxjs/toolkit",
        "@carbon/react",
        "@carbon/react/icons",
        "single-spa",
        "single-spa-react",
        "react-pivottable/PivotTableUI",
      ],
    },

    build: {
      outDir: pathFromRoot("./apps/shell/dist"),
      emptyOutDir: true,
      sourcemap: true,

      commonjsOptions: {
        include: [/react-pivottable/, /node_modules/],
        transformMixedEsModules: true,
      },
    },
  };
});
