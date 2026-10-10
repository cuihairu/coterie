import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { Report } from '../../lib/api'

export const Route = createFileRoute('/_app/admin/')({
  component: AdminPage,
})

type Status = 'open' | 'resolved' | 'dismissed'

// Platform admin inbox (FR-15): flags on publicly listed circles. The
// admin triages by coterie/reporter identity and decides resolve or
// dismiss; deciding never touches the coterie itself.
function AdminPage() {
  const qc = useQueryClient()
  const [status, setStatus] = useState<Status>('open')
  const [error, setError] = useState('')
  const [deciding, setDeciding] = useState<string | null>(null)
  const [note, setNote] = useState('')

  const reportsQuery = useQuery({
    queryKey: ['reports', status],
    queryFn: () => api<{ items: Report[]; meta: { total: number } }>(`/admin/reports?status=${status}`),
  })

  const decide = useMutation({
    mutationFn: ({ id, verdict }: { id: string; verdict: 'resolve' | 'dismiss' }) =>
      api<unknown>(`/admin/reports/${id}/${verdict}`, {
        method: 'POST',
        body: JSON.stringify({ note }),
      }),
    onSuccess: () => {
      setError('')
      setDeciding(null)
      setNote('')
      void qc.invalidateQueries({ queryKey: ['reports'] })
    },
    onError: (err) => setError(err.message),
  })

  const items = reportsQuery.data?.items ?? []

  return (
    <div>
      <h1 className="mb-1 text-xl font-semibold text-slate-900">举报处置</h1>
      <p className="mb-4 text-sm text-slate-500">平台管理员处置对公开共享圈的举报。</p>
      <div className="mb-4 flex gap-2">
        {(['open', 'resolved', 'dismissed'] as Status[]).map((s) => (
          <button
            key={s}
            onClick={() => setStatus(s)}
            className={`rounded-full px-3 py-1 text-xs font-medium ${
              status === s ? 'bg-slate-900 text-white' : 'bg-white text-slate-600 hover:bg-slate-100'
            }`}
          >
            {s === 'open' ? '待处置' : s === 'resolved' ? '已解决' : '已驳回'}
          </button>
        ))}
      </div>
      {reportsQuery.isLoading ? (
        <p className="text-slate-500">加载中…</p>
      ) : reportsQuery.error ? (
        <p className="text-red-600">加载失败：{(reportsQuery.error as Error).message}</p>
      ) : items.length === 0 ? (
        <p className="text-slate-500">暂无举报。</p>
      ) : (
        <div className="space-y-3">
          {items.map((r) => (
            <div key={r.id} className="rounded-xl border border-slate-200 bg-white p-4">
              <div className="mb-1 flex items-center justify-between">
                <span className="font-medium text-slate-900">{r.coterie_name ?? r.coterie_id}</span>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs ${
                    r.status === 'open'
                      ? 'bg-amber-50 text-amber-700'
                      : r.status === 'resolved'
                        ? 'bg-emerald-50 text-emerald-700'
                        : 'bg-slate-100 text-slate-500'
                  }`}
                >
                  {r.status}
                </span>
              </div>
              <p className="text-sm text-slate-700">{r.reason}</p>
              <p className="mt-1 text-xs text-slate-400">
                举报人 {r.reporter_username ?? r.reporter_id}
                {r.reporter_email ? `（${r.reporter_email}）` : ''} · {new Date(r.created_at).toLocaleString()}
              </p>
              {r.resolution_note && (
                <p className="mt-1 text-xs text-slate-500">处置备注：{r.resolution_note}</p>
              )}
              {r.status === 'open' && (
                <div className="mt-2">
                  {deciding === r.id ? (
                    <span className="inline-flex items-center gap-1">
                      <input
                        className="w-56 rounded border border-slate-300 px-1.5 py-0.5 text-xs"
                        placeholder="处置备注（选填）"
                        value={note}
                        onChange={(e) => setNote(e.target.value)}
                      />
                      <button
                        onClick={() => decide.mutate({ id: r.id, verdict: 'resolve' })}
                        disabled={decide.isPending}
                        className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                      >
                        确认解决
                      </button>
                      <button
                        onClick={() => decide.mutate({ id: r.id, verdict: 'dismiss' })}
                        disabled={decide.isPending}
                        className="text-xs text-slate-600 hover:underline disabled:opacity-50"
                      >
                        确认驳回
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
                        setDeciding(r.id)
                        setNote('')
                      }}
                      className="text-xs text-slate-600 hover:underline"
                    >
                      处置
                    </button>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
    </div>
  )
}
