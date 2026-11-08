import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api } from '../../lib/api'
import type { Dispute, ListResponse, Member } from '../../lib/api'

export function DisputesTab({
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
                  <p className="mt-1 text-xs text-slate-600">
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
