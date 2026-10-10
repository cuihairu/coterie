import { Link, Outlet, createFileRoute, redirect, useRouter } from '@tanstack/react-router'
import { api, clearSession, getToken, getUser, logout } from '../lib/api'

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

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
          <Link to="/coteries" className="text-lg font-semibold text-slate-900">
            Coterie
          </Link>
          <div className="flex items-center gap-4 text-sm">
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
