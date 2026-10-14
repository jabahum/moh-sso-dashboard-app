import { runInNewContext } from "node:vm";

export function selectedRuntimeConfig(html, hostname) {
  const configured = html.match(
    /<script\b[^>]*\bsrc=["']([^"']*config(?:\.[a-z-]+)?\.js)["']/,
  )?.[1];
  if (configured) return configured.replace("%BASE_URL%", "/portal/");
  const written = [];
  const browser = {
    window: { location: { hostname } },
    document: { write: (value) => written.push(String(value)) },
  };
  for (const match of html.matchAll(/<script>\s*([\s\S]*?)<\/script>/g)) {
    runInNewContext(match[1], browser, { timeout: 1000 });
  }
  return written.join("").match(/src=["']([^"']+)["']/)?.[1];
}

export function runtimeConfig(source) {
  const browser = { window: {} };
  runInNewContext(source, browser, { timeout: 1000 });
  return browser.window.__APP_CONFIG__;
}
