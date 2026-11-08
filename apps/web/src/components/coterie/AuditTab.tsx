import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, AUDIT_ACTIONS, AUDIT_LABEL } from '../../lib/api'
import type { AuditLog, ListResponse } from '../../lib/api'

const AUDIT_PAGE = 20

export function AuditTab({ coterieId }: { coterieId: string }) {
  const [action, setAction] = useState('')
  const [offset, setOffset] = useState(0)

  const query = useQuery({
    queryKey: ['audit', coterieId, action, offset],
    queryFn: () => {
      const p = new URLSearchParams({ limit: String(AUDIT_PAGE), offset: String(offset) })
      if (action) p.set('action', action)
      return api<ListResponse<AuditLog>>(`/coteries/${coterieId}/audit-logs?${p}`)
    },
  })

  if (query.isLoading) return <p className="text-slate-500">加载中…</p>
  if (query.error) return <p className="text-red-600">加载失败：{query.error.message}</p>

  const items = query.data?.items ?? []
  const meta = query.data?.meta

  return (
    <div>
      <div className="mb-4 flex items-center gap-2">
        <select
          className="rounded border border-slate-300 px-2 py-1.5 text-sm"
          value={action}
          onChange={(e) => {
            setAction(e.target.value)
            setOffset(0)
          }}
        >
          <option value="">全部动作</option>
          {AUDIT_ACTIONS.map((a) => (
            <option key={a} value={a}>
              {AUDIT_LABEL[a] ?? a}
            </option>
          ))}
        </select>
        {meta && (
          <span className="text-xs text-slate-600">
            共 {meta.total} 条 · 第 {Math.floor(offset / AUDIT_PAGE) + 1} 页
          </span>
        )}
      </div>
      {items.length === 0 ? (
        <p className="text-slate-500">暂无审计记录。</p>
      ) : (
        <div className="space-y-2">
          {items.map((l) => (
            <div key={l.id} className="rounded-xl border border-slate-200 bg-white p-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-sm">
                  <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                    {AUDIT_LABEL[l.action] ?? l.action}
                  </span>
                  <span className="ml-2 text-xs text-slate-500">
                    操作人 {(l.actor_id ?? 'system').slice(0, 8)}…
                    <span className="ml-1 text-slate-600">
                      {l.entity_type} {l.entity_id.slice(0, 8)}…
                    </span>
                  </span>
                </p>
                <span className="shrink-0 text-xs text-slate-600">
                  {new Date(l.created_at).toLocaleString()}
                </span>
              </div>
              {(l.before || l.after) && (
                <p className="mt-1 break-all font-mono text-xs text-slate-500">
                  {l.before && <span>− {JSON.stringify(l.before)} </span>}
                  {l.after && <span>+ {JSON.stringify(l.after)}</span>}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
      {meta && meta.total > AUDIT_PAGE && (
        <div className="mt-4 flex items-center gap-3">
          <button
            onClick={() => setOffset(Math.max(0, offset - AUDIT_PAGE))}
            disabled={offset === 0}
            className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
          >
            上一页
          </button>
          <button
            onClick={() => setOffset(offset + AUDIT_PAGE)}
            disabled={offset + AUDIT_PAGE >= meta.total}
            className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
