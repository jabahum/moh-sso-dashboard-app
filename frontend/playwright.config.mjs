/* global process */
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/browser",
  timeout: 90_000,
  expect: { timeout: 30_000 },
  workers: 1,
  retries: 0,
  forbidOnly: Boolean(process.env.CI),
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:5417",
    browserName: "chromium",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command:
      "node scripts/prepare-browser-dependencies.mjs && python3 ../infra/scripts/serve-browser-fixture.py",
    url: "http://127.0.0.1:5417/",
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
