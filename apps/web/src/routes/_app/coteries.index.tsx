import { useQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { Coterie, ListResponse } from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/')({
  component: CoteriesPage,
})

function CoteriesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['coteries'],
    queryFn: () => api<ListResponse<Coterie>>('/coteries'),
  })

  if (isLoading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">加载失败：{error.message}</p>

  const items = data?.items ?? []
  if (items.length === 0) {
    return (
      <div>
        <p className="text-slate-500">还没有加入任何共享圈。</p>
        <Link
          to="/coteries/new"
          className="mt-3 inline-block rounded-lg bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800"
        >
          创建共享圈
        </Link>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <Link
          to="/coteries/new"
          className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800"
        >
          创建共享圈
        </Link>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        {items.map((c) => (
          <Link
            key={c.id}
            to="/coteries/$coterieId"
            params={{ coterieId: c.id }}
            className="block rounded-xl border border-slate-200 bg-white p-5 shadow-sm hover:border-emerald-600"
          >
            <div className="mb-1 flex items-center justify-between">
              <h2 className="font-medium text-slate-900">{c.name}</h2>
              <span
                className={`rounded-full px-2 py-0.5 text-xs ${
                  c.status === 'open'
                    ? 'bg-emerald-50 text-emerald-700'
                    : 'bg-slate-100 text-slate-600'
                }`}
              >
                {c.status}
              </span>
            </div>
            <p className="text-sm text-slate-500">
              成员 {c.member_count} · 席位 {c.seats_free}/{c.seats_total} 空闲
              {c.full ? ' · 已满' : ''}
            </p>
            <p className="mt-1 text-xs text-slate-400">
              {c.listing === 'public' ? '公开目录' : '私有'} · 创建于{' '}
              {new Date(c.created_at).toLocaleDateString()}
            </p>
          </Link>
        ))}
      </div>
    </div>
  )
}
