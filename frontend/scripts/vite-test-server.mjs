/* global process, setTimeout */
import { stop } from "esbuild";

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
