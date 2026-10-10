import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouter } from '@tanstack/react-router'
import { api, ApiError, getUser } from '../../lib/api'
import type { Coterie, ListResponse, Product, Provider, Subscription } from '../../lib/api'

export const Route = createFileRoute('/_app/coteries/new')({
  component: NewCoteriePage,
})

// The create walk mirrors the API journey: provider → product →
// subscription → coterie. Existing catalog entries can be reused, or
// new ones created inline, so a first-time owner needs no detour.
function NewCoteriePage() {
  const router = useRouter()

  const [providerMode, setProviderMode] = useState<'existing' | 'new'>('existing')
  const [providerID, setProviderID] = useState('')
  const [slug, setSlug] = useState('')
  const [providerName, setProviderName] = useState('')
  const [category, setCategory] = useState('video')

  const [productMode, setProductMode] = useState<'existing' | 'new'>('existing')
  const [productID, setProductID] = useState('')
  const [productName, setProductName] = useState('')

  const [billingCycle, setBillingCycle] = useState('monthly')
  const [cycleDays, setCycleDays] = useState('30')
  const [price, setPrice] = useState('')
  const [currency, setCurrency] = useState('USD')
  const [startDate, setStartDate] = useState(() => new Date().toISOString().slice(0, 10))
  const [maxSeats, setMaxSeats] = useState('2')

  const [coterieName, setCoterieName] = useState('')
  const [capacity, setCapacity] = useState('2')
  const [error, setError] = useState('')

  const providersQuery = useQuery({
    queryKey: ['providers'],
    queryFn: () => api<ListResponse<Provider>>('/providers'),
  })
  const productsQuery = useQuery({
    queryKey: ['products', providerID],
    queryFn: () => api<ListResponse<Product>>(`/products?provider_id=${providerID}`),
    enabled: providerMode === 'existing' && providerID !== '',
  })

  const create = useMutation({
    mutationFn: async () => {
      let pid = providerID
      if (providerMode === 'new') {
        const p = await api<Provider>('/providers', {
          method: 'POST',
          body: JSON.stringify({ slug, name: providerName, category }),
        })
        pid = p.id
      }
      let prdID = productID
      if (productMode === 'new') {
        const prd = await api<Product>('/products', {
          method: 'POST',
          body: JSON.stringify({ provider_id: pid, name: productName }),
        })
        prdID = prd.id
      }
      const sub = await api<Subscription>('/subscriptions', {
        method: 'POST',
        body: JSON.stringify({
          product_id: prdID,
          owner_user_id: getUser()?.id,
          billing_cycle: billingCycle,
          price,
          currency,
          start_date: startDate,
          max_seats: Number(maxSeats),
          ...(billingCycle === 'custom' ? { cycle_days: Number(cycleDays) } : {}),
        }),
      })
      return api<Coterie>('/coteries', {
        method: 'POST',
        body: JSON.stringify({ subscription_id: sub.id, name: coterieName, capacity: Number(capacity) }),
      })
    },
    onSuccess: (c) => {
      router.navigate({ to: '/coteries/$coterieId', params: { coterieId: c.id } })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : '创建失败')
    },
  })

  const input = 'w-full rounded-lg border border-slate-300 px-3 py-2 text-sm'
  const label = 'mb-1 block text-sm font-medium text-slate-700'

  return (
    <div className="max-w-lg">
      <Link to="/coteries" className="text-sm text-slate-500 hover:text-slate-700">
        ← 返回圈列表
      </Link>
      <h1 className="mt-2 mb-6 text-xl font-semibold text-slate-900">创建共享圈</h1>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          setError('')
          create.mutate()
        }}
        className="space-y-6"
      >
        <fieldset className="rounded-xl border border-slate-200 bg-white p-4">
          <legend className="px-1 text-sm font-medium text-slate-700">服务商</legend>
          <div className="mb-3 flex gap-2 text-sm">
            {(['existing', 'new'] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setProviderMode(m)}
                className={`rounded-md px-3 py-1 ${
                  providerMode === m ? 'bg-emerald-50 font-medium text-emerald-800' : 'text-slate-500'
                }`}
              >
                {m === 'existing' ? '选择已有' : '新建'}
              </button>
            ))}
          </div>
          {providerMode === 'existing' ? (
            <select
              className={input}
              value={providerID}
              onChange={(e) => {
                setProviderID(e.target.value)
                setProductID('')
              }}
              required
            >
              <option value="">选择服务商…</option>
              {(providersQuery.data?.items ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}（{p.category}）
                </option>
              ))}
            </select>
          ) : (
            <div className="space-y-2">
              <input className={input} placeholder="标识 slug（如 netflix）" value={slug} onChange={(e) => setSlug(e.target.value)} required />
              <input className={input} placeholder="名称" value={providerName} onChange={(e) => setProviderName(e.target.value)} required />
              <input className={input} placeholder="分类（如 video）" value={category} onChange={(e) => setCategory(e.target.value)} required />
            </div>
          )}
        </fieldset>

        <fieldset className="rounded-xl border border-slate-200 bg-white p-4">
          <legend className="px-1 text-sm font-medium text-slate-700">产品</legend>
          <div className="mb-3 flex gap-2 text-sm">
            {(['existing', 'new'] as const).map((m) => (
              <button
                key={m}
                type="button"
                onClick={() => setProductMode(m)}
                className={`rounded-md px-3 py-1 ${
                  productMode === m ? 'bg-emerald-50 font-medium text-emerald-800' : 'text-slate-500'
                }`}
              >
                {m === 'existing' ? '选择已有' : '新建'}
              </button>
            ))}
          </div>
          {productMode === 'existing' ? (
            <select className={input} value={productID} onChange={(e) => setProductID(e.target.value)} required>
              <option value="">选择产品…</option>
              {(productsQuery.data?.items ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          ) : (
            <input className={input} placeholder="产品名称" value={productName} onChange={(e) => setProductName(e.target.value)} required />
          )}
        </fieldset>

        <fieldset className="rounded-xl border border-slate-200 bg-white p-4">
          <legend className="px-1 text-sm font-medium text-slate-700">订阅</legend>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <span className={label}>计费周期</span>
              <select className={input} value={billingCycle} onChange={(e) => setBillingCycle(e.target.value)}>
                <option value="monthly">按月</option>
                <option value="yearly">按年</option>
                <option value="custom">自定义天数</option>
              </select>
            </div>
            {billingCycle === 'custom' && (
              <div>
                <span className={label}>周期天数</span>
                <input className={input} type="number" min={1} value={cycleDays} onChange={(e) => setCycleDays(e.target.value)} required />
              </div>
            )}
            <div>
              <span className={label}>价格</span>
              <input className={input} placeholder="如 15.00" value={price} onChange={(e) => setPrice(e.target.value)} required />
            </div>
            <div>
              <span className={label}>币种</span>
              <input className={input} value={currency} onChange={(e) => setCurrency(e.target.value.toUpperCase())} required />
            </div>
            <div>
              <span className={label}>开始日期</span>
              <input className={input} type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} required />
            </div>
            <div>
              <span className={label}>席位数</span>
              <input className={input} type="number" min={1} value={maxSeats} onChange={(e) => setMaxSeats(e.target.value)} required />
            </div>
          </div>
        </fieldset>

        <fieldset className="rounded-xl border border-slate-200 bg-white p-4">
          <legend className="px-1 text-sm font-medium text-slate-700">圈</legend>
          <div className="grid grid-cols-2 gap-2">
            <div className="col-span-2">
              <span className={label}>圈名称</span>
              <input className={input} placeholder="如 家庭共享圈" value={coterieName} onChange={(e) => setCoterieName(e.target.value)} required />
            </div>
            <div>
              <span className={label}>成员上限</span>
              <input className={input} type="number" min={1} value={capacity} onChange={(e) => setCapacity(e.target.value)} required />
            </div>
          </div>
        </fieldset>

        {error && <p className="text-sm text-red-600">{error}</p>}
        <button
          type="submit"
          disabled={create.isPending}
          className="w-full rounded-lg bg-emerald-700 py-2 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
        >
          {create.isPending ? '创建中…' : '创建圈'}
        </button>
      </form>
    </div>
  )
}
