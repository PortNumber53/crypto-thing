import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type Product } from '@/lib/api'
import { fmtUSD, fmtPct } from '@/lib/utils'
import { Search, RefreshCw, CandlestickChart, Loader2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

function PriceChange({ v }: { v: number }) {
  const positive = v >= 0
  return (
    <span className={positive ? 'text-emerald-400' : 'text-rose-400'}>
      {fmtPct(v / 100)}
    </span>
  )
}

function ProductRow({ p, onViewCandles }: { p: Product; onViewCandles: (id: string) => void }) {
  return (
    <tr className="border-b border-slate-800 hover:bg-slate-800/40 transition-colors">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-slate-700 flex items-center justify-center text-xs font-bold text-slate-300">
            {p.base_currency_id.slice(0, 2)}
          </div>
          <div>
            <p className="text-sm font-medium text-slate-200">{p.display_name}</p>
            <p className="text-xs text-slate-500">{p.product_id}</p>
          </div>
        </div>
      </td>
      <td className="px-4 py-3 text-right text-sm text-slate-200">
        {p.price > 0 ? fmtUSD(p.price) : '—'}
      </td>
      <td className="px-4 py-3 text-right text-sm">
        {p.price_change_24h !== 0 ? <PriceChange v={p.price_change_24h} /> : <span className="text-slate-500">—</span>}
      </td>
      <td className="px-4 py-3 text-right text-sm text-slate-400">
        {p.volume_24h > 0 ? fmtUSD(p.volume_24h) : '—'}
      </td>
      <td className="px-4 py-3 text-right text-sm text-slate-400">
        {p.candle_count.toLocaleString()}
      </td>
      <td className="px-4 py-3 text-right">
        <span className={`inline-flex px-2 py-0.5 rounded text-xs font-medium ${
          p.status === 'online' ? 'bg-emerald-500/10 text-emerald-400' : 'bg-slate-700 text-slate-400'
        }`}>{p.status || '—'}</span>
      </td>
      <td className="px-4 py-3 text-right">
        <button
          onClick={() => onViewCandles(p.product_id)}
          className="p-1.5 text-slate-500 hover:text-blue-400 hover:bg-blue-500/10 rounded transition-colors"
          title="View candles"
        >
          <CandlestickChart size={14} />
        </button>
      </td>
    </tr>
  )
}

export default function Products() {
  const [q, setQ] = useState('')
  const navigate = useNavigate()
  const qc = useQueryClient()

  const { data: products = [], isLoading } = useQuery({
    queryKey: ['products', q],
    queryFn: () => api.products('coinbase', q),
  })

  const sync = useMutation({
    mutationFn: api.syncProducts,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['products'] }),
  })

  const filtered = q
    ? products.filter(p =>
        p.product_id.toLowerCase().includes(q.toLowerCase()) ||
        p.display_name.toLowerCase().includes(q.toLowerCase()),
      )
    : products

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Products</h1>
          <p className="text-sm text-slate-400 mt-0.5">
            {products.length.toLocaleString()} pairs from Coinbase
          </p>
        </div>
        <button
          onClick={() => sync.mutate()}
          disabled={sync.isPending}
          className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-400 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
        >
          {sync.isPending ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
          {sync.isPending ? 'Syncing…' : 'Sync Products'}
        </button>
      </div>

      {sync.isSuccess && (
        <div className="px-4 py-2 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-sm">
          Synced {sync.data.synced} products ({sync.data.total} total from API)
        </div>
      )}
      {sync.isError && (
        <div className="px-4 py-2 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm">
          Sync failed: {sync.error?.message}
        </div>
      )}

      {/* Search */}
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
        <input
          value={q}
          onChange={e => setQ(e.target.value)}
          placeholder="Search by symbol or name…"
          className="w-full pl-9 pr-4 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
        />
      </div>

      {/* Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="border-b border-slate-800">
              {['Pair', 'Price', '24h %', '24h Volume', 'Candles', 'Status', ''].map(h => (
                <th key={h} className={`px-4 py-3 text-xs font-medium text-slate-500 ${h === 'Pair' ? 'text-left' : 'text-right'}`}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={7} className="px-4 py-12 text-center text-slate-500 text-sm">
                <Loader2 size={20} className="animate-spin mx-auto mb-2" />Loading products…
              </td></tr>
            ) : filtered.length === 0 ? (
              <tr><td colSpan={7} className="px-4 py-12 text-center text-slate-500 text-sm">
                No products found{q ? ` for "${q}"` : ''}
              </td></tr>
            ) : (
              filtered.map(p => (
                <ProductRow
                  key={p.product_id}
                  p={p}
                  onViewCandles={id => navigate(`/candles?product=${id}`)}
                />
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
