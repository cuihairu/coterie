import { useState } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type {
  BillingPeriod,
  Contribution,
  Coterie,
  ListResponse,
  Member,
  Seat,
} from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/$coterieId')({
  component: CoterieDetailPage,
})

type Tab = 'members' | 'seats' | 'periods'

function CoterieDetailPage() {
  const { coterieId } = Route.useParams()
  const [tab, setTab] = useState<Tab>('members')

  const coterieQuery = useQuery({
    queryKey: ['coterie', coterieId],
    queryFn: () => api<Coterie>(`/coteries/${coterieId}`),
  })
  const membersQuery = useQuery({
    queryKey: ['members', coterieId],
    queryFn: () => api<ListResponse<Member>>(`/coteries/${coterieId}/members`),
  })

  const coterie = coterieQuery.data
  const subID = coterie?.subscription_id

  const seatsQuery = useQuery({
    queryKey: ['seats', subID],
    queryFn: () => api<ListResponse<Seat>>(`/subscriptions/${subID}/seats`),
    enabled: !!subID,
  })
  const periodsQuery = useQuery({
    queryKey: ['periods', subID],
    queryFn: () => api<ListResponse<BillingPeriod>>(`/subscriptions/${subID}/billing-periods`),
    enabled: !!subID,
  })

  const periods = periodsQuery.data?.items ?? []
  const contributionsQueries = useQueries({
    queries: periods.map((p) => ({
      queryKey: ['contributions', p.id],
      queryFn: () => api<ListResponse<Contribution>>(`/billing-periods/${p.id}/contributions`),
    })),
  })

  if (coterieQuery.isLoading) return <p className="text-slate-500">加载中…</p>
  if (coterieQuery.error)
    return <p className="text-red-600">加载失败：{coterieQuery.error.message}</p>
  if (!coterie) return null

  const tabs: { key: Tab; label: string }[] = [
    { key: 'members', label: '成员' },
    { key: 'seats', label: '席位' },
    { key: 'periods', label: '账期' },
  ]

  return (
    <div>
      <Link to="/coteries" className="text-sm text-slate-500 hover:text-slate-700">
        ← 返回圈列表
      </Link>
      <div className="mt-2 mb-6">
        <h1 className="text-xl font-semibold text-slate-900">{coterie.name}</h1>
        <p className="text-sm text-slate-500">
          状态 {coterie.status} · {coterie.listing === 'public' ? '公开目录' : '私有'} · 成员{' '}
          {coterie.member_count} · 席位 {coterie.seats_free}/{coterie.seats_total} 空闲
        </p>
      </div>

      <div className="mb-4 flex gap-1 border-b border-slate-200">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 text-sm ${
              tab === t.key
                ? 'border-b-2 border-emerald-700 font-medium text-emerald-800'
                : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'members' && <MembersTab query={membersQuery} />}
      {tab === 'seats' && <SeatsTab query={seatsQuery} />}
      {tab === 'periods' && <PeriodsTab periods={periods} queries={contributionsQueries} />}
    </div>
  )
}

function MembersTab({ query }: { query: UseQueryResult<ListResponse<Member>, Error> }) {
  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []
  if (items.length === 0) return <p className="text-slate-500">暂无成员。</p>
  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-slate-200 text-left text-slate-500">
          <th className="py-2 font-medium">用户</th>
          <th className="py-2 font-medium">角色</th>
          <th className="py-2 font-medium">加入时间</th>
        </tr>
      </thead>
      <tbody>
        {items.map((m) => (
          <tr key={m.id} className="border-b border-slate-100">
            <td className="py-2 font-mono text-xs text-slate-700">{m.user_id.slice(0, 8)}…</td>
            <td className="py-2">{m.role}</td>
            <td className="py-2 text-slate-500">{new Date(m.joined_at).toLocaleDateString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function SeatsTab({ query }: { query: UseQueryResult<ListResponse<Seat>, Error> }) {
  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []
  if (items.length === 0) return <p className="text-slate-500">暂无席位。</p>
  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-slate-200 text-left text-slate-500">
          <th className="py-2 font-medium">席位</th>
          <th className="py-2 font-medium">状态</th>
          <th className="py-2 font-medium">成员</th>
        </tr>
      </thead>
      <tbody>
        {items.map((s) => (
          <tr key={s.id} className="border-b border-slate-100">
            <td className="py-2">{s.label}</td>
            <td className="py-2">{s.status}</td>
            <td className="py-2 font-mono text-xs text-slate-700">
              {s.member_id ? `${s.member_id.slice(0, 8)}…` : '—'}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function PeriodsTab({
  periods,
  queries,
}: {
  periods: BillingPeriod[]
  queries: UseQueryResult<ListResponse<Contribution>, Error>[]
}) {
  if (periods.length === 0) return <p className="text-slate-500">暂无账期。</p>
  return (
    <div className="space-y-4">
      {periods.map((p, i) => {
        const rows = queries[i]?.data?.items ?? []
        return (
          <div key={p.id} className="rounded-xl border border-slate-200 bg-white p-4">
            <div className="mb-2 flex items-center justify-between">
              <h3 className="font-medium text-slate-900">
                {p.start_date} ~ {p.end_date}
              </h3>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                {p.status}
              </span>
            </div>
            {rows.length === 0 ? (
              <p className="text-sm text-slate-400">本期无分摊。</p>
            ) : (
              <table className="w-full text-sm">
                <tbody>
                  {rows.map((c) => (
                    <tr key={c.id} className="border-t border-slate-100">
                      <td className="py-1.5 font-mono text-xs text-slate-700">
                        {c.member_id.slice(0, 8)}…
                      </td>
                      <td className="py-1.5 text-right">
                        {c.amount} {c.currency}
                      </td>
                      <td className="py-1.5 text-right text-slate-500">{c.status}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )
      })}
    </div>
  )
}
