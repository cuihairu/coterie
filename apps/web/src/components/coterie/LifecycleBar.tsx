import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../lib/api'
import type { Coterie } from '../../lib/api'

// Mirrors the server lifecycle (internal/coterie/service.go transitions).
const NEXT_STATUS: Record<string, string[]> = {
  draft: ['open', 'closed'],
  open: ['active', 'closed'],
  active: ['paused', 'closed'],
  paused: ['active', 'closed'],
  closed: [],
}

const STATUS_LABEL: Record<string, string> = {
  open: '发布',
  active: '激活',
  paused: '暂停',
  closed: '关闭',
}

export function LifecycleBar({ coterie }: { coterie: Coterie }) {
  const qc = useQueryClient()
  const [error, setError] = useState('')

  const transition = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      api<Coterie>(`/coteries/${coterie.id}`, { method: 'PATCH', body: JSON.stringify(body) }),
    onSuccess: () => {
      setError('')
      void qc.invalidateQueries({ queryKey: ['coterie', coterie.id] })
    },
    onError: (err) => setError(err.message),
  })

  const nexts = NEXT_STATUS[coterie.status] ?? []

  return (
    <div className="mb-4 flex items-center gap-2">
      {nexts.map((s) => (
        <button
          key={s}
          onClick={() => transition.mutate({ status: s })}
          disabled={transition.isPending}
          className={`rounded-lg px-3 py-1.5 text-sm font-medium ${
            s === 'closed'
              ? 'bg-slate-200 text-slate-700 hover:bg-slate-300'
              : 'bg-emerald-700 text-white hover:bg-emerald-800'
          } disabled:opacity-50`}
        >
          {STATUS_LABEL[s] ?? s}
        </button>
      ))}
      <button
        onClick={() => transition.mutate({ payment_gate: !coterie.payment_gate })}
        disabled={transition.isPending}
        className="rounded-lg bg-slate-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
        title="开启后同意加入需先收准入费（D25）"
      >
        {coterie.payment_gate ? '关闭支付闸门' : '开启支付闸门'}
      </button>
      <button
        onClick={() =>
          transition.mutate({ listing: coterie.listing === 'public' ? 'private' : 'public' })
        }
        disabled={transition.isPending}
        className="rounded-lg bg-slate-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
        title="公开后出现在市场目录，任何人可申请加入（D10）"
      >
        {coterie.listing === 'public' ? '下架（转私有）' : '上架公开目录'}
      </button>
      {error && <span className="text-sm text-red-600">{error}</span>}
    </div>
  )
}
