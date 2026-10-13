// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  disablePush,
  enablePush,
  listPush,
  pushSupported,
  revokePush,
  urlBase64ToUint8Array,
} from './push'

// The module touches browser-only globals (Notification, service
// worker registration, fetch through the api wrapper); stub exactly
// those instead of emulating a real push stack.

const PUBLIC_KEY = 'A'.repeat(87) // 65-byte point, base64url — like vapidkeygen emits
const ENDPOINT = 'https://push.example.com/sub/1'
const KEYS = { p256dh: 'Bpk', auth: 'auth1' }

function fakeSub() {
  return {
    endpoint: ENDPOINT,
    toJSON: () => ({ endpoint: ENDPOINT, keys: KEYS }),
    unsubscribe: vi.fn(() => Promise.resolve(true)),
  }
}

type Route = { match: (url: string) => boolean; status: number; body?: unknown }

// Route fetch by URL predicate: push flows hit several endpoints in
// one call, so a single stubbed Response is not enough.
function mockFetch(routes: Route[]) {
  const calls: { url: string; init?: RequestInit }[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: string, init?: RequestInit) => {
      calls.push({ url, init })
      const route = routes.find((r) => r.match(String(url)))
      const headers = new Headers()
      if (route?.body !== undefined) headers.set('content-type', 'application/json')
      return Promise.resolve(
        new Response(route?.body === undefined ? null : JSON.stringify(route.body), {
          status: route?.status ?? 500,
          headers,
        }),
      )
    }),
  )
  return calls
}

