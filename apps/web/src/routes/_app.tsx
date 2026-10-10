import { useQuery } from '@tanstack/react-query'
import { Link, Outlet, createFileRoute, redirect, useRouter } from '@tanstack/react-router'
import { api, clearSession, getToken, getUser, logout } from '../lib/api'
import type { ListResponse, Notification } from '../lib/api'

export const Route = createFileRoute('/_app')({
  beforeLoad: async () => {
    // Entry verifies the stored token against the server so a revoked
    // or expired session bounces at the door, not after a failed fetch.
    if (!getToken()) throw redirect({ to: '/login' })
    try {
      await api('/auth/me')
    } catch {
      // A 401 already cleared and redirected in api(); anything else
      // (server down) still sends the user to a clean login page.
      clearSession()
      throw redirect({ to: '/login' })
    }
  },
  component: AppLayout,
})

function AppLayout() {
  const router = useRouter()
  const user = getUser()

  // Unread badge for the header bell; a quiet poll keeps it roughly
  // current without websockets. Keyed under the ['notifications']
  // prefix so mark-read on the inbox refreshes the badge too.
  const unreadQuery = useQuery({
    queryKey: ['notifications', 'unread'],
    queryFn: () => api<ListResponse<Notification>>('/notifications?unread=true'),
    refetchInterval: 30_000,
  })
  const unread = unreadQuery.data?.meta.total ?? 0

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <Link to="/coteries" className="text-lg font-semibold text-slate-900">
            Coterie
          </Link>
          <div className="flex items-center gap-4 text-sm">
            <Link to="/marketplace" className="text-slate-600 hover:text-slate-900">
              广场
            </Link>
            <Link to="/notifications" className="relative text-slate-600 hover:text-slate-900" title="通知">
              通知
              {unread > 0 && (
                <span className="absolute -right-3 -top-2 rounded-full bg-rose-600 px-1.5 text-xs font-medium text-white">
                  {unread > 99 ? '99+' : unread}
                </span>
              )}
            </Link>
            <span className="text-slate-500">{user?.email}</span>
            <button
              onClick={() => {
                // Server-side revoke first; the local clear is what
                // ends the client session either way.
                void logout()
                clearSession()
                router.navigate({ to: '/login' })
              }}
              className="text-slate-600 hover:text-slate-900"
            >
              退出
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
