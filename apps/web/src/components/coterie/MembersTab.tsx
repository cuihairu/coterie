import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { api, getUser } from '../../lib/api'
import type { Invitation, ListResponse, Member } from '../../lib/api'

export function MembersTab({
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
          <p className="mt-1 text-xs text-slate-600">邀请令牌只显示一次，请立即复制发给对方。</p>
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
                          <span className="ml-1 text-slate-600">
                            （{new Date(inv.expire_at).toLocaleString()} 前有效）
                          </span>
                        )}
                      </span>
                      <span
                        className={
                          state === 'accepted'
                            ? 'text-emerald-700'
                            : state === 'expired'
                              ? 'text-slate-600'
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
              className="rounded-lg border border-rose-300 px-3 py-1.5 text-sm text-rose-800 hover:bg-rose-50"
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
                    <span className="ml-1 text-slate-600">（我）</span>
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
      {myMember && <p className="mt-2 text-xs text-slate-600">成员 {items.length} 人。</p>}
    </div>
  )
}
