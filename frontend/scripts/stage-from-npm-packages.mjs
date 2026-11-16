/* global process */
process.env.FRONTEND_MODULE_SOURCE = "npm";
await import("./stage-microfrontends.mjs");
