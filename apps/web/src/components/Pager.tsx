// Shared offset pager for the paged list pages (marketplace directory,
// notifications, admin reports). The pages hold an offset state and query
// with limit/offset, so the pager speaks the same unit: it renders nothing
// while one page covers the whole list, and clamps the previous step at 0.
type PagerProps = {
  offset: number
  pageSize: number
  total: number
  onOffsetChange: (offset: number) => void
  className?: string
}

export function Pager({ offset, pageSize, total, onOffsetChange, className }: PagerProps) {
  if (total <= pageSize) return null
  return (
    <div className={className ?? 'mt-4 flex items-center gap-3'}>
      <button
        onClick={() => onOffsetChange(Math.max(0, offset - pageSize))}
        disabled={offset === 0}
        className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
      >
        上一页
      </button>
      <button
        onClick={() => onOffsetChange(offset + pageSize)}
        disabled={offset + pageSize >= total}
        className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600 hover:bg-slate-50 disabled:opacity-40"
      >
        下一页
      </button>
    </div>
  )
}
