import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, Loader2, Pencil, Play } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, type RunStrategyRequest } from '@/lib/api'
import { daysAgo, fmtDate, fmtPct, today } from '@/lib/utils'
import { Field, fieldClass, Loading, PageHeader, Panel, primaryButton, secondaryButton } from '@/components/UI'

export default function StrategyDetail() {
  const id = Number(useParams().id)
  const nav = useNavigate()
  const qc = useQueryClient()
  const [request, setRequest] = useState<RunStrategyRequest>({ exchange: 'coinbase', product_id: 'BTC-USD', granularity: '1h', start: daysAgo(365), end: today(), save: true })
  const strategy = useQuery({ queryKey: ['strategy',id], queryFn: () => api.strategy(id), enabled: Number.isFinite(id) })
  const results = useQuery({ queryKey: ['backtests','strategy',id], queryFn: () => api.backtests('',50,id), enabled: Number.isFinite(id) })
  const products = useQuery({ queryKey: ['products',''], queryFn: () => api.products() })
  const run = useMutation({ mutationFn: () => api.runStrategy(id,request), onSuccess: result => { qc.invalidateQueries({ queryKey: ['backtests'] }); if (result.id) nav(`/backtests/${result.id}`) } })
  if (strategy.isLoading) return <Loading />
  if (!strategy.data) return <Panel className="p-8 text-center text-rose-400">{strategy.error?.message ?? 'Strategy not found'}</Panel>
  const s = strategy.data
  return <div className="space-y-6">
    <PageHeader eyebrow="Saved strategy" title={s.name} description={s.description || 'Reusable backtest configuration'} actions={<><Link className={secondaryButton} to="/strategies"><ArrowLeft size={15} />Strategies</Link><Link className={primaryButton} to={`/strategies/${id}/edit`}><Pencil size={14} />Edit</Link></>} />
    <div className="grid gap-5 xl:grid-cols-[.8fr_1.2fr]">
      <div className="space-y-5"><Panel className="p-5"><h2 className="font-medium text-white">Configuration</h2><dl className="mt-5 grid grid-cols-2 gap-4 text-sm">{[
        ['Signals',s.signals || 'Adaptive'],['Combination',s.combination],['Bull signals',s.bull_signals || '—'],['Bear signals',s.bear_signals || '—'],['Threshold',String(s.threshold)],['Regime EMA',`${s.regime_fast} / ${s.regime_slow}`],['Fees',fmtPct(s.fee_rate)],['Slippage',fmtPct(s.slippage)],['Tax',fmtPct(s.tax_rate)],['Min edge',fmtPct(s.min_edge)],['Minimum R/R',String(s.rr_min)],['Max loss',s.max_loss ? fmtPct(s.max_loss) : 'Disabled'],['Profit gate',s.profit_gate ? 'Enabled' : 'Disabled'],['Sizing',s.position_size ? `$${s.position_size.toLocaleString()} / $${s.capital.toLocaleString()}` : 'Full equity'],
      ].map(([key,value]) => <div key={key}><dt className="text-xs text-slate-600">{key}</dt><dd className="mt-1 break-words text-slate-300">{value}</dd></div>)}</dl></Panel>
      <Panel className="p-5"><h2 className="font-medium text-white">Run this strategy</h2><p className="mt-1 text-xs text-slate-500">Choose the market and evaluation window</p><form onSubmit={e => { e.preventDefault(); run.mutate() }} className="mt-5 space-y-4"><Field label="Product"><select className={fieldClass} value={request.product_id} onChange={e => setRequest(r => ({...r,product_id:e.target.value}))}><option>BTC-USD</option>{products.data?.filter(p => p.product_id !== 'BTC-USD').slice(0,300).map(p => <option key={p.product_id}>{p.product_id}</option>)}</select></Field><div className="grid grid-cols-3 gap-3"><Field label="Interval"><select className={fieldClass} value={request.granularity} onChange={e => setRequest(r => ({...r,granularity:e.target.value}))}>{['1m','5m','15m','30m','1h','2h','6h','1d'].map(g => <option key={g}>{g}</option>)}</select></Field><Field label="Start"><input className={fieldClass} type="date" value={request.start} onChange={e => setRequest(r => ({...r,start:e.target.value}))} /></Field><Field label="End"><input className={fieldClass} type="date" value={request.end} onChange={e => setRequest(r => ({...r,end:e.target.value}))} /></Field></div><label className="flex items-center gap-2 text-sm text-slate-400"><input type="checkbox" className="accent-cyan-500" checked={request.save} onChange={e => setRequest(r => ({...r,save:e.target.checked}))} />Save report</label>{run.isError && <p className="text-sm text-rose-400">{run.error.message}</p>}<button className={`${primaryButton} w-full`} disabled={run.isPending}>{run.isPending ? <Loader2 className="animate-spin" size={15} /> : <Play size={15} />}{run.isPending ? 'Running…' : 'Run strategy'}</button></form></Panel></div>
      <Panel className="overflow-hidden self-start"><div className="border-b border-slate-800 px-5 py-4"><h2 className="font-medium text-white">Strategy results</h2><p className="mt-1 text-xs text-slate-500">Saved runs linked to this configuration</p></div>{results.isLoading ? <Loading /> : !results.data?.length ? <p className="p-12 text-center text-sm text-slate-600">No runs yet. Use the form to establish a baseline.</p> : results.data.map(r => <Link key={r.id} to={`/backtests/${r.id}`} className="grid grid-cols-[1fr_auto_auto] items-center gap-5 border-b border-slate-800/70 px-5 py-4 last:border-0 hover:bg-slate-800/30"><div><p className="font-medium text-slate-200">{r.product_id} · {r.granularity}</p><p className="mt-1 text-xs text-slate-600">{fmtDate(r.start_time)} — {fmtDate(r.end_time)}</p></div><div className="text-right"><p className={r.total_return >= 0 ? 'font-medium text-emerald-400' : 'font-medium text-rose-400'}>{fmtPct(r.total_return)}</p><p className="text-xs text-slate-600">Sharpe {r.sharpe.toFixed(2)}</p></div><ArrowRight size={15} className="text-slate-600" /></Link>)}</Panel>
    </div>
  </div>
}
