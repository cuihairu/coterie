import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { api } from '../../lib/api'
import type { Coterie, ListResponse, Member } from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/')({
  component: CoteriesPage,
})

function CoteriesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['coteries'],
    queryFn: () => api<ListResponse<Coterie>>('/coteries'),
  })

  if (isLoading) return <p className="text-slate-500">加载中…</p>
  if (error) return <p className="text-red-600">加载失败：{error.message}</p>

  const items = data?.items ?? []

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <JoinByToken />
        <Link
          to="/coteries/new"
          className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800"
        >
          创建共享圈
        </Link>
      </div>
      {items.length === 0 ? (
        <p className="text-slate-500">还没有加入任何共享圈。</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {items.map((c) => (
            <Link
              key={c.id}
              to="/coteries/$coterieId"
              params={{ coterieId: c.id }}
              className="block rounded-xl border border-slate-200 bg-white p-5 shadow-sm hover:border-emerald-600"
            >
              <div className="mb-1 flex items-center justify-between">
                <h2 className="font-medium text-slate-900">{c.name}</h2>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs ${
                    c.status === 'open'
                      ? 'bg-emerald-50 text-emerald-700'
                      : 'bg-slate-100 text-slate-600'
                  }`}
                >
                  {c.status}
                </span>
              </div>
              <p className="text-sm text-slate-500">
                成员 {c.member_count} · 席位 {c.seats_free}/{c.seats_total} 空闲
                {c.full ? ' · 已满' : ''}
              </p>
              <p className="mt-1 text-xs text-slate-400">
                {c.listing === 'public' ? '公开目录' : '私有'} · 创建于{' '}
                {new Date(c.created_at).toLocaleDateString()}
              </p>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

// JoinByToken redeems an invitation token — the receiving end of the
// Owner's 邀请成员 action on the circle detail page.
function JoinByToken() {
  const router = useRouter()
  const qc = useQueryClient()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')

  const join = useMutation({
    mutationFn: () =>
      api<Member>('/invitations/accept', {
        method: 'POST',
        body: JSON.stringify({ token: token.trim() }),
      }),
    onSuccess: (m) => {
      setToken('')
      setError('')
      void qc.invalidateQueries({ queryKey: ['coteries'] })
      router.navigate({ to: '/coteries/$coterieId', params: { coterieId: m.coterie_id } })
    },
    onError: (err) => setError(err.message),
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        if (token.trim()) join.mutate()
      }}
      className="flex items-center gap-2"
    >
      <input
        className="w-56 rounded-lg border border-slate-300 px-3 py-1.5 text-sm"
        placeholder="粘贴邀请令牌加入圈"
        value={token}
        onChange={(e) => setToken(e.target.value)}
      />
      <button
        type="submit"
        disabled={join.isPending || !token.trim()}
        className="rounded-lg border border-emerald-700 px-3 py-1.5 text-sm font-medium text-emerald-800 hover:bg-emerald-50 disabled:opacity-50"
      >
        {join.isPending ? '加入中…' : '加入'}
      </button>
      {error && <span className="text-sm text-red-600">{error}</span>}
    </form>
  )
}
