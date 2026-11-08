import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { UsageCapsEditor } from '../UsageCapsEditor'
import type { ListResponse, Member, SharingPolicy, Subscription, UsageLimit, UsageRecord } from '../../lib/api'

export function UsageTab({
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
    mutationFn: async ({ kind, limit }: { kind: 'usage' | 'quota'; limit: UsageLimit | null }) => {
      const sub = subscriptionQuery.data
      const policy: SharingPolicy = { ...(sub?.sharing_policy ?? {}) }
      const key = kind === 'usage' ? 'usage_limit' : 'quota_limit'
      if (limit) {
        policy[key] = limit
      } else {
        delete policy[key]
      }
      return api<Subscription>(`/subscriptions/${subID}`, {
        method: 'PATCH',
        body: JSON.stringify({ sharing_policy: policy }),
      })
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['subscription', subID] })
    },
  })

  if (subscriptionQuery.isLoading) return <p className="text-slate-500">加载中…</p>
  if (subscriptionQuery.error)
    return <p className="text-red-600">加载失败：{subscriptionQuery.error.message}</p>

  const limit = subscriptionQuery.data?.sharing_policy?.usage_limit
  const quotaLimit = subscriptionQuery.data?.sharing_policy?.quota_limit
  const members = membersQuery.data?.items ?? []
  // member_id on a record is the member row id; map it to the user id
  // the member table displays.
  const nameOf = new Map(members.map((m) => [m.id, m.user_id.slice(0, 8) + '…']))
  const records = usageQuery.data?.items ?? []

  return (
    <div>
      <div className="mb-4 rounded-lg border border-slate-200 bg-white p-4">
        <UsageCapsEditor
          caps={[
            { kind: 'usage', label: '每成员每期用量上限', hint: '超出记录返回 409', cap: limit },
            {
              kind: 'quota',
              label: '全订阅每期用量上限',
              hint: '全体成员合计超出返回 409',
              cap: quotaLimit,
            },
          ]}
          isOwner={isOwner}
          onSave={async (kind, cap) => {
            await saveLimit.mutateAsync({ kind, limit: cap })
          }}
        />
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
