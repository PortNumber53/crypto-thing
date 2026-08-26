import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { createChart, type UTCTimestamp } from 'lightweight-charts'
import { api, type EquityPoint } from '@/lib/api'
import { fmtDate, fmtDateTime, fmtPct, fmtUSD } from '@/lib/utils'
import { Loading, Metric, PageHeader, Panel, secondaryButton } from '@/components/UI'

function EquityChart({ points }: { points: EquityPoint[] }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!ref.current || !points.length) return
    const chart = createChart(ref.current, { width: ref.current.clientWidth, height: 340, layout: { background: { color: 'transparent' }, textColor: '#64748b' }, grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } }, rightPriceScale: { borderColor: '#1e293b' }, timeScale: { borderColor: '#1e293b', timeVisible: true } })
    const series = chart.addLineSeries({ color: '#22d3ee', lineWidth: 2, priceFormat: { type: 'price', precision: 2, minMove: .01 } })
    series.setData(points.map(p => ({ time: (new Date(p.time).getTime() / 1000) as UTCTimestamp, value: p.equity })))
    chart.timeScale().fitContent()
    const resize = () => ref.current && chart.applyOptions({ width: ref.current.clientWidth })
    window.addEventListener('resize', resize)
    return () => { window.removeEventListener('resize', resize); chart.remove() }
  }, [points])
  return <div ref={ref} />
}

export default function BacktestDetail() {
  const id = Number(useParams().id)
  const query = useQuery({ queryKey: ['backtest', id], queryFn: () => api.getBacktest(id), enabled: Number.isFinite(id) })
  if (query.isLoading) return <Loading label="Loading report" />
  if (!query.data) return <Panel className="p-8 text-center text-rose-400">{query.error?.message ?? 'Backtest not found'}</Panel>
  const r = query.data
  const trades = r.trades ?? []
  return <div className="space-y-6">
    <PageHeader eyebrow="Backtest report" title={`${r.product_id} · ${r.granularity}`} description={`${fmtDate(r.start_time)} — ${fmtDate(r.end_time)} · created ${fmtDateTime(r.created_at)}`} actions={<Link to="/backtests" className={secondaryButton}><ArrowLeft size={15} />All backtests</Link>} />
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4"><Metric label="Total return" value={fmtPct(r.total_return)} tone={r.total_return >= 0 ? 'positive' : 'negative'} /><Metric label="Sharpe ratio" value={r.sharpe.toFixed(3)} tone={r.sharpe >= 0 ? 'positive' : 'negative'} /><Metric label="Maximum drawdown" value={fmtPct(r.max_drawdown)} tone="negative" /><Metric label="Win rate" value={fmtPct(r.win_rate)} tone={r.win_rate >= .5 ? 'positive' : 'neutral'} /><Metric label="Sortino" value={r.sortino.toFixed(3)} /><Metric label="Calmar" value={r.calmar.toFixed(3)} /><Metric label="Trades" value={String(r.num_trades)} /><Metric label="Starting capital" value={r.capital ? fmtUSD(r.capital) : 'Variable'} /></div>
    <Panel className="p-5"><div className="mb-5"><h2 className="font-medium text-white">Equity curve</h2><p className="mt-1 text-xs text-slate-500">Mark-to-market portfolio value throughout the test</p></div>{r.equity_curve?.length ? <EquityChart points={r.equity_curve} /> : <p className="py-24 text-center text-sm text-slate-600">This older result has no stored equity curve.</p>}</Panel>
    <div className="grid gap-5 xl:grid-cols-[.8fr_1.2fr]">
      <Panel className="p-5"><h2 className="font-medium text-white">Configuration</h2><dl className="mt-5 grid grid-cols-2 gap-x-5 gap-y-4 text-sm">{[
        ['Signals', r.signals || 'Adaptive regime'], ['Combination', r.combination], ['Bull signals', r.bull_signals || '—'], ['Bear signals', r.bear_signals || '—'], ['Threshold', String(r.threshold)], ['Regime EMA', `${r.regime_fast} / ${r.regime_slow}`], ['Fee', fmtPct(r.fee_rate)], ['Slippage', fmtPct(r.slippage)], ['Tax', fmtPct(r.tax_rate)], ['Min edge', fmtPct(r.min_edge)], ['Risk / reward', String(r.rr_min)], ['Max loss', r.max_loss ? fmtPct(r.max_loss) : 'Disabled'], ['Profit gate', r.profit_gate ? 'Enabled' : 'Disabled'], ['Position size', r.position_size ? fmtPct(r.position_size) : 'Full equity'],
      ].map(([k,v]) => <div key={k}><dt className="text-xs text-slate-600">{k}</dt><dd className="mt-1 break-words text-slate-300">{v}</dd></div>)}</dl></Panel>
      <Panel className="overflow-hidden"><div className="border-b border-slate-800 px-5 py-4"><h2 className="font-medium text-white">Trade log</h2><p className="mt-1 text-xs text-slate-500">All completed positions, after costs and tax</p></div><div className="max-h-[520px] overflow-auto"><table className="w-full min-w-[650px] text-sm"><thead className="sticky top-0 bg-slate-900"><tr>{['Direction','Entry','Exit','Entry price','Exit price','Net return'].map((h,i) => <th key={h} className={`px-4 py-3 text-xs font-medium text-slate-500 ${i < 3 ? 'text-left' : 'text-right'}`}>{h}</th>)}</tr></thead><tbody>{trades.length ? trades.map((t,i) => <tr key={`${t.entry_time}-${i}`} className="border-t border-slate-800/70"><td className="px-4 py-3 capitalize text-slate-300">{t.direction}</td><td className="px-4 py-3 text-xs text-slate-500">{fmtDateTime(t.entry_time)}</td><td className="px-4 py-3 text-xs text-slate-500">{fmtDateTime(t.exit_time)}</td><td className="px-4 py-3 text-right">{fmtUSD(t.entry_price)}</td><td className="px-4 py-3 text-right">{fmtUSD(t.exit_price)}</td><td className={`px-4 py-3 text-right font-medium ${t.profit ? 'text-emerald-400' : 'text-rose-400'}`}>{fmtPct(t.net_return)}</td></tr>) : <tr><td colSpan={6} className="py-20 text-center text-slate-600">No completed trades</td></tr>}</tbody></table></div></Panel>
    </div>
  </div>
}
