import { useQuery } from '@tanstack/react-query'
import { AlertCircle, ArrowRight, BarChart3, BookOpen, CandlestickChart, FlaskConical, RefreshCw } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { fmtDate, fmtPct } from '@/lib/utils'
import { Metric, PageHeader, Panel, primaryButton, secondaryButton } from '@/components/UI'

export default function Dashboard() {
  const { data, isLoading, isError, refetch } = useQuery({ queryKey: ['stats'], queryFn: api.stats })
  const recent = data?.recent_backtests ?? []
  const best = data?.best_backtest
  return <div className="space-y-7">
    <PageHeader eyebrow="Research workspace" title="Good decisions start with honest tests." description="Track your data coverage, validate strategy ideas, and inspect the assumptions behind every result." actions={<><Link className={secondaryButton} to="/market"><BarChart3 size={15} />Explore market</Link><Link className={primaryButton} to="/backtests/new"><FlaskConical size={15} />New backtest</Link></>} />
    {isError && <div className="flex items-center gap-2 rounded-xl border border-rose-500/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-300"><AlertCircle size={16} />API unavailable. Start the server with <code className="rounded bg-black/20 px-1.5">cryptool serve</code>.<button className="ml-auto" onClick={() => refetch()}><RefreshCw size={15} /></button></div>}
    <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <Metric label="Markets" value={isLoading ? '—' : (data?.products ?? 0).toLocaleString()} hint="Coinbase products" />
      <Metric label="Candles" value={isLoading ? '—' : (data?.candles ?? 0).toLocaleString()} hint="Across stored intervals" />
      <Metric label="Backtests" value={isLoading ? '—' : (data?.backtests ?? 0).toLocaleString()} hint="Saved research runs" />
      <Metric label="Strategies" value={isLoading ? '—' : (data?.strategies ?? 0).toLocaleString()} hint="Reusable configurations" />
    </div>
    <div className="grid gap-5 xl:grid-cols-[1.2fr_.8fr]">
      <Panel className="overflow-hidden">
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-4"><div><h2 className="font-medium text-white">Recent research</h2><p className="mt-1 text-xs text-slate-500">Your latest saved backtests</p></div><Link className="flex items-center gap-1 text-xs text-cyan-400 hover:text-cyan-300" to="/backtests">View all <ArrowRight size={13} /></Link></div>
        {recent.length === 0 ? <div className="p-10 text-center text-sm text-slate-500">No saved runs yet. Your first result will appear here.</div> : <div>{recent.map(bt => <Link key={bt.id} to={`/backtests/${bt.id}`} className="grid grid-cols-[1fr_auto] gap-4 border-b border-slate-800/70 px-5 py-4 transition last:border-0 hover:bg-slate-800/30"><div className="min-w-0"><div className="flex items-center gap-2"><span className="font-medium text-slate-200">{bt.product_id}</span><span className="rounded bg-slate-800 px-2 py-0.5 text-[10px] uppercase text-slate-500">{bt.granularity}</span></div><p className="mt-1 truncate text-xs text-slate-500">{bt.signals || `${bt.bull_signals} / ${bt.bear_signals}`} · {fmtDate(bt.created_at)}</p></div><div className="text-right"><p className={bt.total_return >= 0 ? 'font-semibold text-emerald-400' : 'font-semibold text-rose-400'}>{fmtPct(bt.total_return)}</p><p className="mt-1 text-xs text-slate-500">{bt.num_trades} trades</p></div></Link>)}</div>}
      </Panel>
      <div className="space-y-5">
        <Panel className="relative overflow-hidden p-5"><div className="absolute -right-10 -top-10 h-32 w-32 rounded-full bg-cyan-500/10 blur-2xl" /><p className="text-xs font-medium uppercase tracking-wider text-slate-500">Best observed return</p>{best ? <><p className={best.total_return >= 0 ? 'mt-4 text-4xl font-semibold text-emerald-400' : 'mt-4 text-4xl font-semibold text-rose-400'}>{fmtPct(best.total_return)}</p><p className="mt-2 text-sm text-slate-300">{best.product_id} · {best.granularity}</p><div className="mt-5 grid grid-cols-3 gap-2 text-xs"><div><p className="text-slate-600">Sharpe</p><p className="mt-1 text-slate-300">{best.sharpe.toFixed(2)}</p></div><div><p className="text-slate-600">Max DD</p><p className="mt-1 text-slate-300">{fmtPct(best.max_drawdown)}</p></div><div><p className="text-slate-600">Win rate</p><p className="mt-1 text-slate-300">{fmtPct(best.win_rate)}</p></div></div></> : <p className="mt-8 text-sm text-slate-500">Save a backtest to establish a benchmark.</p>}</Panel>
        <Panel className="p-5"><h2 className="font-medium text-white">Research paths</h2><div className="mt-4 space-y-2"><Link to="/candles" className="flex items-center gap-3 rounded-lg p-2 text-sm text-slate-400 hover:bg-slate-800 hover:text-white"><CandlestickChart size={16} className="text-violet-400" />Inspect price history</Link><Link to="/strategies" className="flex items-center gap-3 rounded-lg p-2 text-sm text-slate-400 hover:bg-slate-800 hover:text-white"><BookOpen size={16} className="text-amber-400" />Manage strategies</Link></div></Panel>
      </div>
    </div>
  </div>
}
