import { Fragment, useState } from 'react'
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { api, getUser } from '../../lib/api'
import type {
  AuditLog,
  BillingPeriod,
  BlockEntry,
  Contribution,
  Coterie,
  Dispute,
  Invitation,
  JoinRequest,
  ListResponse,
  Member,
  PaymentRecord,
  Report,
  Seat,
  SharingPolicy,
  Subscription,
  UsageLimit,
  UsageRecord,
} from '../../lib/api'
import { AUDIT_ACTIONS, AUDIT_LABEL } from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/$coterieId')({
  component: CoterieDetailPage,
})

type Tab = 'members' | 'seats' | 'periods' | 'usage' | 'settings' | 'disputes' | 'audit' | 'requests'

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

// ReportBar is the non-owner flag surface (FR-15): anyone logged in
// can flag a publicly listed circle; one open report per user.
function ReportBar({ coterieId }: { coterieId: string }) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')

  const report = useMutation({
    mutationFn: () =>
      api<Report>(`/coteries/${coterieId}/report`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    onSuccess: () => {
      setDone(true)
      setOpen(false)
    },
    onError: (err) => setError(err.message),
  })

  if (done) {
    return <p className="mb-4 rounded-xl border border-slate-200 bg-white p-3 text-xs text-slate-500">已提交举报，平台管理员会处置。</p>
  }
  return (
    <div className="mb-4 rounded-xl border border-slate-200 bg-white p-3">
      {open ? (
        <span className="inline-flex items-center gap-1">
          <input
            className="w-64 rounded border border-slate-300 px-2 py-1 text-xs"
            placeholder="举报原因（必填）"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <button
            onClick={() => report.mutate()}
            disabled={report.isPending || !reason.trim()}
            className="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-rose-700 disabled:opacity-50"
          >
            {report.isPending ? '提交中…' : '提交举报'}
          </button>
          <button
            onClick={() => setOpen(false)}
            className="text-xs text-slate-500 hover:underline"
          >
            取消
          </button>
          {error && <span className="text-xs text-red-600">{error}</span>}
        </span>
      ) : (
        <button
          onClick={() => setOpen(true)}
          className="text-xs text-slate-500 hover:underline"
        >
          举报此圈
        </button>
      )}
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
      <button
        onClick={() =>
          transition.mutate({ listing: coterie.listing === 'public' ? 'private' : 'public' })
        }
        disabled={transition.isPending}
        className="rounded-lg bg-slate-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
        title="公开后出现在市场目录，任何人可申请加入（D10）"
      >
        {coterie.listing === 'public' ? '下架（转私有）' : '上架公开目录'}
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
  const [blocking, setBlocking] = useState<string | null>(null)

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

  // D26 blocklist: bars a user from the admission paths. It never
  // removes membership, so it is offered on non-accepted rows only.
  const blocksQuery = useQuery({
    queryKey: ['blocks', coterieId],
    queryFn: () => api<ListResponse<BlockEntry>>(`/coteries/${coterieId}/blocks`),
  })
  const blockedUsers = new Set((blocksQuery.data?.items ?? []).map((b) => b.user_id))

  const block = useMutation({
    mutationFn: (userID: string) =>
      api<unknown>(`/coteries/${coterieId}/blocks/${userID}`, { method: 'PUT' }),
    onSuccess: () => {
      setError('')
      setBlocking(null)
      void qc.invalidateQueries({ queryKey: ['blocks', coterieId] })
    },
    onError: (err) => setError(err.message),
  })

  const unblock = useMutation({
    mutationFn: (userID: string) =>
      api<void>(`/coteries/${coterieId}/blocks/${userID}`, { method: 'DELETE' }),
    onSuccess: () => {
      setError('')
      void qc.invalidateQueries({ queryKey: ['blocks', coterieId] })
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
  const blocks = blocksQuery.data?.items ?? []

  return (
    <div className="space-y-2">
      {error && <p className="text-sm text-red-600">{error}</p>}
      {items.length === 0 && <p className="text-slate-500">暂无加入申请。</p>}
      {items.map((r) => (
        <div key={r.id} className="rounded-xl border border-slate-200 bg-white p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-slate-900">
                {r.username}
                <span className="ml-2 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                  {r.status}
                </span>
                {blockedUsers.has(r.user_id) && (
                  <span className="ml-1 rounded-full bg-rose-50 px-2 py-0.5 text-xs text-rose-600">
                    已封禁
                  </span>
                )}
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
              {r.status !== 'accepted' &&
                (blocking === r.user_id ? (
                  <>
                    <button
                      onClick={() => block.mutate(r.user_id)}
                      disabled={block.isPending}
                      className="text-xs text-rose-600 hover:underline disabled:opacity-50"
                    >
                      确认拉黑
                    </button>
                    <button
                      onClick={() => setBlocking(null)}
                      className="text-xs text-slate-500 hover:underline"
                    >
                      取消
                    </button>
                  </>
                ) : (
                  !blockedUsers.has(r.user_id) && (
                    <button
                      onClick={() => setBlocking(r.user_id)}
                      className="text-xs text-rose-600 hover:underline"
                    >
                      拉黑
                    </button>
                  )
                ))}
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

      <h3 className="pt-2 text-sm font-medium text-slate-900">封禁名单</h3>
      {blocks.length === 0 ? (
        <p className="text-sm text-slate-400">暂无封禁用户。</p>
      ) : (
        blocks.map((b) => (
          <div
            key={b.user_id}
            className="flex items-center justify-between rounded-xl border border-slate-200 bg-white p-3"
          >
            <p className="text-sm text-slate-700">
              {b.username}
              <span className="ml-2 text-xs text-slate-400">{b.email}</span>
            </p>
            <button
              onClick={() => unblock.mutate(b.user_id)}
              disabled={unblock.isPending}
              className="text-xs text-slate-600 hover:underline disabled:opacity-50"
            >
              解封
            </button>
          </div>
        ))
      )}
    </div>
  )
}

// UsageTab shows the metered consumption ledger (D9) for the coterie's
// subscription — records are append-only, negative amounts are
// corrections. The owner records usage and manages the D21 per-member
// per-period cap stored in the subscription's sharing policy.
function UsageTab({
  subID,
  subscriptionQuery,
  usageQuery,
  membersQuery,
  isOwner,
}: {
  subID?: string
  subscriptionQuery: UseQueryResult<Subscription, Error>
  usageQuery: UseQueryResult<ListResponse<UsageRecord>, Error>
  membersQuery: UseQueryResult<ListResponse<Member>, Error>
  isOwner: boolean
}) {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [memberID, setMemberID] = useState('')
  const [unit, setUnit] = useState('')
  const [amount, setAmount] = useState('')
  const [editingLimit, setEditingLimit] = useState(false)
  const [limitUnit, setLimitUnit] = useState('')
  const [limitPer, setLimitPer] = useState('')

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['usage', subID] })
  }

  const record = useMutation({
    mutationFn: () =>
      api<UsageRecord>(`/subscriptions/${subID}/usage-records`, {
        method: 'POST',
        body: JSON.stringify({ member_id: memberID, unit, amount }),
      }),
    onSuccess: () => {
      setError('')
      setAmount('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const saveLimit = useMutation({
    mutationFn: (limit: UsageLimit | null) => {
      const sub = subscriptionQuery.data
      const policy: SharingPolicy = { ...(sub?.sharing_policy ?? {}) }
      if (limit) {
        policy.usage_limit = limit
      } else {
        delete policy.usage_limit
      }
      return api<Subscription>(`/subscriptions/${subID}`, {
        method: 'PATCH',
        body: JSON.stringify({ sharing_policy: policy }),
      })
    },
    onSuccess: () => {
      setError('')
      setEditingLimit(false)
      void qc.invalidateQueries({ queryKey: ['subscription', subID] })
    },
    onError: (err) => setError(err.message),
  })

  if (subscriptionQuery.isLoading) return <p className="text-slate-500">加载中…</p>
  if (subscriptionQuery.error)
    return <p className="text-red-600">加载失败：{subscriptionQuery.error.message}</p>

  const limit = subscriptionQuery.data?.sharing_policy?.usage_limit
  const members = membersQuery.data?.items ?? []
  // member_id on a record is the member row id; map it to the user id
  // the member table displays.
  const nameOf = new Map(members.map((m) => [m.id, m.user_id.slice(0, 8) + '…']))
  const records = usageQuery.data?.items ?? []

  return (
    <div>
      <div className="mb-4 rounded-lg border border-slate-200 bg-white p-4">
        <div className="flex items-center justify-between">
          <p className="text-sm text-slate-700">
            {limit ? (
              <>
                每成员每期用量上限 <span className="font-medium">{limit.per_period}</span>{' '}
                {limit.unit}
                <span className="ml-1 text-xs text-slate-400">（超出记录返回 409）</span>
              </>
            ) : (
              <span className="text-slate-500">未设用量上限。</span>
            )}
          </p>
          {isOwner && !editingLimit && (
            <button
              onClick={() => {
                setLimitUnit(limit?.unit ?? '')
                setLimitPer(limit?.per_period ?? '')
                setEditingLimit(true)
              }}
              className="text-sm text-emerald-700 hover:underline"
            >
              {limit ? '修改' : '设置上限'}
            </button>
          )}
        </div>
        {isOwner && editingLimit && (
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <input
              className="w-28 rounded border border-slate-300 px-2 py-1 text-sm"
              placeholder="单位，如 credits"
              value={limitUnit}
              onChange={(e) => setLimitUnit(e.target.value)}
            />
            <input
              className="w-32 rounded border border-slate-300 px-2 py-1 text-sm"
              placeholder="每期上限"
              value={limitPer}
              onChange={(e) => setLimitPer(e.target.value)}
            />
            <button
              onClick={() => {
                if (!limitUnit.trim() || !limitPer.trim()) {
                  setError('单位与上限均必填')
                  return
                }
                saveLimit.mutate({ unit: limitUnit.trim(), per_period: limitPer.trim() })
              }}
              disabled={saveLimit.isPending}
              className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
            >
              保存
            </button>
            <button
              onClick={() => saveLimit.mutate(null)}
              disabled={saveLimit.isPending}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50"
            >
              清除上限
            </button>
            <button
              onClick={() => setEditingLimit(false)}
              className="text-sm text-slate-500 hover:underline"
            >
              取消
            </button>
          </div>
        )}
      </div>

      {isOwner && (
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <select
            className="rounded border border-slate-300 px-2 py-1.5 text-sm"
            value={memberID}
            onChange={(e) => setMemberID(e.target.value)}
          >
            <option value="">选择成员</option>
            {members.map((m) => (
              <option key={m.id} value={m.id}>
                {m.user_id.slice(0, 8)}…（{m.role}）
              </option>
            ))}
          </select>
          <input
            className="w-32 rounded border border-slate-300 px-2 py-1.5 text-sm"
            placeholder="单位，如 GB"
            value={unit}
            onChange={(e) => setUnit(e.target.value)}
          />
          <input
            className="w-32 rounded border border-slate-300 px-2 py-1.5 text-sm"
            placeholder="用量，负数为修正"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
          <button
            onClick={() => record.mutate()}
            disabled={record.isPending || !memberID || !unit.trim() || !amount.trim()}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {record.isPending ? '记账中…' : '记账'}
          </button>
        </div>
      )}
      {error && <p className="mb-4 text-sm text-red-600">{error}</p>}

      {usageQuery.isLoading ? (
        <p className="text-slate-500">加载中…</p>
      ) : records.length === 0 ? (
        <p className="text-slate-500">暂无用量记录。</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-slate-500">
              <th className="py-2 font-medium">成员</th>
              <th className="py-2 font-medium">用量</th>
              <th className="py-2 font-medium">单位</th>
              <th className="py-2 font-medium">记账时间</th>
            </tr>
          </thead>
          <tbody>
            {records.map((r) => (
              <tr key={r.id} className="border-b border-slate-100">
                <td className="py-2 font-mono text-xs text-slate-700">
                  {nameOf.get(r.member_id) ?? r.member_id.slice(0, 8) + '…'}
                </td>
                <td className={`py-2 font-medium ${r.amount.startsWith('-') ? 'text-red-600' : ''}`}>
                  {r.amount}
                </td>
                <td className="py-2 text-slate-600">{r.unit}</td>
                <td className="py-2 text-slate-500">{new Date(r.recorded_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// SubscriptionTab is the owner's subscription settings (PATCH
// /subscriptions/{id}): price, billing cycle with the D20 custom-days
// pairing, renewal date, and the auto-billing toggle. Local edits win
// over the fetched subscription until saved; every save lands in the
// audit ledger as subscription_updated.
function SubscriptionTab({
  subID,
  subscriptionQuery,
}: {
  subID?: string
  subscriptionQuery: UseQueryResult<Subscription, Error>
}) {
  const qc = useQueryClient()
  const sub = subscriptionQuery.data
  const [error, setError] = useState('')
  const [price, setPrice] = useState<string | null>(null)
  const [cycle, setCycle] = useState<string | null>(null)
  const [cycleDays, setCycleDays] = useState('')
  const [renewal, setRenewal] = useState<string | null>(null)
  const [autoBilling, setAutoBilling] = useState<boolean | null>(null)

  // Local edits win over the fetched subscription until saved; the
  // fallbacks keep the payload total until the query resolves.
  const effPrice = price ?? sub?.price ?? ''
  const effCycle = cycle ?? sub?.billing_cycle ?? 'monthly'
  const effRenewal = renewal ?? sub?.renewal_date ?? ''
  const effAuto = autoBilling ?? sub?.auto_billing ?? false
  const effDays = cycleDays === '' ? (sub?.cycle_days ?? 0) : Number(cycleDays)
  const priceValid = /^\d+(\.\d{1,2})?$/.test(effPrice.trim())
  const daysValid = effCycle !== 'custom' || (effDays >= 1 && effDays <= 365)

  // Only the fields the owner actually touched: the audit entry
  // snapshots what the request names, so no-op fields would pollute
  // the ledger with null → null rows.
  const payload: Record<string, unknown> = {}
  if (price !== null && effPrice.trim() !== sub?.price) payload.price = effPrice.trim()
  if (cycle !== null && effCycle !== sub?.billing_cycle) payload.billing_cycle = effCycle
  if (
    cycle !== null ||
    cycleDays !== '' ||
    (effCycle === 'custom' && effDays !== (sub?.cycle_days ?? 0))
  ) {
    // Leaving a custom cycle always clears the days (D20 pairing
    // rule); entering one requires 1..365.
    payload.cycle_days = effCycle === 'custom' ? effDays : 0
  }
  if (renewal !== null && effRenewal !== (sub?.renewal_date ?? '')) payload.renewal_date = effRenewal
  if (autoBilling !== null && effAuto !== (sub?.auto_billing ?? false)) payload.auto_billing = effAuto

  const save = useMutation({
    mutationFn: () =>
      api<Subscription | undefined>(`/subscriptions/${subID}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      setError('')
      setPrice(null)
      setCycle(null)
      setCycleDays('')
      setRenewal(null)
      setAutoBilling(null)
      void qc.invalidateQueries({ queryKey: ['subscription', subID] })
    },
    onError: (err) => setError(err.message),
  })

  if (!sub) return <p className="text-slate-500">加载中…</p>

  const input = 'rounded border border-slate-300 px-2 py-1 text-xs'

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4">
      <div className="mb-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
        <div>
          <span className="mb-1 block text-xs font-medium text-slate-500">价格（{sub.currency}）</span>
          <input
            className={input}
            placeholder="如 15.00"
            value={effPrice}
            onChange={(e) => setPrice(e.target.value)}
          />
        </div>
        <div>
          <span className="mb-1 block text-xs font-medium text-slate-500">计费周期</span>
          <select className={input} value={effCycle} onChange={(e) => setCycle(e.target.value)}>
            <option value="monthly">按月</option>
            <option value="yearly">按年</option>
            <option value="custom">自定义天数</option>
          </select>
        </div>
        {effCycle === 'custom' && (
          <div>
            <span className="mb-1 block text-xs font-medium text-slate-500">周期天数</span>
            <input
              className={input}
              type="number"
              min={1}
              max={365}
              value={cycleDays === '' ? (sub.cycle_days ?? '') : cycleDays}
              onChange={(e) => setCycleDays(e.target.value)}
            />
          </div>
        )}
        <div>
          <span className="mb-1 block text-xs font-medium text-slate-500">自动续期日</span>
          <input
            className={input}
            type="date"
            value={effRenewal}
            onChange={(e) => setRenewal(e.target.value)}
          />
        </div>
        <div className="flex items-end">
          <label className="flex items-center gap-2 text-xs text-slate-600">
            <input
              type="checkbox"
              checked={effAuto}
              onChange={(e) => setAutoBilling(e.target.checked)}
            />
            到期自动滚动账期
          </label>
        </div>
      </div>
      <p className="mb-3 text-xs text-slate-400">
        开始日期 {sub.start_date} · 席位上限 {sub.max_seats}
        {sub.max_members ? ` · 成员上限 ${sub.max_members}` : ''}（不可在此修改）
      </p>
      {error && <p className="mb-2 text-sm text-red-600">{error}</p>}
      <button
        onClick={() => save.mutate()}
        disabled={save.isPending || !subID || !priceValid || !daysValid || Object.keys(payload).length === 0}
        className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
      >
        {save.isPending ? '保存中…' : Object.keys(payload).length === 0 ? '无改动' : '保存订阅设置'}
      </button>
    </div>
  )
}

// DisputesTab is the owner's dispute ledger (D14): members raise
// disputes from the contribution rows; the owner decides resolved or
// rejected — one-way, never mutating the contribution itself. The
// listing endpoint is owner-only, so the query lives in this tab.
function DisputesTab({
  subID,
  membersQuery,
}: {
  subID?: string
  membersQuery: UseQueryResult<ListResponse<Member>, Error>
}) {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const [deciding, setDeciding] = useState<string | null>(null)
  const [note, setNote] = useState('')

  const query = useQuery({
    queryKey: ['disputes', subID, status],
    queryFn: () =>
      api<ListResponse<Dispute>>(
        `/subscriptions/${subID}/disputes${status ? `?status=${status}` : ''}`,
      ),
    enabled: !!subID,
  })

  const decide = useMutation({
    mutationFn: (body: { id: string; decision: 'resolved' | 'rejected' }) =>
      api<Dispute>(`/disputes/${body.id}/decide`, {
        method: 'POST',
        body: JSON.stringify({ decision: body.decision, note }),
      }),
    onSuccess: () => {
      setError('')
      setDeciding(null)
      setNote('')
      void qc.invalidateQueries({ queryKey: ['disputes', subID] })
    },
    onError: (err) => setError(err.message),
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>

  const items = query.data?.items ?? []
  const nameOf = new Map(
    (membersQuery.data?.items ?? []).map((m) => [m.user_id, m.user_id.slice(0, 8) + '…']),
  )

  return (
    <div>
      <div className="mb-4 flex items-center gap-2">
        <select
          className="rounded border border-slate-300 px-2 py-1.5 text-sm"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="">全部状态</option>
          <option value="open">待裁决</option>
          <option value="resolved">已受理</option>
          <option value="rejected">已驳回</option>
        </select>
      </div>
      {error && <p className="mb-4 text-sm text-red-600">{error}</p>}
      {items.length === 0 ? (
        <p className="text-slate-500">暂无争议。成员可在账期分摊行上发起争议。</p>
      ) : (
        <div className="space-y-2">
          {items.map((d) => (
            <div key={d.id} className="rounded-xl border border-slate-200 bg-white p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm text-slate-900">
                    <span className="font-medium">{nameOf.get(d.raised_by) ?? d.raised_by.slice(0, 8) + '…'}</span>
                    <span className="ml-2 rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                      {d.status === 'open' ? '待裁决' : d.status === 'resolved' ? '已受理' : '已驳回'}
                    </span>
                  </p>
                  <p className="mt-1 text-sm text-slate-700">{d.reason}</p>
                  {d.evidence && <p className="mt-1 text-xs text-slate-500">凭证：{d.evidence}</p>}
                  <p className="mt-1 text-xs text-slate-400">
                    分摊 {d.contribution_id.slice(0, 8)}… · {new Date(d.created_at).toLocaleString()}
                  </p>
                  {d.status !== 'open' && d.resolution_note && (
                    <p className="mt-1 text-xs text-slate-500">裁决备注：{d.resolution_note}</p>
                  )}
                </div>
                {d.status === 'open' && (
                  <div className="shrink-0">
                    {deciding === d.id ? (
                      <span className="inline-flex items-center gap-1">
                        <input
                          className="w-32 rounded border border-slate-300 px-1 py-0.5 text-xs"
                          placeholder="裁决备注"
                          value={note}
                          onChange={(e) => setNote(e.target.value)}
                        />
                        <button
                          onClick={() => decide.mutate({ id: d.id, decision: 'resolved' })}
                          disabled={decide.isPending}
                          className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                        >
                          受理
                        </button>
                        <button
                          onClick={() => decide.mutate({ id: d.id, decision: 'rejected' })}
                          disabled={decide.isPending}
                          className="text-xs text-slate-600 hover:underline disabled:opacity-50"
                        >
                          驳回
                        </button>
                        <button
                          onClick={() => {
                            setDeciding(null)
                            setNote('')
                          }}
                          className="text-xs text-slate-500 hover:underline"
                        >
                          取消
                        </button>
                      </span>
                    ) : (
                      <button
                        onClick={() => {
                          setDeciding(d.id)
                          setNote('')
                        }}
                        className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800"
                      >
                        裁决
                      </button>
                    )}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// Mirrors internal/audit/model.go actions for display.


const AUDIT_PAGE = 20

// AuditTab is the owner's read-only audit ledger (D17): every money
// and membership change lands here in the same transaction that made
// it. Filter by action, page through, inspect before/after snapshots.
function AuditTab({ coterieId }: { coterieId: string }) {
  const [action, setAction] = useState('')
  const [offset, setOffset] = useState(0)

  const query = useQuery({
    queryKey: ['audit', coterieId, action, offset],
    queryFn: () => {
      const p = new URLSearchParams({ limit: String(AUDIT_PAGE), offset: String(offset) })
      if (action) p.set('action', action)
      return api<ListResponse<AuditLog>>(`/coteries/${coterieId}/audit-logs?${p}`)
    },
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>

  const items = query.data?.items ?? []
  const meta = query.data?.meta

  return (
    <div>
      <div className="mb-4 flex items-center gap-2">
        <select
          className="rounded border border-slate-300 px-2 py-1.5 text-sm"
          value={action}
          onChange={(e) => {
            setAction(e.target.value)
            setOffset(0)
          }}
        >
          <option value="">全部动作</option>
          {AUDIT_ACTIONS.map((a) => (
            <option key={a} value={a}>
              {AUDIT_LABEL[a] ?? a}
            </option>
          ))}
        </select>
        {meta && (
          <span className="text-xs text-slate-400">
            共 {meta.total} 条 · 第 {Math.floor(offset / AUDIT_PAGE) + 1} 页
          </span>
        )}
      </div>
      {items.length === 0 ? (
        <p className="text-slate-500">暂无审计记录。</p>
      ) : (
        <div className="space-y-2">
          {items.map((l) => (
            <div key={l.id} className="rounded-xl border border-slate-200 bg-white p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm">
                  <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                    {AUDIT_LABEL[l.action] ?? l.action}
                  </span>
                  <span className="ml-2 text-xs text-slate-500">
                    操作人 {(l.actor_id ?? 'system').slice(0, 8)}…
                    <span className="ml-1 text-slate-400">
                      {l.entity_type} {l.entity_id.slice(0, 8)}…
                    </span>
                  </span>
                </p>
                <span className="shrink-0 text-xs text-slate-400">
                  {new Date(l.created_at).toLocaleString()}
                </span>
              </div>
              {(l.before || l.after) && (
                <p className="mt-1 break-all font-mono text-xs text-slate-500">
                  {l.before && <span>− {JSON.stringify(l.before)} </span>}
                  {l.after && <span>+ {JSON.stringify(l.after)}</span>}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
      {meta && meta.total > AUDIT_PAGE && (
        <div className="mt-4 flex items-center gap-3">
          <button
            onClick={() => setOffset(Math.max(0, offset - AUDIT_PAGE))}
            disabled={offset === 0}
            className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
          >
            上一页
          </button>
          <button
            onClick={() => setOffset(offset + AUDIT_PAGE)}
            disabled={offset + AUDIT_PAGE >= meta.total}
            className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
          >
            下一页
          </button>
        </div>
      )}
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
  const router = useRouter()
  const [inviteToken, setInviteToken] = useState('')
  const [error, setError] = useState('')
  const [confirming, setConfirming] = useState<string | null>(null)
  const [leaving, setLeaving] = useState(false)

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
      void qc.invalidateQueries({ queryKey: ['invitations', coterieId] })
    },
    onError: (err) => setError(err.message),
  })

  // Invitation history answers "did my invite land?" — tokens never
  // re-show, but accepted_at/expire_at decide each row's state.
  // queryFn runs outside render, so the one "now" snapshot needed to
  // judge expiry stays out of the render tree (react/purity).
  const invitationsQuery = useQuery({
    queryKey: ['invitations', coterieId],
    queryFn: async () => {
      const res = await api<ListResponse<Invitation>>(`/coteries/${coterieId}/invitations`)
      const now = Date.now()
      const expiredIds = new Set(
        res.items
          .filter((inv) => new Date(inv.expire_at).getTime() < now)
          .map((inv) => inv.id),
      )
      return { items: res.items, expiredIds }
    },
    enabled: isOwner,
  })

  const remove = useMutation({
    mutationFn: (memberID: string) => api<void>(`/members/${memberID}`, { method: 'DELETE' }),
    onSuccess: () => {
      setError('')
      setConfirming(null)
      void qc.invalidateQueries({ queryKey: ['members', coterieId] })
      void qc.invalidateQueries({ queryKey: ['coterie', coterieId] })
      void qc.invalidateQueries({ queryKey: ['seats'] })
    },
    onError: (err) => setError(err.message),
  })

  const leave = useMutation({
    mutationFn: () => api<void>(`/coteries/${coterieId}/leave`, { method: 'POST' }),
    onSuccess: () => {
      // The circle is gone from this user's view; the list refetches
      // fresh on arrival.
      void qc.invalidateQueries({ queryKey: ['coteries'] })
      router.navigate({ to: '/coteries' })
    },
    onError: (err) => setError(err.message),
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []
  const me = getUser()
  const myMember = me ? items.find((m) => m.user_id === me.id) : undefined

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
          {invitationsQuery.data && invitationsQuery.data.items.length > 0 && (
            <div className="mt-3">
              <p className="mb-1 text-xs font-medium text-slate-500">邀请记录</p>
              <ul className="max-w-xl space-y-1">
                {invitationsQuery.data.items.map((inv) => {
                  const state = inv.accepted_at
                    ? 'accepted'
                    : invitationsQuery.data.expiredIds.has(inv.id)
                      ? 'expired'
                      : 'pending'
                  const stateLabel =
                    state === 'accepted' ? '已接受' : state === 'expired' ? '已过期' : '待使用'
                  return (
                    <li
                      key={inv.id}
                      className="flex items-center justify-between rounded border border-slate-100 bg-slate-50 px-2 py-1 text-xs text-slate-600"
                    >
                      <span>
                        {new Date(inv.created_at).toLocaleString()} · {inv.role}
                        {state === 'pending' && (
                          <span className="ml-1 text-slate-400">
                            （{new Date(inv.expire_at).toLocaleString()} 前有效）
                          </span>
                        )}
                      </span>
                      <span
                        className={
                          state === 'accepted'
                            ? 'text-emerald-700'
                            : state === 'expired'
                              ? 'text-slate-400'
                              : 'text-amber-700'
                        }
                      >
                        {stateLabel}
                      </span>
                    </li>
                  )
                })}
              </ul>
            </div>
          )}
        </div>
      )}
      {!isOwner && (
        <div className="mb-4 flex items-center gap-2">
          {leaving ? (
            <>
              <span className="text-sm text-slate-600">退出后席位将释放，确认退出？</span>
              <button
                onClick={() => leave.mutate()}
                disabled={leave.isPending}
                className="rounded-lg bg-rose-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-rose-800 disabled:opacity-50"
              >
                确认退出
              </button>
              <button
                onClick={() => setLeaving(false)}
                className="text-sm text-slate-500 hover:underline"
              >
                取消
              </button>
            </>
          ) : (
            <button
              onClick={() => setLeaving(true)}
              className="rounded-lg border border-rose-300 px-3 py-1.5 text-sm text-rose-700 hover:bg-rose-50"
            >
              退出共享圈
            </button>
          )}
          {!isOwner && error && <span className="text-sm text-red-600">{error}</span>}
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
              {isOwner && <th className="py-2 font-medium" />}
            </tr>
          </thead>
          <tbody>
            {items.map((m) => (
              <tr key={m.id} className="border-b border-slate-100">
                <td className="py-2 font-mono text-xs text-slate-700">
                  {m.user_id.slice(0, 8)}
                  {me && m.user_id === me.id && (
                    <span className="ml-1 text-slate-400">（我）</span>
                  )}
                </td>
                <td className="py-2">{m.role}</td>
                <td className="py-2 text-slate-500">{new Date(m.joined_at).toLocaleDateString()}</td>
                {isOwner && (
                  <td className="py-2 text-right">
                    {m.role !== 'owner' &&
                      (confirming === m.id ? (
                        <span className="inline-flex items-center gap-1">
                          <span className="text-xs text-slate-600">确认移除？</span>
                          <button
                            onClick={() => remove.mutate(m.id)}
                            disabled={remove.isPending}
                            className="text-xs text-rose-700 hover:underline disabled:opacity-50"
                          >
                            移除
                          </button>
                          <button
                            onClick={() => setConfirming(null)}
                            className="text-xs text-slate-500 hover:underline"
                          >
                            取消
                          </button>
                        </span>
                      ) : (
                        <button
                          onClick={() => setConfirming(m.id)}
                          className="text-xs text-rose-700 hover:underline"
                        >
                          移除
                        </button>
                      ))}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {myMember && <p className="mt-2 text-xs text-slate-400">成员 {items.length} 人。</p>}
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
  const [editing, setEditing] = useState<string | null>(null)
  const [editLabel, setEditLabel] = useState('')
  const [releasing, setReleasing] = useState<string | null>(null)
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

  // updateSeat is the PATCH /seats/{id} surface: relabel, or take a
  // seat out of circulation (free⇄disabled; occupied seats go through
  // assign/release instead). Both land as seat_updated audit entries.
  const updateSeat = useMutation({
    mutationFn: ({ seatID, body }: { seatID: string; body: Record<string, unknown> }) =>
      api<Seat>(`/seats/${seatID}`, { method: 'PATCH', body: JSON.stringify(body) }),
    onSuccess: () => {
      setError('')
      setEditing(null)
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  // releaseSeat frees an occupied seat — the only way back from
  // occupied besides the member leaving; disabling one is a 409.
  const releaseSeat = useMutation({
    mutationFn: (seatID: string) => api<Seat>(`/seats/${seatID}/release`, { method: 'POST' }),
    onSuccess: () => {
      setError('')
      setReleasing(null)
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
                <td className="py-2">
                  {editing === s.id ? (
                    <input
                      className="w-32 rounded border border-slate-300 px-1 py-0.5 text-xs"
                      value={editLabel}
                      onChange={(e) => setEditLabel(e.target.value)}
                    />
                  ) : (
                    s.label
                  )}
                </td>
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
                    {assigning !== s.id && editing === s.id && (
                      <span className="inline-flex items-center gap-1">
                        <button
                          onClick={() =>
                            updateSeat.mutate({ seatID: s.id, body: { label: editLabel.trim() } })
                          }
                          disabled={updateSeat.isPending || !editLabel.trim()}
                          className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                        >
                          保存
                        </button>
                        <button
                          onClick={() => setEditing(null)}
                          className="text-xs text-slate-500 hover:underline"
                        >
                          取消
                        </button>
                      </span>
                    )}
                    {assigning !== s.id && editing !== s.id && (
                      <>
                        <button
                          onClick={() => {
                            setEditing(s.id)
                            setEditLabel(s.label)
                          }}
                          className="ml-2 text-xs text-slate-600 hover:underline"
                        >
                          编辑
                        </button>
                        {s.status !== 'occupied' &&
                          (s.status === 'free' ? (
                            <button
                              onClick={() =>
                                updateSeat.mutate({ seatID: s.id, body: { status: 'disabled' } })
                              }
                              disabled={updateSeat.isPending}
                              className="ml-2 text-xs text-slate-500 hover:underline disabled:opacity-50"
                            >
                              停用
                            </button>
                          ) : (
                            <button
                              onClick={() =>
                                updateSeat.mutate({ seatID: s.id, body: { status: 'free' } })
                              }
                              disabled={updateSeat.isPending}
                              className="ml-2 text-xs text-slate-500 hover:underline disabled:opacity-50"
                            >
                              启用
                            </button>
                          ))}
                        {s.status === 'occupied' &&
                          (releasing === s.id ? (
                            <span className="inline-flex items-center gap-1">
                              <button
                                onClick={() => releaseSeat.mutate(s.id)}
                                disabled={releaseSeat.isPending}
                                className="text-xs text-red-600 hover:underline disabled:opacity-50"
                              >
                                确认释放
                              </button>
                              <button
                                onClick={() => setReleasing(null)}
                                className="text-xs text-slate-500 hover:underline"
                              >
                                取消
                              </button>
                            </span>
                          ) : (
                            <button
                              onClick={() => setReleasing(s.id)}
                              className="ml-2 text-xs text-slate-500 hover:underline"
                            >
                              释放
                            </button>
                          ))}
                      </>
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
  membersQuery,
  isOwner,
}: {
  subID?: string
  periods: BillingPeriod[]
  queries: UseQueryResult<ListResponse<Contribution>, Error>[]
  membersQuery: UseQueryResult<ListResponse<Member>, Error>
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
  const [adjusting, setAdjusting] = useState<string | null>(null)
  const [adjAmount, setAdjAmount] = useState('')
  const [adjStatus, setAdjStatus] = useState('pending')
  const [payHistory, setPayHistory] = useState<string | null>(null)
  const [disputing, setDisputing] = useState<string | null>(null)
  const [disputeReason, setDisputeReason] = useState('')
  const [disputeEvidence, setDisputeEvidence] = useState('')

  // member_id on a contribution is the member row id; the raise check
  // and the 争议 button both care about the current user behind it.
  const myUserID = getUser()?.id
  const userIDOf = new Map(
    (membersQuery.data?.items ?? []).map((m) => [m.id, m.user_id]),
  )

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['periods', subID] })
    void qc.invalidateQueries({ queryKey: ['contributions'] })
    void qc.invalidateQueries({ queryKey: ['disputes', subID] })
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

  const raiseDispute = useMutation({
    mutationFn: (contributionID: string) =>
      api<Dispute>(`/contributions/${contributionID}/disputes`, {
        method: 'POST',
        body: JSON.stringify({ reason: disputeReason, evidence: disputeEvidence }),
      }),
    onSuccess: () => {
      setError('')
      setDisputing(null)
      setDisputeReason('')
      setDisputeEvidence('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  // adjust is the owner's manual settlement patch (PATCH
  // /contributions/{id}): re-amount a pending share or flip its
  // status; both land in the audit ledger as contribution_updated.
  const adjust = useMutation({
    mutationFn: (contributionID: string) =>
      api<Contribution>(`/contributions/${contributionID}`, {
        method: 'PATCH',
        body: JSON.stringify({ amount: adjAmount, status: adjStatus }),
      }),
    onSuccess: () => {
      setError('')
      setAdjusting(null)
      refresh()
    },
    onError: (err) => setError(err.message),
  })
  const adjAmountValid = /^\d+(\.\d{1,2})?$/.test(adjAmount.trim())

  // Payment history for one contribution (D25): the records behind a
  // settled row, incl. manual entries with their external reference.
  const paymentsQuery = useQuery({
    queryKey: ['payments', payHistory],
    queryFn: () => api<ListResponse<PaymentRecord>>(`/contributions/${payHistory}/payments`),
    enabled: !!payHistory,
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
                      {rows.map((c) => {
                        const mine = userIDOf.get(c.member_id) === myUserID
                        return (
                        <Fragment key={c.id}>
                        <tr className="border-t border-slate-100">
                          <td className="py-1.5 font-mono text-xs text-slate-700">
                            {c.member_id.slice(0, 8)}…
                          </td>
                          <td className="py-1.5 text-right">
                            {c.amount} {c.currency}
                          </td>
                          <td className="py-1.5 text-right text-slate-500">{c.status}</td>
                          {isOwner && c.status === 'pending' && (
                            <td className="py-1.5 text-right">
                              {adjusting === c.id ? (
                                <span className="inline-flex items-center gap-1">
                                  <input
                                    className="w-20 rounded border border-slate-300 px-1 py-0.5 text-xs"
                                    placeholder="金额"
                                    value={adjAmount}
                                    onChange={(e) => setAdjAmount(e.target.value)}
                                  />
                                  <select
                                    className="rounded border border-slate-300 px-1 py-0.5 text-xs"
                                    value={adjStatus}
                                    onChange={(e) => setAdjStatus(e.target.value)}
                                  >
                                    <option value="pending">pending</option>
                                    <option value="paid">paid</option>
                                    <option value="waived">waived</option>
                                    <option value="cancelled">cancelled</option>
                                  </select>
                                  <button
                                    onClick={() => adjust.mutate(c.id)}
                                    disabled={adjust.isPending || !adjAmountValid}
                                    className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                                  >
                                    确认
                                  </button>
                                  <button
                                    onClick={() => setAdjusting(null)}
                                    className="text-xs text-slate-500 hover:underline"
                                  >
                                    取消
                                  </button>
                                </span>
                              ) : paying === c.id ? (
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
                                <span className="inline-flex items-center gap-2">
                                  <button
                                    onClick={() => {
                                      setPaying(c.id)
                                      setPayRef('')
                                    }}
                                    className="text-xs text-emerald-700 hover:underline"
                                  >
                                    登记收款
                                  </button>
                                  {p.status === 'open' && (
                                    <button
                                      onClick={() => {
                                        setAdjusting(c.id)
                                        setAdjAmount(c.amount)
                                        setAdjStatus(c.status)
                                      }}
                                      className="text-xs text-slate-600 hover:underline"
                                    >
                                      调整
                                    </button>
                                  )}
                                  <button
                                    onClick={() => setPayHistory(payHistory === c.id ? null : c.id)}
                                    className="text-xs text-slate-500 hover:underline"
                                  >
                                    收款记录
                                  </button>
                                </span>
                              )}
                            </td>
                          )}
                          {mine && c.status !== 'cancelled' && (
                            <td className="py-1.5 text-right">
                              {disputing === c.id ? (
                                <span className="inline-flex flex-col items-end gap-1">
                                  <span className="inline-flex items-center gap-1">
                                    <input
                                      className="w-40 rounded border border-slate-300 px-1 py-0.5 text-xs"
                                      placeholder="争议原因（必填）"
                                      value={disputeReason}
                                      onChange={(e) => setDisputeReason(e.target.value)}
                                    />
                                    <input
                                      className="w-40 rounded border border-slate-300 px-1 py-0.5 text-xs"
                                      placeholder="凭证/说明（选填）"
                                      value={disputeEvidence}
                                      onChange={(e) => setDisputeEvidence(e.target.value)}
                                    />
                                    <button
                                      onClick={() => raiseDispute.mutate(c.id)}
                                      disabled={raiseDispute.isPending || !disputeReason.trim()}
                                      className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                                    >
                                      提交
                                    </button>
                                    <button
                                      onClick={() => {
                                        setDisputing(null)
                                        setDisputeReason('')
                                        setDisputeEvidence('')
                                      }}
                                      className="text-xs text-slate-500 hover:underline"
                                    >
                                      取消
                                    </button>
                                  </span>
                                </span>
                              ) : (
                                <button
                                  onClick={() => {
                                    setDisputing(c.id)
                                    setDisputeReason('')
                                    setDisputeEvidence('')
                                  }}
                                  className="text-xs text-slate-600 hover:underline"
                                >
                                  争议
                                </button>
                              )}
                            </td>
                          )}
                        </tr>
                        {payHistory === c.id && (
                          <tr>
                            <td colSpan={5} className="py-1.5">
                              {paymentsQuery.isLoading ? (
                                <p className="text-xs text-slate-400">加载中…</p>
                              ) : (paymentsQuery.data?.items ?? []).length === 0 ? (
                                <p className="text-xs text-slate-400">暂无收款记录。</p>
                              ) : (
                                <table className="w-full text-xs">
                                  <tbody>
                                    {(paymentsQuery.data?.items ?? []).map((pay) => (
                                      <tr key={pay.id} className="border-t border-slate-100">
                                        <td className="py-1 text-slate-700">{pay.method}</td>
                                        <td className="py-1 text-slate-500">{pay.status}</td>
                                        <td className="py-1 text-slate-500">
                                          {pay.external_ref ?? '—'}
                                        </td>
                                        <td className="py-1 text-slate-500">
                                          {pay.paid_at
                                            ? new Date(pay.paid_at).toLocaleString()
                                            : new Date(pay.created_at).toLocaleString()}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              )}
                            </td>
                          </tr>
                        )}
                        </Fragment>
                        )
                      })}
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
