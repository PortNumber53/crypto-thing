import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CandlestickChart, Loader2, RefreshCw, Search } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { fmtPct, fmtUSD } from '@/lib/utils'
import { fieldClass, PageHeader, Panel, primaryButton } from '@/components/UI'

export default function Market() {
  const [search, setSearch] = useState('')
  const qc = useQueryClient()
  const products = useQuery({ queryKey: ['products', search], queryFn: () => api.products('coinbase', search) })
  const sync = useMutation({ mutationFn: api.syncProducts, onSuccess: () => qc.invalidateQueries({ queryKey: ['products'] }) })
  return <div className="space-y-6">
    <PageHeader eyebrow="Market data" title="Markets" description="Browse Coinbase products, coverage, price changes, and jump directly into historical charts." actions={<button className={primaryButton} disabled={sync.isPending} onClick={() => sync.mutate()}>{sync.isPending ? <Loader2 size={15} className="animate-spin" /> : <RefreshCw size={15} />}Sync products</button>} />
    <div className="relative max-w-xl"><Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" size={15} /><input className={`${fieldClass} pl-9`} value={search} onChange={e => setSearch(e.target.value)} placeholder="Search symbols or product names" /></div>
    {sync.isError && <p className="text-sm text-rose-400">{sync.error.message}</p>}
    <Panel className="overflow-hidden"><div className="overflow-x-auto"><table className="w-full min-w-[760px] text-sm"><thead className="border-b border-slate-800 bg-slate-900"><tr>{['Pair','Price','24h change','Volume','Coverage','Status',''].map((h, i) => <th key={h} className={`px-5 py-3 text-xs font-medium uppercase tracking-wider text-slate-500 ${i ? 'text-right' : 'text-left'}`}>{h}</th>)}</tr></thead><tbody>{products.isLoading ? <tr><td colSpan={7} className="py-16 text-center text-slate-500"><Loader2 className="mx-auto animate-spin" size={18} /></td></tr> : (products.data ?? []).map(p => <tr key={p.product_id} className="border-b border-slate-800/70 last:border-0 hover:bg-slate-800/25"><td className="px-5 py-3"><div className="flex items-center gap-3"><div className="grid h-8 w-8 place-items-center rounded-full bg-slate-800 text-[10px] font-bold text-cyan-300">{p.base_currency_id.slice(0,2)}</div><div><p className="font-medium text-slate-200">{p.display_name}</p><p className="text-xs text-slate-600">{p.product_id}</p></div></div></td><td className="px-5 py-3 text-right tabular-nums">{p.price ? fmtUSD(p.price) : '—'}</td><td className={`px-5 py-3 text-right tabular-nums ${p.price_change_24h >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>{p.price_change_24h ? fmtPct(p.price_change_24h / 100) : '—'}</td><td className="px-5 py-3 text-right text-slate-400">{p.volume_24h ? fmtUSD(p.volume_24h) : '—'}</td><td className="px-5 py-3 text-right text-slate-400">{p.candle_count.toLocaleString()}</td><td className="px-5 py-3 text-right"><span className={p.status === 'online' ? 'rounded-full bg-emerald-500/10 px-2 py-1 text-xs text-emerald-400' : 'text-xs text-slate-500'}>{p.status || 'unknown'}</span></td><td className="px-5 py-3 text-right"><Link aria-label={`Chart ${p.product_id}`} className="inline-flex rounded-lg p-2 text-slate-500 hover:bg-cyan-500/10 hover:text-cyan-400" to={`/candles?product=${p.product_id}`}><CandlestickChart size={15} /></Link></td></tr>)}</tbody></table></div></Panel>
  </div>
}
