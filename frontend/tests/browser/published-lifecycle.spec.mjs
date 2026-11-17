/* global window, document */
import { readFileSync } from "node:fs";
import { URL } from "node:url";
import { test, expect } from "@playwright/test";

test("staged published lifecycle mounts with the host React runtime", async ({ page }) => {
  const failures = [];
  page.on("pageerror", (error) => failures.push(error.message));
  const imports = {
    ...JSON.parse(readFileSync(new URL("../../dist/import-map.json", import.meta.url))).imports,
    ...JSON.parse(readFileSync(new URL(
      "../../node_modules/.cache/browser-dependencies/import-map.json", import.meta.url,
    ))).imports,
  };
  await page.route("**/portal/__published-lifecycle.html", (route) => route.fulfill({
    contentType: "text/html",
    // Match the production host's environment shim, without its bundled React.
    body: `<!doctype html><script>globalThis.process={env:{NODE_ENV:"production"}};</script><script type="importmap">${JSON.stringify({ imports })}</script><div id="fixture"></div>`,
  }));
  await page.goto("/portal/__published-lifecycle.html");
  await page.evaluate(async () => {
    const React = await import("react");
    const { createReactMicrofrontendLifecycle } = await import("@moh-sso/microfrontend");
    window.testPublishedLifecycle = createReactMicrofrontendLifecycle(
      () => React.createElement("h1", null, "Published lifecycle mounted"),
      { withRedux: false, withModal: false, withToast: false, withHeaderPanel: false },
    );
    const props = { name: "published-fixture", domElement: document.getElementById("fixture") };
    await window.testPublishedLifecycle.bootstrap(props);
    await window.testPublishedLifecycle.mount(props);
  });
  await expect(page.getByRole("heading", { name: "Published lifecycle mounted" })).toBeVisible();
  await page.evaluate(async () => {
    await window.testPublishedLifecycle.unmount({
      name: "published-fixture", domElement: document.getElementById("fixture"),
    });
  });
  await expect(page.locator("#fixture")).toBeEmpty();
  expect(failures).toEqual([]);
});
