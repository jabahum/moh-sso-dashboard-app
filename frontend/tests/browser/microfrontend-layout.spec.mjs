/* global window, document */
import { readFileSync } from "node:fs";
import { URL } from "node:url";
import { test, expect } from "@playwright/test";
import ts from "typescript";
import * as sass from "sass";

const containerSource = readFileSync(
  new URL("../../apps/shell/src/app/microfrontends/containers.ts", import.meta.url),
  "utf8",
);
const containerScript = ts.transpileModule(containerSource, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const layoutCSS = sass.compileString(
  readFileSync(
    new URL("../../apps/shell/src/app/layouts/user/user-layout.scss", import.meta.url),
    "utf8",
  ),
).css;

async function shell(page) {
  await page.setContent(`
    <style>
      body { margin: 0; }
      * { box-sizing: border-box; }
      ${layoutCSS}
    </style>
    <div class="user-layout">
      <header style="position:fixed;height:3rem;top:0">Portal header</header>
      <div class="user-layout__body">
        <aside class="cds--side-nav" style="position:fixed">Navigation</aside>
        <div class="user-layout__content-shell">
          <main class="user-layout__content cds--content" id="main-content"></main>
        </div>
      </div>
      <footer>Portal footer</footer>
    </div>
  `);
  await page.addScriptTag({
    content: `(() => { const exports = {}; ${containerScript}; window.testContainers = exports; })();`,
  });
}

test("early orchestrated content attaches near the header inside the shell", async ({ page }) => {
  await shell(page);
  const earlyMount = await page.evaluate(() => {
    const container = window.testContainers.getMicrofrontendContainer("@moh-sso/documents");
    container.innerHTML = '<h1 style="margin:0">Document Management</h1>';
    return {
      connected: container.isConnected,
      bodyFallback: !!document.getElementById("microfrontend-orchestrated-root"),
    };
  });
  expect(earlyMount).toEqual({ connected: false, bodyFallback: false });

  await page.evaluate(() => {
    const slot = document.createElement("div");
    document.getElementById("main-content").appendChild(slot);
    window.testContainers.attachMicrofrontendContainer("@moh-sso/documents", slot);
  });
  const heading = page.getByRole("heading", { name: "Document Management" });
  await expect(heading).toBeVisible();
  await expect(page.locator("#main-content h1")).toHaveCount(1);
  const bounds = await heading.boundingBox();
  expect(bounds.y).toBeGreaterThanOrEqual(48);
  expect(bounds.y).toBeLessThan(160);
  expect(bounds.x).toBeGreaterThanOrEqual(248);
  await expect(page.locator("[id^='single-spa-application-']")).toHaveCount(1);
});

test("route replacement preserves the lifecycle container without leaving body content", async ({
  page,
}) => {
  await shell(page);
  const result = await page.evaluate(() => {
    const api = window.testContainers;
    const main = document.getElementById("main-content");
    const documents = api.getMicrofrontendContainer("@moh-sso/documents");
    documents.textContent = "Documents";
    const first = document.createElement("div");
    main.appendChild(first);
    api.attachMicrofrontendContainer("@moh-sso/documents", first);
    first.remove();
    const preserved = api.getMicrofrontendContainer("@moh-sso/documents") === documents;
    const detachedDuringUnmount = !documents.isConnected;
    const users = api.getMicrofrontendContainer("@moh-sso/users");
    users.textContent = "Users";
    const second = document.createElement("div");
    main.appendChild(second);
    api.attachMicrofrontendContainer("@moh-sso/users", second);
    const usersInsideShell = main.contains(users) && !documents.isConnected;
    second.remove();
    const third = document.createElement("div");
    main.appendChild(third);
    api.attachMicrofrontendContainer("@moh-sso/documents", third);
    api.attachMicrofrontendContainer("@moh-sso/documents", third);
    return {
      preserved,
      detachedDuringUnmount,
      usersInsideShell,
      content: third.textContent,
      childCount: third.children.length,
      directBodyModule: !!document.querySelector("body > [data-microfrontend]"),
    };
  });
  expect(result).toEqual({
    preserved: true,
    detachedDuringUnmount: true,
    usersInsideShell: true,
    content: "Documents",
    childCount: 1,
    directBodyModule: false,
  });
});
