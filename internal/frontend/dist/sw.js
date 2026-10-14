/* Tayga web client — GPL-3.0-only. No mailbox content is cached here. */
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()));
self.addEventListener('push', event => {
  event.waitUntil((async () => {
    let data;
    try { data = event.data.json(); } catch (_) { return; }
    const windows = await self.clients.matchAll({type: 'window', includeUncontrolled: true});
    if (windows.some(window => window.visibilityState === 'visible')) return;
    const href = ['mail', 'calendar', 'chat'].includes(data.href) ? data.href : 'mail';
    await self.registration.showNotification(data.title || 'Tayga Mail', {
      tag: data.tag || 'tayga', data: {href}, body: 'Откройте Tayga Mail для просмотра.'
    });
  })());
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const href = event.notification.data?.href || 'mail';
    const windows = await self.clients.matchAll({type:'window', includeUncontrolled:true});
    for (const window of windows) {
      if (new URL(window.url).origin === self.location.origin) {
        await window.focus(); window.postMessage({type:'tayga-notification', href}); return;
      }
    }
    await self.clients.openWindow('/?notification=' + encodeURIComponent(href));
  })());
});