function stubWorkerEnv(opts: {
  permission?: NotificationPermission
  sub?: ReturnType<typeof fakeSub> | null
  subscribeErr?: Error
}) {
  const subscribe = vi.fn((_options: PushSubscriptionOptions) =>
    opts.subscribeErr ? Promise.reject(opts.subscribeErr) : Promise.resolve(opts.sub ?? null),
  )
  const manager = { subscribe, getSubscription: () => Promise.resolve(opts.sub ?? null) }
  const registration = { pushManager: manager }
  const sw = {
    register: vi.fn(() => Promise.resolve(registration)),
    getRegistration: vi.fn(() => Promise.resolve(registration)),
  }
  vi.stubGlobal('navigator', { serviceWorker: sw })
  vi.stubGlobal('PushManager', {})
  vi.stubGlobal('Notification', {
    requestPermission: vi.fn(() => Promise.resolve(opts.permission ?? 'granted')),
  })
  return { sw, subscribe }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('urlBase64ToUint8Array', () => {
  it('pads and decodes a base64url key into bytes', () => {
    const bytes = urlBase64ToUint8Array(PUBLIC_KEY)
    expect(bytes).toHaveLength(65)
    expect(Array.from(bytes).every((b) => b === 0)).toBe(true)
  })

  it('maps the url-safe alphabet back to standard base64', () => {
    // '_-8' -> '/+8' -> 0xFF 0xEF
    expect(Array.from(urlBase64ToUint8Array('_-8'))).toEqual([255, 239])
  })
})

describe('pushSupported', () => {
  it('is false without a service worker container', () => {
    vi.stubGlobal('navigator', {})
    vi.stubGlobal('PushManager', {})
    expect(pushSupported()).toBe(false)
  })

  it('is true when both serviceWorker and PushManager exist', () => {
    stubWorkerEnv({})
    expect(pushSupported()).toBe(true)
  })
})

describe('enablePush', () => {
  it('runs permission, worker, key fetch, subscribe, and register in order', async () => {
    const sub = fakeSub()
    const { sw, subscribe } = stubWorkerEnv({ sub })
    const calls = mockFetch([
      {
        match: (u) => u.endsWith('/push/vapid-public-key'),
        status: 200,
        body: { publicKey: PUBLIC_KEY },
      },
      { match: (u) => u.endsWith('/push/subscriptions'), status: 201, body: { id: 's1' } },
    ])

    expect(await enablePush()).toEqual({ ok: true })
    expect(sw.register).toHaveBeenCalledWith('/sw.js')
    expect(subscribe).toHaveBeenCalledTimes(1)
    const args = subscribe.mock.calls[0][0]
    expect(args.userVisibleOnly).toBe(true)
    expect(args.applicationServerKey).toHaveLength(65)
    const posted = calls.find((c) => c.init?.method === 'POST')
    expect(posted?.url).toBe('/api/v1/push/subscriptions')
    expect(JSON.parse(String(posted?.init?.body))).toEqual({ endpoint: ENDPOINT, keys: KEYS })
  })

  it('refuses before asking permission when the browser cannot subscribe', async () => {
    vi.stubGlobal('navigator', {})
    const requestPermission = vi.fn()
    vi.stubGlobal('Notification', { requestPermission })

    expect(await enablePush()).toEqual({ ok: false, reason: '当前浏览器不支持 Web Push' })
    expect(requestPermission).not.toHaveBeenCalled()
  })

  it('stops when notification permission is denied', async () => {
    const { sw } = stubWorkerEnv({ permission: 'denied' })
    mockFetch([])

    expect(await enablePush()).toEqual({ ok: false, reason: '通知权限被拒绝' })
    expect(sw.register).not.toHaveBeenCalled()
  })

  it('maps a 404 public key to the server-disabled reason', async () => {
    stubWorkerEnv({ sub: fakeSub() })
    mockFetch([{ match: (u) => u.includes('vapid-public-key'), status: 404, body: null }])

    expect(await enablePush()).toEqual({ ok: false, reason: '服务端未开启推送' })
  })

  it('unsubscribes locally when the server rejects the registration', async () => {
    const sub = fakeSub()
    stubWorkerEnv({ sub })
    mockFetch([
      {
        match: (u) => u.includes('vapid-public-key'),
        status: 200,
        body: { publicKey: PUBLIC_KEY },
      },
      { match: (u) => u.endsWith('/push/subscriptions'), status: 500, body: null },
    ])

    expect(await enablePush()).toEqual({ ok: false, reason: '注册推送失败' })
    expect(sub.unsubscribe).toHaveBeenCalled()
  })
})

describe('endpoint management', () => {
  it('listPush returns the registered rows', async () => {
    const rows = [
      { id: 's1', user_id: 'u1', endpoint: ENDPOINT, created_at: '2026-10-13T00:00:00Z' },
    ]
    mockFetch([{ match: (u) => u.endsWith('/push/subscriptions'), status: 200, body: { items: rows } }])

    expect(await listPush()).toEqual(rows)
  })

  it('revokePush deletes by id', async () => {
    const calls = mockFetch([{ match: (u) => u.endsWith('/push/subscriptions/s7'), status: 204 }])

    await revokePush('s7')
    expect(calls[0].url).toBe('/api/v1/push/subscriptions/s7')
    expect(calls[0].init?.method).toBe('DELETE')
  })
})

describe('disablePush', () => {
  it('returns false when this browser never subscribed', async () => {
    stubWorkerEnv({ sub: null })
    expect(await disablePush()).toBe(false)
  })

  it('deletes the server row matched by endpoint, then unsubscribes', async () => {
    const sub = fakeSub()
    stubWorkerEnv({ sub })
    const calls = mockFetch([
      {
        match: (u) => u.endsWith('/push/subscriptions') && !u.includes('/s9'),
        status: 200,
        body: {
          items: [
            { id: 's9', user_id: 'u1', endpoint: 'https://other.example.com/x', created_at: '' },
            { id: 's1', user_id: 'u1', endpoint: ENDPOINT, created_at: '' },
          ],
        },
      },
      { match: (u) => u.endsWith('/push/subscriptions/s1'), status: 204 },
    ])

    expect(await disablePush()).toBe(true)
    const deleted = calls.find((c) => c.init?.method === 'DELETE')
    expect(deleted?.url).toBe('/api/v1/push/subscriptions/s1')
    expect(sub.unsubscribe).toHaveBeenCalled()
  })

  it('still unsubscribes when the server has no row for this endpoint', async () => {
    const sub = fakeSub()
    stubWorkerEnv({ sub })
    mockFetch([
      { match: (u) => u.endsWith('/push/subscriptions'), status: 200, body: { items: [] } },
    ])

    expect(await disablePush()).toBe(false)
    expect(sub.unsubscribe).toHaveBeenCalled()
  })
})
