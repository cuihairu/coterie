import { useState } from 'react'
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { api, getUser } from '../../lib/api'
import type {
  BillingPeriod,
  Contribution,
  Coterie,
  Invitation,
  JoinRequest,
  ListResponse,
  Member,
  Seat,
} from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/$coterieId')({
  component: CoterieDetailPage,
})

type Tab = 'members' | 'seats' | 'periods' | 'requests'

// Mirrors the server lifecycle (internal/coterie/service.go transitions).
const NEXT_STATUS: Record<string, string[]> = {
  draft: ['open', 'closed'],
  open: ['active', 'closed'],
  active: ['paused', 'closed'],
  paused: ['active', 'closed'],
  closed: [],
}

const STATUS_LABEL: Record<string, string> = {
  open: '发布',
  active: '激活',
  paused: '暂停',
  closed: '关闭',
}

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
        <PeriodsTab subID={subID} periods={periods} queries={contributionsQueries} isOwner={isOwner} />
      )}
      {tab === 'requests' && isOwner && <RequestsTab coterieId={coterieId} query={requestsQuery} />}
    </div>
  )
}

function LifecycleBar({ coterie }: { coterie: Coterie }) {
  const qc = useQueryClient()
  const [error, setError] = useState('')

  const transition = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      api<Coterie>(`/coteries/${coterie.id}`, { method: 'PATCH', body: JSON.stringify(body) }),
    onSuccess: () => {
      setError('')
      void qc.invalidateQueries({ queryKey: ['coterie', coterie.id] })
    },
    onError: (err) => setError(err.message),
  })

  const nexts = NEXT_STATUS[coterie.status] ?? []

  return (
    <div className="mb-4 flex items-center gap-2">
      {nexts.map((s) => (
        <button
          key={s}
          onClick={() => transition.mutate({ status: s })}
          disabled={transition.isPending}
          className={`rounded-lg px-3 py-1.5 text-sm font-medium ${
            s === 'closed'
              ? 'bg-slate-200 text-slate-700 hover:bg-slate-300'
              : 'bg-emerald-700 text-white hover:bg-emerald-800'
          } disabled:opacity-50`}
        >
          {STATUS_LABEL[s] ?? s}
        </button>
      ))}
      <button
        onClick={() => transition.mutate({ payment_gate: !coterie.payment_gate })}
        disabled={transition.isPending}
        className="rounded-lg bg-slate-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
        title="开启后同意加入需先收准入费（D25）"
      >
        {coterie.payment_gate ? '关闭支付闸门' : '开启支付闸门'}
      </button>
      {error && <span className="text-sm text-red-600">{error}</span>}
    </div>
  )
}

