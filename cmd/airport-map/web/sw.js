// The airport map's service worker: it makes the page installable as an
// app (tablets working a position). Nothing is cached: the map is live,
// every request goes to the map server.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (e) => e.waitUntil(self.clients.claim()));
self.addEventListener('fetch', () => { /* the network, as without it */ });
