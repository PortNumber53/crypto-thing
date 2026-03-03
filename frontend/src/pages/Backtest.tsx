import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type BacktestResult, type RunBacktestRequest } from '@/lib/api'
import { fmtPct, fmtDate, daysAgo, today } from '@/lib/utils'
import { FlaskConical, Trash2, Loader2, TrendingUp, TrendingDown } from 'lucide-react'

const SIGNALS = ['rsi', 'macd', 'bbands', 'ema_cross']
const COMBOS = ['voting', 'consensus', 'weighted']
const GRANS = ['1m', '5m', '15m', '30m', '1h', '2h', '6h', '1d']

function MetricCard({ label, value, positive }: { label: string; value: string; positive?: boolean }) {
  return (
    <div className="bg-slate-800 rounded-lg px-4 py-3">
      <p className="text-xs text-slate-500 mb-1">{label}</p>
      <p className={`text-base font-bold ${positive == null ? 'text-slate-200' : positive ? 'text-emerald-400' : 'text-rose-400'}`}>
        {value}
      </p>
    </div>
  )
}

function ResultCard({ r, onDelete }: { r: BacktestResult; onDelete: () => void }) {
  const [expanded, setExpanded] = useState(false)
  const ret = r.total_return
  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
      <div
        className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-slate-800/40 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          <div className={`w-2 h-2 rounded-full ${ret >= 0 ? 'bg-emerald-400' : 'bg-rose-400'}`} />
          <div>
            <span className="text-sm font-medium text-slate-200">{r.product_id}</span>
            <span className="mx-2 text-slate-600">·</span>
            <span className="text-xs text-slate-500">{r.granularity} · {r.signals} · {r.combination}</span>
          </div>
        </div>
        <div className="flex items-center gap-4">
          <div className="hidden sm:flex items-center gap-4 text-sm">
            <span className={ret >= 0 ? 'text-emerald-400 font-semibold' : 'text-rose-400 font-semibold'}>
              {ret >= 0 ? <TrendingUp size={13} className="inline mr-1" /> : <TrendingDown size={13} className="inline mr-1" />}
              {fmtPct(ret)}
            </span>
            <span className="text-slate-500 text-xs">{r.num_trades} trades</span>
          </div>
          <button
            onClick={e => { e.stopPropagation(); onDelete() }}
            className="p-1.5 text-slate-600 hover:text-rose-400 hover:bg-rose-500/10 rounded transition-colors"
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>
      {expanded && (
        <div className="px-4 pb-4 border-t border-slate-800 pt-4 space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            <MetricCard label="Total Return" value={fmtPct(r.total_return)} positive={r.total_return >= 0} />
            <MetricCard label="Sharpe Ratio" value={r.sharpe.toFixed(3)} positive={r.sharpe > 0} />
            <MetricCard label="Sortino Ratio" value={r.sortino.toFixed(3)} positive={r.sortino > 0} />
            <MetricCard label="Max Drawdown" value={fmtPct(r.max_drawdown)} positive={false} />
            <MetricCard label="Win Rate" value={`${(r.win_rate * 100).toFixed(1)}%`} positive={r.win_rate > 0.5} />
            <MetricCard label="Calmar Ratio" value={r.calmar.toFixed(3)} positive={r.calmar > 0} />
            <MetricCard label="Trades" value={r.num_trades.toString()} />
            <MetricCard label="Period" value={`${fmtDate(r.start_time)} – ${fmtDate(r.end_time)}`} />
          </div>
          <div className="text-xs text-slate-500 flex flex-wrap gap-3">
            <span>Fee: {(r.fee_rate * 100).toFixed(2)}%</span>
            <span>Slippage: {(r.slippage * 100).toFixed(2)}%</span>
            <span>Tax: {(r.tax_rate * 100).toFixed(0)}%</span>
            <span>Min Edge: {(r.min_edge * 100).toFixed(2)}%</span>
            <span>R/R Min: {r.rr_min}</span>
            <span>Threshold: {r.threshold}</span>
          </div>
        </div>
      )}
    </div>
  )
}

const DEFAULT_FORM: RunBacktestRequest = {
  product_id: 'BTC-USD',
  granularity: '1h',
  start: daysAgo(365),
  end: today(),
  signals: ['rsi', 'macd'],
  combination: 'voting',
  threshold: 0.5,
  fee_rate: 0.001,
  slippage: 0.001,
  tax_rate: 0.30,
  min_edge: 0.005,
  rr_min: 1.5,
  save: true,
}

