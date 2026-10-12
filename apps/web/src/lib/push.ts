// Browser plumbing for Web Push (design D22). The server side is
// three endpoints (register / list / delete + the VAPID public key);
// this module hides the enable dance — permission, service worker,
// subscribe, register — behind one call, and mirrors the Go DTOs
// (internal/push/model.go, http.go).

import { api, ApiError } from './api'

// PushSubscriptionInfo is one row of GET /push/subscriptions; the key
// material never leaves the server (json:"-").
export interface PushSubscriptionInfo {
  id: string
  user_id: string
  endpoint: string
  created_at: string
}

export type EnableResult = { ok: true } | { ok: false; reason: string }

// pushSupported reports whether this browser can subscribe at all —
// no service worker or no PushManager (e.g. iOS Safari outside the
// home screen) means the UI should not offer the toggle.
export function pushSupported(): boolean {
  return typeof navigator !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window
}

// urlBase64ToUint8Array converts the VAPID public key (base64url, no
// padding) into the bytes applicationServerKey expects. The explicit
// ArrayBuffer keeps the view typed as ArrayBuffer-backed, which is
// what BufferSource demands.
export function urlBase64ToUint8Array(b64url: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (b64url.length % 4)) % 4)
  const base64 = (b64url + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  const bytes = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i)
  return bytes
}

function toSubscriptionBody(sub: PushSubscription): {
  endpoint: string
  keys: { p256dh: string; auth: string }
} {
  const json = sub.toJSON()
  const keys = json.keys as { p256dh?: string; auth?: string } | undefined
  if (!json.endpoint || !keys?.p256dh || !keys.auth) {
    throw new Error('subscription is missing endpoint or keys')
  }
  return { endpoint: json.endpoint, keys: { p256dh: keys.p256dh, auth: keys.auth } }
}

// activePushSubscription returns the browser-side subscription for this
// origin, or null when the user never enabled push here.
export async function activePushSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null
  const reg = await navigator.serviceWorker.getRegistration()
  if (!reg) return null
  return reg.pushManager.getSubscription()
}

// enablePush runs the whole flow: ask for notification permission,
// register /sw.js, fetch the server's VAPID key, subscribe, and hand
// the subscription to the server. Every failure comes back as a
// user-facing reason instead of throwing, so the toggle can render it.
export async function enablePush(): Promise<EnableResult> {
  if (!pushSupported()) return { ok: false, reason: '当前浏览器不支持 Web Push' }
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') return { ok: false, reason: '通知权限被拒绝' }

  const reg = await navigator.serviceWorker.register('/sw.js')
  let publicKey: string
  try {
    publicKey = (await api<{ publicKey: string }>('/push/vapid-public-key')).publicKey
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return { ok: false, reason: '服务端未开启推送' }
    }
    return { ok: false, reason: '获取推送公钥失败' }
  }

  let sub: PushSubscription
  try {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(publicKey),
    })
  } catch {
    return { ok: false, reason: '订阅被浏览器拒绝' }
  }

  try {
    await api<PushSubscriptionInfo>('/push/subscriptions', {
      method: 'POST',
      body: JSON.stringify(toSubscriptionBody(sub)),
    })
  } catch {
    await sub.unsubscribe().catch(() => undefined)
    return { ok: false, reason: '注册推送失败' }
  }
  return { ok: true }
}

// disablePush removes the server row for this browser's subscription
// (matched by endpoint — the server id is not known locally) and then
// unsubscribes locally. Returns false when there was nothing to undo.
export async function disablePush(): Promise<boolean> {
  const sub = await activePushSubscription()
  if (!sub) return false
  const { items } = await api<{ items: PushSubscriptionInfo[] }>('/push/subscriptions')
  const row = items.find((s) => s.endpoint === sub.endpoint)
  if (row) await api<void>(`/push/subscriptions/${row.id}`, { method: 'DELETE' })
  await sub.unsubscribe()
  return row !== undefined
}
