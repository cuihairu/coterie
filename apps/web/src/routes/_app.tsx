import { Link, Outlet, createFileRoute, redirect, useRouter } from '@tanstack/react-router'
import { clearSession, getUser } from '../lib/api'

export const Route = createFileRoute('/_app')({
  beforeLoad: () => {
    if (!getUser()) throw redirect({ to: '/login' })
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
