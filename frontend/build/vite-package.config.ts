import react from "@vitejs/plugin-react";
import { readFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

type PackageConfigOptions = {
  packageUrl: string;
  name: string;
};

export function definePackageConfig({ packageUrl, name }: PackageConfigOptions) {
  const pathFromPackage = (path: string) => fileURLToPath(new URL(path, packageUrl));
  const pathFromFrontend = (path: string) => fileURLToPath(new URL(`../../${path}`, packageUrl));
  const shell = JSON.parse(readFileSync(pathFromFrontend("apps/shell/package.json"), "utf8"));

  return defineConfig({
    define: {
      "import.meta.env.VITE_APP_VERSION": JSON.stringify(
        process.env.VITE_APP_VERSION || shell.version,
      ),
      "import.meta.env.VITE_BUILD_TIME": JSON.stringify(
        process.env.VITE_BUILD_TIME || new Date().toISOString(),
      ),
    },
    plugins: [react()],
    resolve: {
      alias: [
        { find: "@moh-sso/api", replacement: pathFromFrontend("packages/api/src") },
        { find: "@moh-sso/auth", replacement: pathFromFrontend("packages/auth/src") },
        { find: "@moh-sso/config", replacement: pathFromFrontend("packages/config/src") },
        {
          find: "@moh-sso/microfrontend",
          replacement: pathFromFrontend("packages/microfrontend/src"),
        },
        { find: "@moh-sso/state", replacement: pathFromFrontend("packages/state/src") },
        { find: "@moh-sso/types", replacement: pathFromFrontend("packages/types/src") },
        { find: "@moh-sso/ui", replacement: pathFromFrontend("packages/ui/src") },
        { find: "@moh-sso/utils", replacement: pathFromFrontend("packages/utils/src") },
      ],
    },
    build: {
      outDir: pathFromPackage("./dist"),
      emptyOutDir: true,
      lib: {
        entry: pathFromPackage("./src/index.ts"),
        name,
        formats: ["es"],
        fileName: () => "index.js",
      },
      rollupOptions: {
        external: [
          "react",
          "react-dom",
          "react-redux",
          "react-router-dom",
          "@reduxjs/toolkit",
          "@carbon/react",
          "@carbon/react/icons",
          "@moh-sso/api",
          "@moh-sso/auth",
          "@moh-sso/config",
          "@moh-sso/microfrontend",
          "@moh-sso/state",
          "@moh-sso/types",
          "@moh-sso/ui",
          "@moh-sso/utils",
        ],
      },
    },
  });
}
