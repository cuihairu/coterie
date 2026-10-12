// The NFR-2 prompt: the platform never asserts a subscription may be
// shared — the person sharing or joining confirms the provider's terms
// of service, region limits, and sharing rights themselves. Rendered on
// the surfaces where that commitment happens (create-circle, the
// marketplace directory).
export function ComplianceNotice({ className }: { className?: string }) {
  return (
    <p
      role="note"
      className={
        className ??
        'mb-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800'
      }
    >
      请自行确认相关 Provider 的服务条款、地区限制与共享权限，平台不代为判断账号是否可共享。
    </p>
  )
}
