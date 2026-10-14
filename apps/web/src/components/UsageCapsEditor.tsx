import { useState } from 'react'
import type { UsageLimit } from '../lib/api'

// One displayable and editable per-period usage cap: the D21
// per-member cap (usage_limit) and the FR-10 subscription-wide cap
// (quota_limit) share the shape and the enforcement semantics.
export interface UsageCapSpec {
  kind: 'usage' | 'quota'
  label: string
  hint: string
  cap?: UsageLimit
}

interface UsageCapsEditorProps {
  caps: UsageCapSpec[]
  isOwner: boolean
  onSave: (kind: 'usage' | 'quota', limit: UsageLimit | null) => Promise<void>
}

// UsageCapsEditor lists the subscription's per-period usage caps and,
// for the owner, one inline form to set, change, or clear whichever
// row it was opened from. Saving writes one cap without touching the
// others; API failures surface inline.
export function UsageCapsEditor({ caps, isOwner, onSave }: UsageCapsEditorProps) {
  const [editing, setEditing] = useState<'none' | 'usage' | 'quota'>('none')
  const [unit, setUnit] = useState('')
  const [per, setPer] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const save = async (kind: 'usage' | 'quota', limit: UsageLimit | null) => {
    setSaving(true)
    try {
      await onSave(kind, limit)
      setError('')
      setEditing('none')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      {caps.map(({ kind, label, hint, cap }) => (
        <div key={kind} className="flex items-center justify-between">
          <p className="text-sm text-slate-700">
            {cap ? (
              <>
                {label} <span className="font-medium">{cap.per_period}</span> {cap.unit}
                <span className="ml-1 text-xs text-slate-400">（{hint}）</span>
              </>
            ) : (
              <span className="text-slate-500">未设{label}。</span>
            )}
          </p>
          {isOwner && editing !== kind && (
            <button
              onClick={() => {
                setUnit(cap?.unit ?? '')
                setPer(cap?.per_period ?? '')
                setEditing(kind)
              }}
              className="text-sm text-emerald-700 hover:underline"
            >
              {cap ? '修改' : '设置上限'}
            </button>
          )}
        </div>
      ))}
      {isOwner && editing !== 'none' && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <input
            className="w-28 rounded border border-slate-300 px-2 py-1 text-sm"
            placeholder="单位，如 credits"
            value={unit}
            onChange={(e) => setUnit(e.target.value)}
          />
          <input
            className="w-32 rounded border border-slate-300 px-2 py-1 text-sm"
            placeholder="每期上限"
            value={per}
            onChange={(e) => setPer(e.target.value)}
          />
          <button
            onClick={() => {
              if (!unit.trim() || !per.trim()) {
                setError('单位与上限均必填')
                return
              }
              void save(editing, { unit: unit.trim(), per_period: per.trim() })
            }}
            disabled={saving}
            className="rounded-lg bg-emerald-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            保存
          </button>
          <button
            onClick={() => void save(editing, null)}
            disabled={saving}
            className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50"
          >
            清除上限
          </button>
          <button onClick={() => setEditing('none')} className="text-sm text-slate-500 hover:underline">
            取消
          </button>
        </div>
      )}
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
    </div>
  )
}
