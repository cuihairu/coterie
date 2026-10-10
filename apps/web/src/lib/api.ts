// API client: token storage, the error envelope, and the response
// shapes the read-only views render. Field names mirror the Go
// models (internal/*/model.go).

const TOKEN_KEY = 'coterie.token'
const USER_KEY = 'coterie.user'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getUser(): User | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as User
  } catch {
    return null
  }
}

export function saveSession(token: string, user: User): void {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

// expireLocalSession ends a dead session locally and returns to the
// login page. Only reached for requests that carried a token, so
// login/register failures never loop through here.
function expireLocalSession(): void {
  clearSession()
  window.location.assign('/login')
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`/api/v1${path}`, { ...init, headers })
  if (!res.ok) {
    // A rejected session is dead: expired or revoked tokens log the
    // user out locally instead of stranding the page with failures.
    if (res.status === 401 && token) {
      expireLocalSession()
    }
    let code = 'unknown'
    let message = res.statusText
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
    } catch {
      // Non-JSON error body; keep the status text.
    }
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// logout revokes the session server-side. Failures are tolerated: a
// network error or an already-revoked token still logs out locally.
export async function logout(): Promise<void> {
  try {
    await api<void>('/auth/logout', { method: 'POST' })
  } catch {
    // Local clear below is what actually ends the client session.
  }
}

export interface User {
  id: string
  username: string
  email: string
  role: string
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Coterie {
  id: string
  subscription_id: string
  name: string
  status: string
  listing: string
  payment_gate: boolean
  member_count: number
  seats_total: number
  seats_free: number
  full: boolean
  created_at: string
}

export interface Member {
  id: string
  coterie_id: string
  user_id: string
  role: string
  status: string
  joined_at: string
}

export interface Seat {
  id: string
  subscription_id: string
  member_id?: string
  label: string
  status: string
  metadata: Record<string, unknown>
}

export interface BillingPeriod {
  id: string
  subscription_id: string
  start_date: string
  end_date: string
  status: string
}

export interface Contribution {
  id: string
  billing_period_id: string
  member_id: string
  amount: string
  currency: string
  status: string
  paid_at?: string
}

export interface Provider {
  id: string
  slug: string
  name: string
  category: string
}

export interface Product {
  id: string
  provider_id: string
  name: string
  tier?: string
}

export interface Invitation {
  id: string
  coterie_id: string
  role: string
  status: string
  token?: string
  expires_at?: string
}

export interface Subscription {
  id: string
  product_id: string
  owner_user_id: string
  billing_cycle: string
  price: string
  currency: string
  start_date: string
  max_seats: number
  sharing_policy?: SharingPolicy | null
}

// SharingPolicy is the subscription's JSONB policy bag; usage_limit is
// the D21 per-member per-period cap enforced on the ledger write.
export interface SharingPolicy {
  usage_limit?: UsageLimit
  [key: string]: unknown
}

export interface UsageLimit {
  unit: string
  per_period: string
}

export interface UsageRecord {
  id: string
  subscription_id: string
  member_id: string
  seat_id?: string
  amount: string
  unit: string
  metadata: Record<string, unknown>
  recorded_at: string
  created_at: string
}

// Dispute is one member's challenge against a contribution (D14);
// the owner decides resolved or rejected, one-way.
export interface Dispute {
  id: string
  contribution_id: string
  subscription_id: string
  raised_by: string
  reason: string
  evidence?: string
  status: string
  resolution_note?: string
  decided_by?: string
  created_at: string
  decided_at?: string
}

export interface Notification {
  id: string
  type: string
  title: string
  body: string
  read_at?: string
  created_at: string
}

export interface JoinRequest {
  id: string
  coterie_id: string
  user_id: string
  username: string
  message: string
  status: string
  created_at: string
}

export interface DirectoryEntry {
  coterie_id: string
  name: string
  status: string
  product_name: string
  provider_name: string
  price: string
  currency: string
  member_count: number
  seats_total: number
  seats_free: number
  full: boolean
  share_estimate: string
  owner?: {
    user_id: string
    username: string
    contributions: { paid: number; pending: number; waived: number; cancelled: number }
    payment_ratio?: string
  }
}

export interface ListResponse<T> {
  items: T[]
  meta: { total: number; limit: number; offset: number }
}
