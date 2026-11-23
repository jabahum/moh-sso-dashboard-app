import { defineConfig } from "@playwright/test";

// The suites share one disposable stack and run in file order (01 contract,
// 02 sync, 03 browser, 04 removal). Each suite uses its own synthetic users, so
// a failure in one does not cascade into another.
export default defineConfig({
  testDir: "./tests",
  timeout: 240_000,
  expect: { timeout: 15_000 },
  workers: 1,
  fullyParallel: false,
  retries: 0,
  forbidOnly: true,
  reporter: [["list"], ["json", { outputFile: "results/report.json" }], ["html", { open: "never", outputFolder: "playwright-report" }]],
  use: {
    browserName: "chromium",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
