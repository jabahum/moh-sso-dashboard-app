/* global self, caches */
// Retired service worker for the old /portal/ scope. Browsers that installed the
// /portal/ PWA fetch this on their next update check; it clears that worker's
// precache (Workbox names it after the /portal/ scope; root caches are kept), unregisters itself and reloads open /portal/ tabs so they follow the
// server redirect to the root-served shell.
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(keys.filter((key) => key.includes("/portal/")).map((key) => caches.delete(key)));
      await self.registration.unregister();
      const clients = await self.clients.matchAll({ type: "window" });
      for (const client of clients) client.navigate(client.url);
    })(),
  );
});
