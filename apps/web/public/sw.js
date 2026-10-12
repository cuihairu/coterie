// Web Push service worker (design D22): shows the notification the
// server encrypted; the decrypted payload is {"title","body"}.
self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data.json()
  } catch {
    // Undecryptable or empty payload: still surface a visible notice,
    // userVisibleOnly promised one.
  }
  event.waitUntil(
    event.registration.showNotification(data.title || 'Coterie', {
      body: data.body || '',
      icon: '/favicon.svg',
      tag: 'coterie',
    }),
  )
})
