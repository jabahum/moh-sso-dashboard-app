/* global process, setTimeout */
import { stop } from "esbuild";

// Inline null overrides are discarded when Vite merges the project config.
// Set watch=null after resolution so even late addWatchFile calls are no-ops.
export function transformOnlyViteOptions() {
  return {
    server: { middlewareMode: true, ws: false, hmr: false, preTransformRequests: false },
    optimizeDeps: { noDiscovery: true, include: [] },
    plugins: [{
      name: "moh-transform-test-no-watch",
      configResolved(config) {
        config.server.watch = null;
        config.optimizeDeps.include = [];
        for (const environment of Object.values(config.environments)) {
          environment.optimizeDeps.include = [];
          environment.optimizeDeps.noDiscovery = true;
        }
      },
    }],
  };
}

// Vite closes its server, but its shared esbuild service can still retain a
// referenced worker/context. Standalone test processes own that service too.
export async function closeViteTestServer(server) {
  try {
    await server?.close();
  } finally {
    await stop();
  }
}

export function failOnOpenTestResources(timeoutMs = 10000) {
  // An unreferenced timer never delays a clean exit. If another resource keeps
  // this process alive, report it and fail rather than consuming the CI timeout.
  setTimeout(() => {
    process.stderr.write(
      `Npm consumption checks finished but resources remain open: ${process.getActiveResourcesInfo().join(", ")}\n`,
      () => process.exit(1),
    );
  }, timeoutMs).unref();
}
