import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { DirectoryEntry, ListResponse } from '../../lib/api'

export const Route = createFileRoute('/_app/marketplace/')({
  component: MarketplacePage,
})

// Read-only directory (FR-16): publicly listed circles with the owner
// reputation badge (D18) and the equal-split share estimate, plus the
// join-request action that starts the Request → accept journey.
function MarketplacePage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['marketplace'],
    queryFn: () => api<ListResponse<DirectoryEntry>>('/marketplace/coteries'),
  })

  if (isLoading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">加载失败：{error.message}</p>

  const items = data?.items ?? []

  return (
    <div>
      <h1 className="mb-1 text-xl font-semibold text-slate-900">共享广场</h1>
      <p className="mb-4 text-sm text-slate-500">公开目录中的共享圈，可申请加入。</p>
      {items.length === 0 ? (
        <p className="text-slate-500">暂无公开圈。圈 Owner 可在圈详情中把圈设为公开。</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {items.map((e) => (
            <EntryCard key={e.coterie_id} entry={e} />
          ))}
        </div>
      )}
    </div>
  )
}

function EntryCard({ entry }: { entry: DirectoryEntry }) {
  const [error, setError] = useState('')
  const [requestID, setRequestID] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)

  const request = useMutation({
    mutationFn: () =>
      api<{ id: string }>(`/coteries/${entry.coterie_id}/join-requests`, {
        method: 'POST',
        body: JSON.stringify({}),
      }),
    onSuccess: (r) => {
      setError('')
      setRequestID(r.id)
    },
    onError: (err) => setError(err.message),
  })

  const cancel = useMutation({
    mutationFn: () => api<void>(`/join-requests/${requestID}`, { method: 'DELETE' }),
    onSuccess: () => {
      setError('')
      setRequestID(null)
      setConfirming(false)
    },
    onError: (err) => setError(err.message),
  })

  const badge = entry.owner

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-1 flex items-center justify-between">
        <h2 className="font-medium text-slate-900">{entry.name}</h2>
        <span
          className={`rounded-full px-2 py-0.5 text-xs ${
            entry.full ? 'bg-slate-100 text-slate-500' : 'bg-emerald-50 text-emerald-700'
          }`}
        >
          {entry.full ? '已满' : '可加入'}
        </span>
      </div>
      <p className="text-sm text-slate-500">
        {entry.provider_name} · {entry.product_name}
      </p>
      <p className="mt-1 text-sm text-slate-700">
        估价 <span className="font-medium">{entry.share_estimate}</span> {entry.currency}
        <span className="text-xs text-slate-400"> /期（{entry.price} 均摊）</span>
      </p>
      <p className="mt-1 text-xs text-slate-400">
        成员 {entry.member_count} · 席位 {entry.seats_free}/{entry.seats_total} 空闲
      </p>
      {badge && (
        <p className="mt-1 text-xs text-slate-400">
          Owner {badge.username} · 结算信誉 {badge.payment_ratio ?? '—'}
          （已缴 {badge.contributions.paid ?? 0} / 待缴 {badge.contributions.pending ?? 0}）
        </p>
      )}
      <div className="mt-3">
        {entry.full ? (
          <span className="text-xs text-slate-400">圈已满员。</span>
        ) : requestID ? (
          <>
            <span className="text-xs text-emerald-700">已提交申请，等待 Owner 处理。</span>
            {confirming ? (
              <>
                <button
                  onClick={() => cancel.mutate()}
                  disabled={cancel.isPending}
                  className="ml-2 rounded-lg bg-red-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50"
                >
                  {cancel.isPending ? '撤回中…' : '确认撤回'}
                </button>
                <button
                  onClick={() => setConfirming(false)}
                  className="ml-1 rounded-lg border border-slate-300 px-2.5 py-1 text-xs text-slate-600 hover:bg-slate-50"
                >
                  保留
                </button>
              </>
            ) : (
              <button
                onClick={() => setConfirming(true)}
                disabled={cancel.isPending}
                className="ml-2 rounded-lg border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
              >
                撤回申请
              </button>
            )}
          </>
        ) : (
          <button
            onClick={() => request.mutate()}
            disabled={request.isPending}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {request.isPending ? '提交中…' : '申请加入'}
          </button>
        )}
        {error && <span className="ml-2 text-sm text-red-600">{error}</span>}
      </div>
    </div>
  )
}
