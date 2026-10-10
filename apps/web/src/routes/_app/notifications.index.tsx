import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { ListResponse, Notification } from '../../lib/api'

export const Route = createFileRoute('/_app/notifications/')({
  component: NotificationsPage,
})

function NotificationsPage() {
  const qc = useQueryClient()
  const { data, isLoading, error } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api<ListResponse<Notification>>('/notifications'),
  })

  const markRead = useMutation({
    mutationFn: (id: string) => api<void>(`/notifications/${id}/read`, { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['notifications'] }),
  })

  if (isLoading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">加载失败：{error.message}</p>

  const items = data?.items ?? []

  return (
    <div>
      <h1 className="mb-4 text-xl font-semibold text-slate-900">通知</h1>
      {items.length === 0 ? (
        <p className="text-slate-500">暂无通知。</p>
      ) : (
        <div className="space-y-2">
          {items.map((n) => {
            const unread = !n.read_at
            return (
              <div
                key={n.id}
                className={`rounded-xl border bg-white p-4 ${
                  unread ? 'border-emerald-300' : 'border-slate-200'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                        {n.type}
                      </span>
                      <h2 className={`text-sm ${unread ? 'font-medium text-slate-900' : 'text-slate-700'}`}>
                        {n.title}
                      </h2>
                    </div>
                    {n.body && <p className="mt-1 text-sm text-slate-500">{n.body}</p>}
                    <p className="mt-1 text-xs text-slate-400">
                      {new Date(n.created_at).toLocaleString()}
                    </p>
                  </div>
                  {unread && (
                    <button
                      onClick={() => markRead.mutate(n.id)}
                      disabled={markRead.isPending}
                      className="shrink-0 rounded-lg border border-slate-300 px-2 py-1 text-xs text-slate-600 hover:bg-slate-50 disabled:opacity-50"
                    >
                      标为已读
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
