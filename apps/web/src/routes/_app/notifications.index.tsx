import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { ListResponse, Notification } from '../../lib/api'
import {
  activePushSubscription,
  disablePush,
  enablePush,
  pushSupported,
} from '../../lib/push'
import { Pager } from '../../components/Pager'
import { PushSetup } from '../../components/PushSetup'

const PAGE_SIZE = 20

export const Route = createFileRoute('/_app/notifications/')({
  component: NotificationsPage,
})

function NotificationsPage() {
  const qc = useQueryClient()
  // The inbox grows without bound; page it or older notifications
  // silently drop off past the server's default limit.
  const [offset, setOffset] = useState(0)
  const { data, isLoading, error } = useQuery({
    queryKey: ['notifications', 'list', offset],
    queryFn: () =>
      api<ListResponse<Notification>>(
        `/notifications?${new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) })}`,
      ),
  })

  const markRead = useMutation({
    mutationFn: (id: string) => api<void>(`/notifications/${id}/read`, { method: 'POST' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['notifications'] }),
  })

  // Push lives in the browser, not the query cache: the subscription
  // is this origin's PushSubscription, so a plain refresh on toggle.
  const [supported] = useState(pushSupported)
  const [pushSub, setPushSub] = useState<PushSubscription | null>(null)
  const [pushMessage, setPushMessage] = useState('')
  const [pushBusy, setPushBusy] = useState(false)
  useEffect(() => {
    if (!supported) return
    void activePushSubscription().then(setPushSub)
  }, [supported])

  const enable = async () => {
    setPushBusy(true)
    setPushMessage('')
    const result = await enablePush()
    setPushBusy(false)
    if (result.ok) setPushSub(await activePushSubscription())
    else setPushMessage(result.reason)
  }
  const disable = async () => {
    setPushBusy(true)
    setPushMessage('')
    try {
      await disablePush()
    } catch {
      setPushMessage('关闭推送失败')
    }
    setPushBusy(false)
    setPushSub(await activePushSubscription())
  }

  if (isLoading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">加载失败：{error.message}</p>

  const items = data?.items ?? []

  return (
    <div>
      <h1 className="mb-4 text-xl font-semibold text-slate-900">通知</h1>
      <PushSetup
        supported={supported}
        enabled={pushSub !== null}
        busy={pushBusy}
        message={pushMessage}
        onEnable={() => void enable()}
        onDisable={() => void disable()}
      />
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
      {data && (
        <Pager
          offset={offset}
          pageSize={PAGE_SIZE}
          total={data.meta.total}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  )
}
