import { Fragment, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api, getUser } from '../../lib/api'
import { chargeMethodFor, PaymentMethodSelect } from '../PaymentMethodSelect'
import type { BillingPeriod, Contribution, Dispute, ListResponse, Member, PaymentRecord } from '../../lib/api'

export function PeriodsTab({
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
  // 拆分方式（D9）：均摊 / 按席位 / 按用量 / 按天数比例 / 手动指定。
  const [genMode, setGenMode] = useState<'equal' | 'per_seat' | 'usage' | 'prorated' | 'fixed'>('equal')
  const [fixedShares, setFixedShares] = useState<Record<string, string>>({})
  const members = membersQuery.data?.items ?? []
  // fixed 模式下留空的成员本期无分摊，至少给出一位成员即可生成。
  const fixedReady = Object.values(fixedShares).some((amount) =>
    /^\d+(\.\d{1,2})?$/.test(amount.trim()),
  )
  const [payMethod, setPayMethod] = useState('')
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

  // The charge can go through any configured channel, not just manual —
  // the list is the server's registered adapters (manual is always on).
  const methodsQuery = useQuery({
    queryKey: ['payment-methods'],
    queryFn: () => api<ListResponse<string>>('/payments/methods'),
  })
  const methods = methodsQuery.data?.items ?? []
  const chargeMethod = chargeMethodFor(methods, payMethod)

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
    mutationFn: ({ periodID }: { periodID: string }) => {
      const body: Record<string, unknown> = { mode: genMode }
      if (genMode === 'fixed') {
        body.items = Object.entries(fixedShares)
          .filter(([, amount]) => amount.trim() !== '')
          .map(([member_id, amount]) => ({ member_id, amount: amount.trim() }))
      }
      return api<ListResponse<Contribution>>(`/billing-periods/${periodID}/contributions/generate`, {
        method: 'POST',
        body: JSON.stringify(body),
      })
    },
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
        body: JSON.stringify({ method: chargeMethod, external_ref: payRef }),
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
          <span className="text-xs text-slate-600">至</span>
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
                        <select
                          value={genMode}
                          onChange={(e) =>
                            setGenMode(e.target.value as 'equal' | 'per_seat' | 'usage' | 'prorated' | 'fixed')
                          }
                          className="rounded border border-slate-300 px-1 py-0.5 text-xs"
                          title="分摊方式（D9）"
                        >
                          <option value="equal">均摊</option>
                          <option value="per_seat">按席位</option>
                          <option value="usage">按用量</option>
                          <option value="prorated">按天数比例</option>
                          <option value="fixed">手动指定</option>
                        </select>
                        <button
                          onClick={() => generate.mutate({ periodID: p.id })}
                          disabled={generate.isPending || (genMode === 'fixed' && !fixedReady)}
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
                {isOwner && p.status === 'open' && genMode === 'fixed' && members.length > 0 && (
                  <div className="mb-2 rounded-lg bg-slate-50 p-2 text-xs text-slate-600">
                    <div className="mb-1 text-slate-500">手动指定本期每位成员的金额（留空的成员本期无分摊）：</div>
                    <div className="flex flex-wrap gap-2">
                      {members.map((m) => (
                        <label key={m.id} className="inline-flex items-center gap-1">
                          <span className="font-mono">{m.id.slice(0, 8)}…</span>
                          <input
                            className="w-16 rounded border border-slate-300 px-1 py-0.5"
                            placeholder="0.00"
                            value={fixedShares[m.id] ?? ''}
                            onChange={(e) =>
                              setFixedShares({ ...fixedShares, [m.id]: e.target.value })
                            }
                          />
                        </label>
                      ))}
                    </div>
                  </div>
                )}
                {rows.length === 0 ? (
                  <p className="text-sm text-slate-600">本期无分摊。</p>
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
                                  <PaymentMethodSelect
                                    methods={methods}
                                    value={chargeMethod}
                                    onChange={setPayMethod}
                                  />
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
                                <p className="text-xs text-slate-600">加载中…</p>
                              ) : (paymentsQuery.data?.items ?? []).length === 0 ? (
                                <p className="text-xs text-slate-600">暂无收款记录。</p>
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
