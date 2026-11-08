import { useState } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { api, getUser } from '../../lib/api'
import type {
  BillingPeriod,
  Contribution,
  Coterie,
  JoinRequest,
  ListResponse,
  Member,
  Seat,
  Subscription,
  UsageRecord,
} from '../../lib/api'
import { LifecycleBar } from '../../components/coterie/LifecycleBar'
import { ReportBar } from '../../components/coterie/ReportBar'
import { RequestsTab } from '../../components/coterie/RequestsTab'
import { UsageTab } from '../../components/coterie/UsageTab'
import { SubscriptionTab } from '../../components/coterie/SubscriptionTab'
import { DisputesTab } from '../../components/coterie/DisputesTab'
import { AuditTab } from '../../components/coterie/AuditTab'
import { MembersTab } from '../../components/coterie/MembersTab'
import { SeatsTab } from '../../components/coterie/SeatsTab'
import { PeriodsTab } from '../../components/coterie/PeriodsTab'

export const Route = createFileRoute('/_app/coteries/$coterieId')({
  component: CoterieDetailPage,
})

type Tab = 'members' | 'seats' | 'periods' | 'usage' | 'settings' | 'disputes' | 'audit' | 'requests'

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
  const subscriptionQuery = useQuery({
    queryKey: ['subscription', subID],
    queryFn: () => api<Subscription>(`/subscriptions/${subID}`),
    enabled: !!subID,
  })
  const usageQuery = useQuery({
    queryKey: ['usage', subID],
    queryFn: () => api<ListResponse<UsageRecord>>(`/subscriptions/${subID}/usage-records`),
    enabled: !!subID,
  })
  const requestsQuery = useQuery({
    queryKey: ['join-requests', coterieId],
    queryFn: () => api<ListResponse<JoinRequest>>(`/coteries/${coterieId}/join-requests`),
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

  const isOwner = (membersQuery.data?.items ?? []).some(
    (m) => m.user_id === getUser()?.id && m.role === 'owner',
  )

  const tabs: { key: Tab; label: string }[] = [
    { key: 'members', label: '成员' },
    { key: 'seats', label: '席位' },
    { key: 'periods', label: '账期' },
    { key: 'usage', label: '用量' },
    ...(isOwner ? [{ key: 'settings' as Tab, label: '订阅' }] : []),
    ...(isOwner ? [{ key: 'disputes' as Tab, label: '争议' }] : []),
    ...(isOwner ? [{ key: 'audit' as Tab, label: '审计' }] : []),
    ...(isOwner ? [{ key: 'requests' as Tab, label: '申请' }] : []),
  ]

  return (
    <div>
      <Link to="/coteries" className="text-sm text-slate-500 hover:text-slate-700">
        ← 返回圈列表
      </Link>
      <div className="mt-2 mb-4">
        <h1 className="text-xl font-semibold text-slate-900">{coterie.name}</h1>
        <p className="text-sm text-slate-500">
          状态 {coterie.status} · {coterie.listing === 'public' ? '公开目录' : '私有'} · 成员{' '}
          {coterie.member_count} · 席位 {coterie.seats_free}/{coterie.seats_total} 空闲
        </p>
      </div>

      {isOwner && <LifecycleBar coterie={coterie} />}
      {!isOwner && coterie.listing === 'public' && <ReportBar coterieId={coterieId} />}

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

      {tab === 'members' && (
        <MembersTab coterieId={coterieId} query={membersQuery} isOwner={isOwner} />
      )}
      {tab === 'seats' && (
        <SeatsTab subID={subID} query={seatsQuery} membersQuery={membersQuery} isOwner={isOwner} />
      )}
      {tab === 'periods' && (
        <PeriodsTab
          subID={subID}
          periods={periods}
          queries={contributionsQueries}
          membersQuery={membersQuery}
          isOwner={isOwner}
        />
      )}
      {tab === 'usage' && (
        <UsageTab
          subID={subID}
          subscriptionQuery={subscriptionQuery}
          usageQuery={usageQuery}
          membersQuery={membersQuery}
          isOwner={isOwner}
        />
      )}
      {tab === 'settings' && isOwner && (
        <SubscriptionTab subID={subID} subscriptionQuery={subscriptionQuery} />
      )}
      {tab === 'disputes' && isOwner && <DisputesTab subID={subID} membersQuery={membersQuery} />}
      {tab === 'audit' && isOwner && <AuditTab coterieId={coterieId} />}
      {tab === 'requests' && isOwner && <RequestsTab coterieId={coterieId} query={requestsQuery} />}
    </div>
  )
}
