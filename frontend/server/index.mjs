/* global process, console, setTimeout */
import { createHost, hostOptions } from "./host.mjs";

const app = await createHost(hostOptions(process.env));
const port = Number(process.env.PORT || 3000);
const host = process.env.HOST || "127.0.0.1";
const server = app.listen(port, host, (error) => {
  if (error) {
    console.error(`Unable to start frontend host: ${error.message}`);
    process.exitCode = 1;
    return;
  }
  console.log(`Frontend host listening at http://${host}:${port}/portal/`);
});
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.once(signal, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(1), 10000).unref();
  });
}
