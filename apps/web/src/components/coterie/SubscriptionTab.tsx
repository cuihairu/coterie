import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api } from '../../lib/api'
import type { Subscription } from '../../lib/api'

export function SubscriptionTab({
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
      <p className="mb-3 text-xs text-slate-600">
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
