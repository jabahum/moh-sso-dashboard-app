import react from "@vitejs/plugin-react";
import { readdirSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, loadEnv } from "vite";
import { VitePWA } from "vite-plugin-pwa";

const pathFromRoot = (path: string) => fileURLToPath(new URL(path, import.meta.url));

const getDirectories = (source: string) =>
  readdirSync(pathFromRoot(source), { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name);

const apps = getDirectories("./apps");
const packages = getDirectories("./packages");

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
  const env = loadEnv(mode, process.cwd(), "");
  const visualizerProxyTarget = env.VITE_VISUALIZER_PROXY_TARGET?.trim();
  const visualizerProxyCookie = env.VITE_VISUALIZER_PROXY_COOKIE?.trim();

  return {
    base: "/portal/",
    root: pathFromRoot("./apps/shell"),
    publicDir: pathFromRoot("./public"),

  plugins: [
    {
      name: "portal-base-redirect",
      configureServer(server) {
        server.middlewares.use((request, response, next) => {
          const match = request.url?.match(/^\/portal(\?.*)?$/);
          if (!match) {
            next();
            return;
          }

          response.writeHead(302, { Location: `/portal/${match[1] ?? ""}` });
          response.end();
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
        navigateFallbackDenylist: [/^\/api\//, /^\/realms\//, /^\/auth\//],
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
        {
          find: "@/config",
          replacement: pathFromRoot("./packages/config/src"),
        },
        {
          find: "@/types",
          replacement: pathFromRoot("./packages/types/src/global"),
        },
        ...dynamicAliases,
        {
          find: "@",
          replacement: pathFromRoot("./apps/shell/src"),
        },
      ],
    },

    server: {
      host: true,
      port: 3000,
      proxy: visualizerProxyTarget
        ? {
            "/visualizer-proxy": {
              target: visualizerProxyTarget,
              changeOrigin: true,
              secure: false,
              rewrite: (path) => path.replace(/^\/visualizer-proxy/, ""),
              cookieDomainRewrite: {
                "dashboards.health.go.ug": "localhost",
              },
              headers: visualizerProxyCookie
                ? {
                    Cookie: visualizerProxyCookie,
                  }
                : undefined,
            },
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
