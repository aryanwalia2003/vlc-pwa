// app-shell cache karo, media/api kabhi cache nahi (streaming/live)
const CACHE = "vlc-pwa-shell-v2";
const SHELL = [
  "/",
  "/app.js",
  "/app.css",
  "/manifest.json",
  "/icon.svg",
  "/icons/icon-180.png",
  "/icons/icon-192.png",
  "/icons/icon-512.png",
];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)));
  self.skipWaiting(); // naya SW turant activate ho
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (
    url.pathname.startsWith("/media/") ||
    url.pathname.startsWith("/api/") ||
    url.pathname.startsWith("/subs/")
  ) {
    return; // live data, cache se mat do
  }
  // network-first: shell hamesha fresh, offline me hi cache se do
  e.respondWith(
    fetch(e.request)
      .then((res) => {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(e.request, copy));
        return res;
      })
      .catch(() => caches.match(e.request))
  );
});
