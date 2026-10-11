// The admission-charge channel picker. It only renders once the server
// registers a second channel — with a single channel there is no choice
// to make and the charge falls back to that channel (manual is always on).
type PaymentMethodSelectProps = {
  methods: string[]
  value: string
  onChange: (method: string) => void
}

export function PaymentMethodSelect({ methods, value, onChange }: PaymentMethodSelectProps) {
  if (methods.length <= 1) return null
  return (
    <select
      className="rounded border border-slate-300 px-1 py-0.5 text-xs"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {methods.map((m) => (
        <option key={m} value={m}>
          {m}
        </option>
      ))}
    </select>
  )
}

// The effective channel for a charge: the user's pick, else the first
// registered channel, else the always-on manual adapter.
export function chargeMethodFor(methods: string[], picked?: string): string {
  return picked || methods[0] || 'manual'
}
