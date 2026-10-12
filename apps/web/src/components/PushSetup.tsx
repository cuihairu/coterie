// PushSetup is the push on/off control for the notifications page.
// Presentational by convention (Pager, ComplianceNotice): the page
// owns the browser state and the enable/disable flow (lib/push), and
// unsupported browsers render nothing at all.
type PushSetupProps = {
  supported: boolean
  enabled: boolean
  busy?: boolean
  message?: string
  onEnable: () => void
  onDisable: () => void
  className?: string
}

export function PushSetup({
  supported,
  enabled,
  busy = false,
  message,
  onEnable,
  onDisable,
  className,
}: PushSetupProps) {
  if (!supported) return null
  return (
    <div className={className ?? 'mb-4 rounded-xl border border-slate-200 bg-white p-4'}>
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium text-slate-900">浏览器推送</h2>
          <p className="mt-1 text-xs text-slate-500">
            {enabled ? '此浏览器会收到系统级通知。' : '开启后通知会直接送达此浏览器，无需打开页面。'}
          </p>
        </div>
        {enabled ? (
          <button
            onClick={onDisable}
            disabled={busy}
            className="shrink-0 rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-50"
          >
            关闭推送
          </button>
        ) : (
          <button
            onClick={onEnable}
            disabled={busy}
            className="shrink-0 rounded-lg border border-emerald-300 bg-emerald-50 px-3 py-1.5 text-sm text-emerald-700 hover:bg-emerald-100 disabled:opacity-50"
          >
            开启推送
          </button>
        )}
      </div>
      {message && <p role="alert" className="mt-2 text-xs text-red-600">{message}</p>}
    </div>
  )
}
