import react from "@vitejs/plugin-react";
import { existsSync, readdirSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig, type Alias } from "vite";
import { visualizer } from "rollup-plugin-visualizer";
import cssInjectedByJsPlugin from "./css-injected-by-js";

type MicrofrontendConfigOptions = {
  appUrl: string;
  name: string;
  extraAliases?: Alias[];
  extraExternal?: string[];
};

export function defineMicrofrontendConfig({
  appUrl,
  name,
  extraAliases = [],
  extraExternal = [],
}: MicrofrontendConfigOptions) {
  const pathFromApp = (path: string) => fileURLToPath(new URL(path, appUrl));

  const pathFromFrontend = (path: string) => fileURLToPath(new URL(`../../${path}`, appUrl));
  const getDirectories = (path: string) =>
    readdirSync(pathFromFrontend(path), { withFileTypes: true })
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name);

  const apps = getDirectories("apps");
  const packages = getDirectories("packages");

  const appEntries: Record<string, string> = {
    index: pathFromApp("./src/index.ts"),
    routes: pathFromApp("./src/routes.tsx"),
    "single-spa": pathFromApp("./src/single-spa.tsx"),
  };

  if (existsSync(pathFromApp("./src/api/index.ts"))) {
    appEntries.api = pathFromApp("./src/api/index.ts");
  }

  if (existsSync(pathFromApp("./src/types/index.ts"))) {
    appEntries.types = pathFromApp("./src/types/index.ts");
  }

  return defineConfig(({ mode }) => {
    const isProduction = mode === "production";
    const analyzeBundle = process.env.ANALYZE_BUNDLE === "true";

    return {
      plugins: [
        react(),

        // The portal imports only single-spa.js through the import map.
        // Inject each micro-frontend's extracted CSS into that JS bundle.
        cssInjectedByJsPlugin({
          topExecutionPriority: false,
        }),

        analyzeBundle &&
          visualizer({
            open: false,
            filename: "bundle-analysis.html",
            gzipSize: true,
            brotliSize: true,
          }),
      ].filter(Boolean),

      define: {
        "process.env.NODE_ENV": JSON.stringify(isProduction ? "production" : "development"),
      },

      resolve: {
        dedupe: [
          "react",
          "react-dom",
          "react-router-dom",
          "react-redux",
          "@carbon/react",
          "single-spa",
          "single-spa-react",
        ],

        alias: [
          ...packages.map((pkg) => ({
            find: new RegExp(`^@moh-sso/${pkg}/(.+)$`),
            replacement: pathFromFrontend(`packages/${pkg}/src/$1`),
          })),
          ...packages.map((pkg) => ({
            find: `@moh-sso/${pkg}`,
            replacement: pathFromFrontend(`packages/${pkg}/src/index.ts`),
          })),
          ...apps.map((app) => ({
            find: `@moh-sso/${app}/single-spa`,
            replacement: pathFromFrontend(`apps/${app}/src/single-spa.tsx`),
          })),
          ...apps.map((app) => ({
            find: new RegExp(`^@moh-sso/${app}/(.+)$`),
            replacement: pathFromFrontend(`apps/${app}/src/$1`),
          })),
          ...apps.map((app) => ({
            find: `@moh-sso/${app}`,
            replacement: pathFromFrontend(`apps/${app}/src/index.ts`),
          })),
          ...extraAliases,
        ],
      },

      build: {
        target: "es2020",
        outDir: pathFromApp("./dist"),
        emptyOutDir: true,
        sourcemap: true,

        commonjsOptions: {
          include: [/react-pivottable/, /node_modules/],
          transformMixedEsModules: true,
        },

        lib: {
          entry: appEntries,
          name,
          formats: ["es"],
          fileName: (_format, entryName) => `${entryName}.js`,
        },

        rollupOptions: {
          external: [
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
            ...packages.map((pkg) => `@moh-sso/${pkg}`),
            ...extraExternal,
          ],

          output: {
            entryFileNames: "[name].js",
            chunkFileNames: "assets/[name]-[hash].js",
            assetFileNames: "assets/[name]-[hash][extname]",
          },
        },
      },
    };
  });
}
