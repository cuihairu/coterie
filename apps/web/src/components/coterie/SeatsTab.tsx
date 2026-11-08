import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { api } from '../../lib/api'
import type { ListResponse, Member, Seat } from '../../lib/api'

export function SeatsTab({
  subID,
  query,
  membersQuery,
  isOwner,
}: {
  subID?: string
  query: UseQueryResult<ListResponse<Seat>, Error>
  membersQuery: UseQueryResult<ListResponse<Member>, Error>
  isOwner: boolean
}) {
  const qc = useQueryClient()
  const [assigning, setAssigning] = useState<string | null>(null)
  const [memberID, setMemberID] = useState('')
  const [editing, setEditing] = useState<string | null>(null)
  const [editLabel, setEditLabel] = useState('')
  // 席位计量类型（D4/D23）：quota 配额 / resource 资源，与 used 投影
  // 共用同一条账本归集管线；普通席位不参与计量。
  const [editKind, setEditKind] = useState<'none' | 'quota' | 'resource'>('none')
  const [editCapacity, setEditCapacity] = useState('')
  const [editUnit, setEditUnit] = useState('')
  const [releasing, setReleasing] = useState<string | null>(null)
  const [error, setError] = useState('')
  const editValid =
    !!editLabel.trim() &&
    (editKind === 'none' || (Number(editCapacity) > 0 && !!editUnit.trim()))

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['seats', subID] })
    void qc.invalidateQueries({ queryKey: ['coterie'] })
  }

  const provision = useMutation({
    mutationFn: () =>
      api<Seat[]>(`/subscriptions/${subID}/seats`, {
        method: 'POST',
        body: JSON.stringify({ count: 1 }),
      }),
    onSuccess: () => {
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const assign = useMutation({
    mutationFn: (seatID: string) =>
      api<Seat>(`/seats/${seatID}/assign`, {
        method: 'POST',
        body: JSON.stringify({ member_id: memberID }),
      }),
    onSuccess: () => {
      setError('')
      setAssigning(null)
      setMemberID('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  // updateSeat is the PATCH /seats/{id} surface: relabel, or take a
  // seat out of circulation (free⇄disabled; occupied seats go through
  // assign/release instead). Both land as seat_updated audit entries.
  const updateSeat = useMutation({
    mutationFn: ({ seatID, body }: { seatID: string; body: Record<string, unknown> }) =>
      api<Seat>(`/seats/${seatID}`, { method: 'PATCH', body: JSON.stringify(body) }),
    onSuccess: () => {
      setError('')
      setEditing(null)
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  // releaseSeat frees an occupied seat — the only way back from
  // occupied besides the member leaving; disabling one is a 409.
  const releaseSeat = useMutation({
    mutationFn: (seatID: string) => api<Seat>(`/seats/${seatID}/release`, { method: 'POST' }),
    onSuccess: () => {
      setError('')
      setReleasing(null)
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  // saveSeatEdit builds the PATCH body: the label always travels; the
  // metadata only travels when the metering actually changed, so a
  // plain relabel never rewrites the projected `used`. Switching the
  // kind clears the old key — quota and resource are the same ledger
  // pipeline (D23), only the metadata name differs.
  const saveSeatEdit = (s: Seat) => {
    const body: Record<string, unknown> = { label: editLabel.trim() }
    const md = s.metadata ?? {}
    const prevKind: 'none' | 'quota' | 'resource' =
      md.quota !== undefined ? 'quota' : md.resource !== undefined ? 'resource' : 'none'
    if (editKind === 'none') {
      if (prevKind !== 'none') body.metadata = {}
      return updateSeat.mutate({ seatID: s.id, body })
    }
    const capacity = Number(editCapacity)
    const unit = editUnit.trim()
    if (!(capacity > 0) || !unit) return
    if (prevKind !== editKind || String(md[editKind]) !== String(capacity) || String(md.unit ?? '') !== unit) {
      body.metadata = {
        [editKind]: capacity,
        unit,
        used: prevKind === editKind && md.used !== undefined ? md.used : 0,
      }
    }
    updateSeat.mutate({ seatID: s.id, body })
  }

  if (query.isLoading || membersQuery.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>
  const items = query.data?.items ?? []
  const members = membersQuery.data?.items ?? []

  return (
    <div>
      {isOwner && (
        <div className="mb-4 flex items-center gap-2">
          <button
            onClick={() => provision.mutate()}
            disabled={provision.isPending || !subID}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {provision.isPending ? '增补中…' : '增补席位'}
          </button>
          {error && <span className="text-sm text-red-600">{error}</span>}
        </div>
      )}
      {items.length === 0 ? (
        <p className="text-slate-500">暂无席位。</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-slate-500">
              <th className="py-2 font-medium">席位</th>
              <th className="py-2 font-medium">状态</th>
              <th className="py-2 font-medium">成员</th>
              {isOwner && <th className="py-2 font-medium" />}
            </tr>
          </thead>
          <tbody>
            {items.map((s) => {
            const md = s.metadata ?? {}
            const kind: 'none' | 'quota' | 'resource' =
              md.quota !== undefined ? 'quota' : md.resource !== undefined ? 'resource' : 'none'
            return (
              <tr key={s.id} className="border-b border-slate-100">
                <td className="py-2">
                  {editing === s.id ? (
                    <div className="space-y-1">
                      <input
                        className="w-32 rounded border border-slate-300 px-1 py-0.5 text-xs"
                        value={editLabel}
                        onChange={(e) => setEditLabel(e.target.value)}
                      />
                      <div className="flex items-center gap-1">
                        <select
                          className="rounded border border-slate-300 px-1 py-0.5 text-xs"
                          value={editKind}
                          onChange={(e) =>
                            setEditKind(e.target.value as 'none' | 'quota' | 'resource')
                          }
                          title="席位计量类型（D4/D23）"
                        >
                          <option value="none">普通席位</option>
                          <option value="quota">配额席位</option>
                          <option value="resource">资源席位</option>
                        </select>
                        {editKind !== 'none' && (
                          <>
                            <input
                              className="w-14 rounded border border-slate-300 px-1 py-0.5 text-xs"
                              placeholder="容量"
                              value={editCapacity}
                              onChange={(e) => setEditCapacity(e.target.value)}
                            />
                            <input
                              className="w-14 rounded border border-slate-300 px-1 py-0.5 text-xs"
                              placeholder="单位"
                              value={editUnit}
                              onChange={(e) => setEditUnit(e.target.value)}
                            />
                          </>
                        )}
                      </div>
                    </div>
                  ) : (
                    <span>
                      {s.label}
                      {kind !== 'none' && (
                        <span className="ml-2 rounded-full bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">
                          {kind === 'quota' ? '配额' : '资源'} {String(md[kind])} {String(md.unit ?? '')}
                          {md.used !== undefined ? ` · 已用 ${String(md.used)}` : ''}
                        </span>
                      )}
                    </span>
                  )}
                </td>
                <td className="py-2">{s.status}</td>
                <td className="py-2 font-mono text-xs text-slate-700">
                  {s.member_id ? `${s.member_id.slice(0, 8)}…` : '—'}
                </td>
                {isOwner && (
                  <td className="py-2 text-right">
                    {s.status === 'free' && assigning !== s.id && (
                      <button
                        onClick={() => {
                          setAssigning(s.id)
                          setMemberID('')
                        }}
                        className="text-xs text-emerald-700 hover:underline"
                      >
                        分配
                      </button>
                    )}
                    {assigning !== s.id && editing === s.id && (
                      <span className="inline-flex items-center gap-1">
                        <button
                          onClick={() => saveSeatEdit(s)}
                          disabled={updateSeat.isPending || !editValid}
                          className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                        >
                          保存
                        </button>
                        <button
                          onClick={() => setEditing(null)}
                          className="text-xs text-slate-500 hover:underline"
                        >
                          取消
                        </button>
                      </span>
                    )}
                    {assigning !== s.id && editing !== s.id && (
                      <>
                        <button
                          onClick={() => {
                            setEditing(s.id)
                            setEditLabel(s.label)
                            const cur = s.metadata ?? {}
                            const curKind: 'none' | 'quota' | 'resource' =
                              cur.quota !== undefined
                                ? 'quota'
                                : cur.resource !== undefined
                                  ? 'resource'
                                  : 'none'
                            setEditKind(curKind)
                            setEditCapacity(curKind === 'none' ? '' : String(cur[curKind]))
                            setEditUnit(curKind === 'none' ? '' : String(cur.unit ?? ''))
                          }}
                          className="ml-2 text-xs text-slate-600 hover:underline"
                        >
                          编辑
                        </button>
                        {s.status !== 'occupied' &&
                          (s.status === 'free' ? (
                            <button
                              onClick={() =>
                                updateSeat.mutate({ seatID: s.id, body: { status: 'disabled' } })
                              }
                              disabled={updateSeat.isPending}
                              className="ml-2 text-xs text-slate-500 hover:underline disabled:opacity-50"
                            >
                              停用
                            </button>
                          ) : (
                            <button
                              onClick={() =>
                                updateSeat.mutate({ seatID: s.id, body: { status: 'free' } })
                              }
                              disabled={updateSeat.isPending}
                              className="ml-2 text-xs text-slate-500 hover:underline disabled:opacity-50"
                            >
                              启用
                            </button>
                          ))}
                        {s.status === 'occupied' &&
                          (releasing === s.id ? (
                            <span className="inline-flex items-center gap-1">
                              <button
                                onClick={() => releaseSeat.mutate(s.id)}
                                disabled={releaseSeat.isPending}
                                className="text-xs text-red-600 hover:underline disabled:opacity-50"
                              >
                                确认释放
                              </button>
                              <button
                                onClick={() => setReleasing(null)}
                                className="text-xs text-slate-500 hover:underline"
                              >
                                取消
                              </button>
                            </span>
                          ) : (
                            <button
                              onClick={() => setReleasing(s.id)}
                              className="ml-2 text-xs text-slate-500 hover:underline"
                            >
                              释放
                            </button>
                          ))}
                      </>
                    )}
                    {assigning === s.id && (
                      <span className="inline-flex items-center gap-1">
                        <select
                          className="rounded border border-slate-300 px-1 py-0.5 text-xs"
                          value={memberID}
                          onChange={(e) => setMemberID(e.target.value)}
                        >
                          <option value="">选择成员…</option>
                          {members.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.user_id.slice(0, 8)}…（{m.role}）
                            </option>
                          ))}
                        </select>
                        <button
                          onClick={() => assign.mutate(s.id)}
                          disabled={!memberID || assign.isPending}
                          className="text-xs text-emerald-700 hover:underline disabled:opacity-50"
                        >
                          确认
                        </button>
                        <button
                          onClick={() => setAssigning(null)}
                          className="text-xs text-slate-500 hover:underline"
                        >
                          取消
                        </button>
                      </span>
                    )}
                  </td>
                )}
              </tr>
            )
            })}
          </tbody>
        </table>
      )}
    </div>
  )
}
