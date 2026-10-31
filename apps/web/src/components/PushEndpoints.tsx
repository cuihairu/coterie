// PushEndpoints lists the signed-in user's registered push endpoints
// (other browsers included) with a per-row revoke — the management
// half of the push story; PushSetup handles this browser's toggle.
// Presentational: the page owns the list query and the delete call.
import type { PushSubscriptionInfo } from '../lib/push'

function hostOf(endpoint: string): string {
  try {
    return new URL(endpoint).host
  } catch {
    return endpoint
  }
}

type PushEndpointsProps = {
  devices: PushSubscriptionInfo[]
  thisEndpoint?: string
  busyId?: string | null
  onRevoke: (id: string) => void
  className?: string
}

export function PushEndpoints({
  devices,
  thisEndpoint,
  busyId = null,
  onRevoke,
  className,
}: PushEndpointsProps) {
  if (devices.length === 0) return null
  return (
    <div className={className ?? 'mb-4 rounded-xl border border-slate-200 bg-white p-4'}>
      <h2 className="text-sm font-medium text-slate-900">已登记推送端点</h2>
      <ul className="mt-2 space-y-2">
        {devices.map((d) => (
          <li key={d.id} className="flex items-center justify-between gap-3 text-sm">
            <div className="min-w-0">
              <p className="truncate text-slate-700">
                {hostOf(d.endpoint)}
                {d.endpoint === thisEndpoint && (
                  <span className="ml-2 rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700">
                    此浏览器
                  </span>
                )}
              </p>
              <p className="text-xs text-slate-600">{new Date(d.created_at).toLocaleString()}</p>
            </div>
            <button
              onClick={() => onRevoke(d.id)}
              disabled={busyId === d.id}
              className="shrink-0 rounded-lg border border-slate-300 px-2 py-1 text-xs text-slate-600 hover:bg-slate-50 disabled:opacity-50"
            >
              注销
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
