import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { api } from '../../lib/api'
import type { Report } from '../../lib/api'

export function ReportBar({ coterieId }: { coterieId: string }) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')

  const report = useMutation({
    mutationFn: () =>
      api<Report>(`/coteries/${coterieId}/report`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    onSuccess: () => {
      setDone(true)
      setOpen(false)
    },
    onError: (err) => setError(err.message),
  })

  if (done) {
    return <p className="mb-4 rounded-xl border border-slate-200 bg-white p-3 text-xs text-slate-500">已提交举报，平台管理员会处置。</p>
  }
  return (
    <div className="mb-4 rounded-xl border border-slate-200 bg-white p-3">
      {open ? (
        <span className="inline-flex items-center gap-1">
          <input
            className="w-64 rounded border border-slate-300 px-2 py-1 text-xs"
            placeholder="举报原因（必填）"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <button
            onClick={() => report.mutate()}
            disabled={report.isPending || !reason.trim()}
            className="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-rose-700 disabled:opacity-50"
          >
            {report.isPending ? '提交中…' : '提交举报'}
          </button>
          <button
            onClick={() => setOpen(false)}
            className="text-xs text-slate-500 hover:underline"
          >
            取消
          </button>
          {error && <span className="text-xs text-red-600">{error}</span>}
        </span>
      ) : (
        <button
          onClick={() => setOpen(true)}
          className="text-xs text-slate-500 hover:underline"
        >
          举报此圈
        </button>
      )}
    </div>
  )
}
