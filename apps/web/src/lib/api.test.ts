import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, logout, saveSession } from './api'
import type { User } from './api'

// api.ts touches the browser at call time; stub just those globals so
// the wrapper can be exercised without a DOM.
const store = new Map<string, string>()
const locations: string[] = []

beforeEach(() => {
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => (store.has(key) ? store.get(key) : null),
    setItem: (key: string, value: string) => void store.set(key, String(value)),
    removeItem: (key: string) => void store.delete(key),
  })
  vi.stubGlobal('window', { location: { assign: (path: string) => void locations.push(path) } })
})

type Seen = { init: RequestInit | undefined; url: string }

// The wrapper builds one Headers object per call; read it through a
// helper so the optional chain stays total.
function getHeader(seen: Seen, name: string): string | null {
  const headers = seen.init?.headers as Headers | undefined
  return headers?.get(name) ?? null
}

function respond(status: number, body?: string, statusText = ''): Response {
  const headers = new Headers()
  if (body !== undefined) headers.set('content-type', 'application/json')
  // No body for an absent payload: a 204 response may not carry one.
  return new Response(body === undefined ? null : body, { status, statusText, headers })
}

function fetchOnce(status: number, body?: string, statusText = ''): Seen {
  // Mutate the returned object rather than rebinding `seen`: the mock
  // runs after fetchOnce has returned, so a rebinding would leave the
  // caller holding the empty object.
  const seen: Seen = { init: undefined, url: '' }
  vi.stubGlobal('fetch', vi.fn((url: string, init?: RequestInit) => {
    seen.url = url
    seen.init = init
    return Promise.resolve(respond(status, body, statusText))
  }))
  return seen
}

const USER: User = { id: 'u1', username: 'owner', email: 'owner@example.com', role: 'owner' }

afterEach(() => {
  store.clear()
  locations.length = 0
  vi.unstubAllGlobals()
})

describe('api client', () => {
  it('sends the bearer token and returns the parsed body', async () => {
    const seen = fetchOnce(200, JSON.stringify({ id: 'c1' }))
    saveSession('tok-1', USER)

    expect(await api<{ id: string }>('/coteries')).toEqual({ id: 'c1' })
    expect(seen.url).toBe('/api/v1/coteries')
    expect(getHeader(seen, 'Authorization')).toBe('Bearer tok-1')
  })

  it('omits the authorization header when signed out', async () => {
    const seen = fetchOnce(200, JSON.stringify({ items: [] }))

    await api('/coteries')
    expect(getHeader(seen, 'Authorization')).toBeNull()
  })

  it('sets the json content type for a request body', async () => {
    const seen = fetchOnce(201, JSON.stringify({ id: 'c1' }))

    await api('/coteries', { method: 'POST', body: JSON.stringify({ name: 'n' }) })
    expect(getHeader(seen, 'content-type')).toBe('application/json')
  })

  it('resolves undefined for a 204', async () => {
    fetchOnce(204)

    expect(await api('/notifications/1/read', { method: 'POST' })).toBeUndefined()
  })

  it('throws an ApiError carrying the envelope fields', async () => {
    fetchOnce(422, JSON.stringify({ error: { code: 'validation', message: 'bad request' } }))

    await expect(api('/coteries')).rejects.toBeInstanceOf(ApiError)
    await expect(api('/coteries')).rejects.toMatchObject({
      status: 422,
      code: 'validation',
      message: 'bad request',
    })
  })

  it('falls back to the status text when the error body is not json', async () => {
    fetchOnce(500, 'gateway exploded', 'Internal Server Error')

    await expect(api('/coteries')).rejects.toMatchObject({
      status: 500,
      code: 'unknown',
      message: 'Internal Server Error',
    })
  })

  it('ends a rejected session locally and redirects to login', async () => {
    const seen = fetchOnce(401, JSON.stringify({ error: { code: 'unauthorized', message: 'expired' } }))
    saveSession('dead', USER)

    await expect(api('/auth/me')).rejects.toMatchObject({ status: 401, code: 'unauthorized' })
    expect(seen.url).toBe('/api/v1/auth/me')
    expect(store.has('coterie.token')).toBe(false)
    expect(store.has('coterie.user')).toBe(false)
    expect(locations).toEqual(['/login'])
  })

  it('does not redirect for a 401 from an anonymous request', async () => {
    fetchOnce(401, JSON.stringify({ error: { code: 'unauthorized', message: 'bad credentials' } }))

    await expect(api('/auth/login', { method: 'POST' })).rejects.toMatchObject({ code: 'unauthorized' })
    expect(locations).toEqual([])
  })

  it('resolves when the revoke request fails, leaving the clear to the caller', async () => {
    fetchOnce(500, 'gateway exploded')
    saveSession('tok-1', USER)

    await expect(logout()).resolves.toBeUndefined()
    expect(store.has('coterie.token')).toBe(true)
  })
})