// RequestsTab is the owner's inbox for join requests (FR-16): pending
// requests are accepted or declined; with the payment gate on (D25),
// accepting holds the request at awaiting_payment and the owner
// records the admission charge to admit.
function RequestsTab({
  coterieId,
  query,
}: {
  coterieId: string
  query: UseQueryResult<ListResponse<JoinRequest>, Error>
}) {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [paying, setPaying] = useState<string | null>(null)
  const [payRef, setPayRef] = useState('')

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['join-requests', coterieId] })
    void qc.invalidateQueries({ queryKey: ['members', coterieId] })
    void qc.invalidateQueries({ queryKey: ['coterie', coterieId] })
  }

  const decide = useMutation({
    mutationFn: (body: { id: string; action: 'accept' | 'decline' }) =>
      body.action === 'accept'
        ? api<JoinRequest>(`/join-requests/${body.id}/accept`, { method: 'POST' })
        : api<JoinRequest>(`/join-requests/${body.id}/decline`, { method: 'POST' }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const payAdmission = useMutation({
    mutationFn: (requestID: string) =>
      api<JoinRequest>(`/join-requests/${requestID}/payments`, {
        method: 'POST',
        body: JSON.stringify({ method: 'manual', external_ref: payRef }),
      }),
    onSuccess: () => {
      setError('')
      setPaying(null)
      setPayRef('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []

  if (items.length === 0) return <p className="text-slate-500">暂无加入申请。</p>

  return (
    <div className="space-y-2">
      {error && <p className="text-sm text-red-600">{error}</p>}
      {items.map((r) => (
        <div key={r.id} className="rounded-xl border border-slate-200 bg-white p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-slate-900">
                {r.username}
                <span className="ml-2 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                  {r.status}
                </span>
              </p>
              {r.message && <p className="mt-1 text-sm text-slate-500">{r.message}</p>}
              <p className="mt-1 text-xs text-slate-400">
                {new Date(r.created_at).toLocaleString()}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              {r.status === 'pending' && (
                <>
                  <button
                    onClick={() => decide.mutate({ id: r.id, action: 'accept' })}
                    disabled={decide.isPending}
                    className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
                  >
                    同意
                  </button>
                  <button
                    onClick={() => decide.mutate({ id: r.id, action: 'decline' })}
                    disabled={decide.isPending}
                    className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50"
                  >
                    拒绝
                  </button>
                </>
              )}
              {r.status === 'awaiting_payment' &&
                (paying === r.id ? (
                  <span className="inline-flex items-center gap-1">
                    <input
                      className="w-32 rounded border border-slate-300 px-1 py-0.5 text-xs"
                      placeholder="备注/流水号"
                      value={payRef}
                      onChange={(e) => setPayRef(e.target.value)}
                    />
                    <button
                      onClick={() => payAdmission.mutate(r.id)}
                      disabled={payAdmission.isPending}
                      className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                    >
                      确认收费
                    </button>
                    <button
                      onClick={() => {
                        setPaying(null)
                        setPayRef('')
                      }}
                      className="text-xs text-slate-500 hover:underline"
                    >
                      取消
                    </button>
                  </span>
                ) : (
                  <button
                    onClick={() => {
                      setPaying(r.id)
                      setPayRef('')
                    }}
                    className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800"
                  >
                    发起收费
                  </button>
                ))}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

function MembersTab({
  coterieId,
  query,
  isOwner,
}: {
  coterieId: string
  query: UseQueryResult<ListResponse<Member>, Error>
  isOwner: boolean
}) {
  const qc = useQueryClient()
  const [inviteToken, setInviteToken] = useState('')
  const [error, setError] = useState('')

  const invite = useMutation({
    mutationFn: () =>
      api<Invitation>(`/coteries/${coterieId}/invitations`, {
        method: 'POST',
        body: JSON.stringify({ role: 'member' }),
      }),
    onSuccess: (inv) => {
      setError('')
      setInviteToken(inv.token ?? '')
      void qc.invalidateQueries({ queryKey: ['members', coterieId] })
    },
    onError: (err) => setError(err.message),
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []

  return (
    <div>
      {isOwner && (
        <div className="mb-4">
          <button
            onClick={() => invite.mutate()}
            disabled={invite.isPending}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {invite.isPending ? '生成中…' : '邀请成员'}
          </button>
          {error && <span className="ml-3 text-sm text-red-600">{error}</span>}
          {inviteToken && (
            <div className="mt-2 flex max-w-xl items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-2">
              <code className="flex-1 truncate font-mono text-xs text-emerald-900">
                {inviteToken}
              </code>
              <button
                onClick={() => void navigator.clipboard.writeText(inviteToken)}
                className="rounded bg-white px-2 py-1 text-xs text-emerald-800 hover:bg-emerald-100"
              >
                复制
              </button>
            </div>
          )}
          <p className="mt-1 text-xs text-slate-400">邀请令牌只显示一次，请立即复制发给对方。</p>
        </div>
      )}
      {items.length === 0 ? (
        <p className="text-slate-500">暂无成员。</p>
      ) : (
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
      )}
    </div>
  )
}

function SeatsTab({
  subID,
  query,
  membersQuery,
  isOwner,
}: {
  subID?: string
  query: UseQueryResult<ListResponse<Seat>, Error>
  membersQuery: UseQueryResult<ListResponse<Member>, Error>
  isOwner: boolean
}) {
  const qc = useQueryClient()
  const [assigning, setAssigning] = useState<string | null>(null)
  const [memberID, setMemberID] = useState('')
  const [error, setError] = useState('')

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['seats', subID] })
    void qc.invalidateQueries({ queryKey: ['coterie'] })
  }

  const provision = useMutation({
    mutationFn: () =>
      api<Seat[]>(`/subscriptions/${subID}/seats`, {
        method: 'POST',
        body: JSON.stringify({ count: 1 }),
      }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const assign = useMutation({
    mutationFn: (seatID: string) =>
      api<Seat>(`/seats/${seatID}/assign`, {
        method: 'POST',
        body: JSON.stringify({ member_id: memberID }),
      }),
    onSuccess: () => {
      setError('')
      setAssigning(null)
      setMemberID('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  if (query.isLoading || membersQuery.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []
  const members = membersQuery.data?.items ?? []

  return (
    <div>
      {isOwner && (
        <div className="mb-4 flex items-center gap-2">
          <button
            onClick={() => provision.mutate()}
            disabled={provision.isPending || !subID}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {provision.isPending ? '增补中…' : '增补席位'}
          </button>
          {error && <span className="text-sm text-red-600">{error}</span>}
        </div>
      )}
      {items.length === 0 ? (
        <p className="text-slate-500">暂无席位。</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-slate-500">
              <th className="py-2 font-medium">席位</th>
              <th className="py-2 font-medium">状态</th>
              <th className="py-2 font-medium">成员</th>
              {isOwner && <th className="py-2 font-medium" />}
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
                {isOwner && (
                  <td className="py-2 text-right">
                    {s.status === 'free' && assigning !== s.id && (
                      <button
                        onClick={() => {
                          setAssigning(s.id)
                          setMemberID('')
                        }}
                        className="text-xs text-emerald-700 hover:underline"
                      >
                        分配
                      </button>
                    )}
                    {assigning === s.id && (
                      <span className="inline-flex items-center gap-1">
                        <select
                          className="rounded border border-slate-300 px-1 py-0.5 text-xs"
                          value={memberID}
                          onChange={(e) => setMemberID(e.target.value)}
                        >
                          <option value="">选择成员…</option>
                          {members.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.user_id.slice(0, 8)}…（{m.role}）
                            </option>
                          ))}
                        </select>
                        <button
                          onClick={() => assign.mutate(s.id)}
                          disabled={!memberID || assign.isPending}
                          className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                        >
                          确认
                        </button>
                        <button
                          onClick={() => setAssigning(null)}
                          className="text-xs text-slate-500 hover:underline"
                        >
                          取消
                        </button>
                      </span>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function PeriodsTab({
  subID,
  periods,
  queries,
  isOwner,
}: {
  subID?: string
  periods: BillingPeriod[]
  queries: UseQueryResult<ListResponse<Contribution>, Error>[]
  isOwner: boolean
}) {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [newStart, setNewStart] = useState(() => new Date().toISOString().slice(0, 10))
  const [newEnd, setNewEnd] = useState(() => {
    const d = new Date()
    d.setMonth(d.getMonth() + 1)
    return d.toISOString().slice(0, 10)
  })
  const [paying, setPaying] = useState<string | null>(null)
  const [payRef, setPayRef] = useState('')

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['periods', subID] })
    void qc.invalidateQueries({ queryKey: ['contributions'] })
  }

  const createPeriod = useMutation({
    mutationFn: () =>
      api<BillingPeriod>(`/subscriptions/${subID}/billing-periods`, {
        method: 'POST',
        body: JSON.stringify({ start_date: newStart, end_date: newEnd }),
      }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const generate = useMutation({
    mutationFn: (periodID: string) =>
      api<ListResponse<Contribution>>(`/billing-periods/${periodID}/contributions/generate`, {
        method: 'POST',
        body: JSON.stringify({ mode: 'equal' }),
      }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const closePeriod = useMutation({
    mutationFn: (periodID: string) =>
      api<BillingPeriod>(`/billing-periods/${periodID}/close`, { method: 'POST' }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const recordPayment = useMutation({
    mutationFn: (contributionID: string) =>
      api<unknown>(`/contributions/${contributionID}/payments`, {
        method: 'POST',
        body: JSON.stringify({ method: 'manual', external_ref: payRef }),
      }),
    onSuccess: () => {
      setError('')
      setPaying(null)
      setPayRef('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  return (
    <div>
      {isOwner && (
        <div className="mb-4 flex flex-wrap items-center gap-2 rounded-xl border border-slate-200 bg-white p-3">
          <input
            type="date"
            value={newStart}
            onChange={(e) => setNewStart(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-xs"
          />
          <span className="text-xs text-slate-400">至</span>
          <input
            type="date"
            value={newEnd}
            onChange={(e) => setNewEnd(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-xs"
          />
          <button
            onClick={() => createPeriod.mutate()}
            disabled={createPeriod.isPending || !subID}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {createPeriod.isPending ? '创建中…' : '新建账期'}
          </button>
          {error && <span className="text-sm text-red-600">{error}</span>}
        </div>
      )}
      {periods.length === 0 ? (
        <p className="text-slate-500">暂无账期。</p>
      ) : (
        <div className="space-y-4">
          {periods.map((p, i) => {
            const rows = queries[i]?.data?.items ?? []
            return (
              <div key={p.id} className="rounded-xl border border-slate-200 bg-white p-4">
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="font-medium text-slate-900">
                    {p.start_date} ~ {p.end_date}
                  </h3>
                  <div className="flex items-center gap-2">
                    <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                      {p.status}
                    </span>
                    {isOwner && p.status === 'open' && (
                      <>
                        <button
                          onClick={() => generate.mutate(p.id)}
                          disabled={generate.isPending}
                          className="rounded px-2 py-0.5 text-xs text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
                        >
                          生成分摊
                        </button>
                        <button
                          onClick={() => closePeriod.mutate(p.id)}
                          disabled={closePeriod.isPending}
                          className="rounded px-2 py-0.5 text-xs text-slate-600 hover:bg-slate-100 disabled:opacity-50"
                        >
                          关闭账期
                        </button>
                      </>
                    )}
                  </div>
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
                          {isOwner && c.status === 'pending' && (
                            <td className="py-1.5 text-right">
                              {paying === c.id ? (
                                <span className="inline-flex items-center gap-1">
                                  <input
                                    className="w-32 rounded border border-slate-300 px-1 py-0.5 text-xs"
                                    placeholder="备注/流水号"
                                    value={payRef}
                                    onChange={(e) => setPayRef(e.target.value)}
                                  />
                                  <button
                                    onClick={() => recordPayment.mutate(c.id)}
                                    disabled={recordPayment.isPending}
                                    className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                                  >
                                    确认
                                  </button>
                                  <button
                                    onClick={() => {
                                      setPaying(null)
                                      setPayRef('')
                                    }}
                                    className="text-xs text-slate-500 hover:underline"
                                  >
                                    取消
                                  </button>
                                </span>
                              ) : (
                                <button
                                  onClick={() => {
                                    setPaying(c.id)
                                    setPayRef('')
                                  }}
                                  className="text-xs text-emerald-700 hover:underline"
                                >
                                  登记收款
                                </button>
                              )}
                            </td>
                          )}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