export default function Backtest() {
  const [form, setForm] = useState<RunBacktestRequest>(DEFAULT_FORM)
  const qc = useQueryClient()

  const { data: products = [] } = useQuery({ queryKey: ['products', ''], queryFn: () => api.products() })
  const { data: results = [], isLoading } = useQuery({ queryKey: ['backtests', '', 50], queryFn: () => api.backtests('', 50) })

  const run = useMutation({
    mutationFn: api.runBacktest,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backtests'] }),
  })

  const del = useMutation({
    mutationFn: (id: number) => api.deleteBacktest(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backtests'] }),
  })

  const toggleSignal = (s: string) => {
    setForm(f => ({
      ...f,
      signals: f.signals.includes(s) ? f.signals.filter(x => x !== s) : [...f.signals, s],
    }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.signals.length) return
    run.mutate(form)
  }

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div>
        <h1 className="text-xl font-bold text-slate-100">Backtest</h1>
        <p className="text-sm text-slate-400 mt-0.5">Run strategy simulations on historical candle data</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Config panel */}
        <form onSubmit={handleSubmit} className="lg:col-span-1 bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4 h-fit">
          <h2 className="text-sm font-semibold text-slate-300">Configuration</h2>

          <div>
            <label className="block text-xs text-slate-500 mb-1">Product</label>
            <select value={form.product_id} onChange={e => setForm(f => ({ ...f, product_id: e.target.value }))}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
              {products.slice(0, 200).map(p => <option key={p.product_id} value={p.product_id}>{p.display_name}</option>)}
            </select>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-slate-500 mb-1">Granularity</label>
              <select value={form.granularity} onChange={e => setForm(f => ({ ...f, granularity: e.target.value }))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
                {GRANS.map(g => <option key={g} value={g}>{g}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-slate-500 mb-1">Combination</label>
              <select value={form.combination} onChange={e => setForm(f => ({ ...f, combination: e.target.value }))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
                {COMBOS.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-slate-500 mb-1">Start</label>
              <input type="date" value={form.start} onChange={e => setForm(f => ({ ...f, start: e.target.value }))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
            </div>
            <div>
              <label className="block text-xs text-slate-500 mb-1">End</label>
              <input type="date" value={form.end} onChange={e => setForm(f => ({ ...f, end: e.target.value }))}
                className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
            </div>
          </div>

          <div>
            <label className="block text-xs text-slate-500 mb-2">Signals</label>
            <div className="grid grid-cols-2 gap-1.5">
              {SIGNALS.map(s => (
                <label key={s} className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={form.signals.includes(s)} onChange={() => toggleSignal(s)}
                    className="w-3.5 h-3.5 rounded border-slate-600 accent-blue-500" />
                  <span className="text-sm text-slate-300 uppercase">{s}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            {([
              ['Fee Rate', 'fee_rate', 0.0001, 0.01, 0.0001],
              ['Slippage', 'slippage', 0.0001, 0.01, 0.0001],
              ['Tax Rate', 'tax_rate', 0, 1, 0.01],
              ['Threshold', 'threshold', 0.1, 1, 0.1],
              ['Min Edge', 'min_edge', 0.001, 0.05, 0.001],
              ['R/R Min', 'rr_min', 0.5, 5, 0.1],
            ] as [string, keyof RunBacktestRequest, number, number, number][]).map(([label, key, min, max, step]) => (
              <div key={key}>
                <label className="block text-xs text-slate-500 mb-1">
                  {label}: <span className="text-slate-400">{Number(form[key]).toFixed(key === 'rr_min' ? 1 : 4)}</span>
                </label>
                <input type="range" min={min} max={max} step={step}
                  value={Number(form[key])}
                  onChange={e => setForm(f => ({ ...f, [key]: parseFloat(e.target.value) }))}
                  className="w-full accent-blue-500" />
              </div>
            ))}
          </div>

          <label className="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" checked={form.save} onChange={e => setForm(f => ({ ...f, save: e.target.checked }))}
              className="w-3.5 h-3.5 accent-blue-500" />
            <span className="text-sm text-slate-400">Save result to database</span>
          </label>

          <button type="submit" disabled={run.isPending || !form.signals.length}
            className="w-full flex items-center justify-center gap-2 py-2.5 bg-blue-500 hover:bg-blue-400 disabled:opacity-50 text-white text-sm font-semibold rounded-lg transition-colors">
            {run.isPending ? <Loader2 size={14} className="animate-spin" /> : <FlaskConical size={14} />}
            {run.isPending ? 'Running…' : 'Run Backtest'}
          </button>

          {run.isError && (
            <p className="text-xs text-rose-400">{run.error?.message}</p>
          )}
        </form>

        {/* Results panel */}
        <div className="lg:col-span-2 space-y-4">
          {/* Latest result highlight */}
          {run.isSuccess && (
            <div className="bg-slate-900 border border-blue-500/30 rounded-xl p-5 space-y-3">
              <h2 className="text-sm font-semibold text-blue-400">Latest Run — {run.data.product_id}</h2>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                <MetricCard label="Total Return" value={fmtPct(run.data.total_return)} positive={run.data.total_return >= 0} />
                <MetricCard label="Sharpe" value={run.data.sharpe.toFixed(3)} positive={run.data.sharpe > 0} />
                <MetricCard label="Win Rate" value={`${(run.data.win_rate * 100).toFixed(1)}%`} positive={run.data.win_rate > 0.5} />
                <MetricCard label="Max DD" value={fmtPct(run.data.max_drawdown)} positive={false} />
              </div>
            </div>
          )}

          <div>
            <h2 className="text-sm font-semibold text-slate-300 mb-3">
              All Results <span className="text-slate-500 font-normal">({results.length})</span>
            </h2>
            {isLoading ? (
              <div className="flex justify-center py-12"><Loader2 size={20} className="animate-spin text-slate-500" /></div>
            ) : results.length === 0 ? (
              <div className="bg-slate-900 border border-slate-800 rounded-xl flex items-center justify-center py-16 text-slate-500 text-sm">
                No results yet — run your first backtest
              </div>
            ) : (
              <div className="space-y-2">
                {results.map(r => (
                  <ResultCard key={r.id} r={r} onDelete={() => r.id && del.mutate(r.id)} />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
