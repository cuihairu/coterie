import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { chargeMethodFor, PaymentMethodSelect } from '../PaymentMethodSelect'
import type { BlockEntry, JoinRequest, ListResponse } from '../../lib/api'

export function RequestsTab({
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
  const [payMethod, setPayMethod] = useState('')
  const [blocking, setBlocking] = useState<string | null>(null)

  // The charge can go through any configured channel, not just manual —
  // the list is the server's registered adapters (manual is always on).
  const methodsQuery = useQuery({
    queryKey: ['payment-methods'],
    queryFn: () => api<ListResponse<string>>('/payments/methods'),
  })
  const methods = methodsQuery.data?.items ?? []
  const chargeMethod = chargeMethodFor(methods, payMethod)

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
              <p className="mt-1 text-xs text-slate-600">
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
        <p className="text-sm text-slate-600">暂无封禁用户。</p>
      ) : (
        blocks.map((b) => (
          <div
            key={b.user_id}
            className="flex items-center justify-between rounded-xl border border-slate-200 bg-white p-3"
          >
            <p className="text-sm text-slate-700">
              {b.username}
              <span className="ml-2 text-xs text-slate-600">{b.email}</span>
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
