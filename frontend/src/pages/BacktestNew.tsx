import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, FlaskConical, Loader2 } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { api, type BacktestResult, type RunBacktestRequest } from '@/lib/api'
import { daysAgo, fmtPct, today } from '@/lib/utils'
import { Field, fieldClass, PageHeader, Panel, primaryButton } from '@/components/UI'

const SIGNALS = ['rsi', 'macd', 'bbands', 'ema_cross', 'sma']
const GRANS = ['1m','5m','15m','30m','1h','2h','6h','1d']
const DEFAULT: RunBacktestRequest = { exchange: 'coinbase', product_id: 'BTC-USD', granularity: '1h', start: daysAgo(365), end: today(), signals: ['rsi','macd'], bull_signals: ['sma','ema_cross'], bear_signals: ['rsi','bbands'], combination: 'voting', regime_fast: 50, regime_slow: 200, threshold: .5, fee_rate: .001, slippage: .001, tax_rate: .3, min_edge: .005, rr_min: 1.5, max_loss: 0, profit_gate: false, position_size: 0, capital: 0, save: true, top: 10 }

function SignalPicker({ value, onChange }: { value: string[]; onChange: (v: string[]) => void }) {
  return <div className="flex flex-wrap gap-2">{SIGNALS.map(s => <button key={s} type="button" onClick={() => onChange(value.includes(s) ? value.filter(x => x !== s) : [...value,s])} className={value.includes(s) ? 'rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-3 py-2 text-xs font-medium uppercase text-cyan-300' : 'rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-xs font-medium uppercase text-slate-500 hover:text-slate-300'}>{s}</button>)}</div>
}

export default function BacktestNew() {
  const [form, setForm] = useState(DEFAULT)
  const [mode, setMode] = useState<'single'|'sweep'>('single')
  const navigate = useNavigate()
  const qc = useQueryClient()
  const products = useQuery({ queryKey: ['products',''], queryFn: () => api.products() })
  const run = useMutation<{ results: BacktestResult[]; single: boolean }, Error, RunBacktestRequest>({
    mutationFn: async v => mode === 'single'
      ? { results: [await api.runBacktest(v)], single: true }
      : { results: await api.runCombinations(v), single: false },
    onSuccess: data => {
      qc.invalidateQueries({ queryKey: ['backtests'] })
      const result = data.results[0]
      if (data.single && result?.id) navigate(`/backtests/${result.id}`)
    },
  })
  const setNum = (key: keyof RunBacktestRequest, value: string) => setForm(f => ({ ...f, [key]: Number(value) }))
  const adaptive = form.combination === 'adaptive'
  return <div className="space-y-6">
    <PageHeader eyebrow="Experiment builder" title="Run a backtest" description="The complete CLI engine is available here: adaptive regimes, execution costs, risk gates, fixed sizing, and exhaustive signal sweeps." />
    <div className="grid gap-6 xl:grid-cols-[1fr_340px]">
      <form onSubmit={e => { e.preventDefault(); run.mutate(form) }} className="space-y-5">
        <Panel className="p-5"><div className="mb-5 flex items-center justify-between"><div><h2 className="font-medium text-white">Test scope</h2><p className="mt-1 text-xs text-slate-500">Market, time range, and experiment type</p></div><div className="flex rounded-lg bg-slate-950 p-1 text-xs"><button type="button" onClick={() => setMode('single')} className={`rounded-md px-3 py-1.5 ${mode === 'single' ? 'bg-slate-800 text-white' : 'text-slate-500'}`}>Single</button><button type="button" onClick={() => setMode('sweep')} className={`rounded-md px-3 py-1.5 ${mode === 'sweep' ? 'bg-slate-800 text-white' : 'text-slate-500'}`}>Signal sweep</button></div></div><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4"><Field label="Product"><select className={fieldClass} value={form.product_id} onChange={e => setForm(f => ({...f,product_id:e.target.value}))}><option value="BTC-USD">BTC-USD</option>{products.data?.filter(p => p.product_id !== 'BTC-USD').slice(0,300).map(p => <option key={p.product_id} value={p.product_id}>{p.display_name}</option>)}</select></Field><Field label="Granularity"><select className={fieldClass} value={form.granularity} onChange={e => setForm(f => ({...f,granularity:e.target.value}))}>{GRANS.map(g => <option key={g}>{g}</option>)}</select></Field><Field label="Start"><input className={fieldClass} type="date" value={form.start} onChange={e => setForm(f => ({...f,start:e.target.value}))} /></Field><Field label="End"><input className={fieldClass} type="date" value={form.end} onChange={e => setForm(f => ({...f,end:e.target.value}))} /></Field></div></Panel>
        <Panel className="p-5"><h2 className="font-medium text-white">Signal logic</h2><p className="mt-1 text-xs text-slate-500">Choose how indicator votes become long or short positions</p><div className="mt-5 grid gap-4 sm:grid-cols-3"><Field label="Combination"><select className={fieldClass} value={form.combination} onChange={e => setForm(f => ({...f,combination:e.target.value}))}>{['voting','consensus','weighted','adaptive'].map(c => <option key={c}>{c}</option>)}</select></Field><Field label="Voting threshold"><input className={fieldClass} type="number" min="0.01" max="1" step="0.05" value={form.threshold} onChange={e => setNum('threshold',e.target.value)} /></Field>{mode === 'sweep' && <Field label="Top results"><input className={fieldClass} type="number" min="1" max="31" value={form.top} onChange={e => setNum('top',e.target.value)} /></Field>}</div>{adaptive ? <div className="mt-5 space-y-5"><div className="grid gap-4 sm:grid-cols-2"><Field label="Bull-regime signals"><SignalPicker value={form.bull_signals ?? []} onChange={v => setForm(f => ({...f,bull_signals:v}))} /></Field><Field label="Bear-regime signals"><SignalPicker value={form.bear_signals ?? []} onChange={v => setForm(f => ({...f,bear_signals:v}))} /></Field></div><div className="grid gap-4 sm:grid-cols-2"><Field label="Fast regime period"><input className={fieldClass} type="number" value={form.regime_fast} onChange={e => setNum('regime_fast',e.target.value)} /></Field><Field label="Slow regime period"><input className={fieldClass} type="number" value={form.regime_slow} onChange={e => setNum('regime_slow',e.target.value)} /></Field></div></div> : mode === 'single' && <div className="mt-5"><Field label="Signals"><SignalPicker value={form.signals} onChange={v => setForm(f => ({...f,signals:v}))} /></Field></div>}</Panel>
        <Panel className="p-5"><h2 className="font-medium text-white">Execution & risk</h2><p className="mt-1 text-xs text-slate-500">Rates are decimals: 0.001 equals 0.1%</p><div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{[
          ['Fee rate','fee_rate','.0001'],['Slippage','slippage','.0001'],['Tax rate','tax_rate','.01'],['Minimum edge','min_edge','.001'],['Minimum R/R','rr_min','.1'],['Maximum loss','max_loss','.01'],['Position size ($)','position_size','1'],['Capital ($)','capital','1'],
        ].map(([label,key,step]) => <Field key={key} label={label}><input className={fieldClass} type="number" min="0" step={step} value={form[key as keyof RunBacktestRequest] as number} onChange={e => setNum(key as keyof RunBacktestRequest,e.target.value)} /></Field>)}</div><div className="mt-5 flex flex-wrap gap-6"><label className="flex items-center gap-2 text-sm text-slate-400"><input className="accent-cyan-500" type="checkbox" checked={form.profit_gate} onChange={e => setForm(f => ({...f,profit_gate:e.target.checked}))} />Only exit profitable positions</label><label className="flex items-center gap-2 text-sm text-slate-400"><input className="accent-cyan-500" type="checkbox" checked={form.save} onChange={e => setForm(f => ({...f,save:e.target.checked}))} />Save result and report detail</label></div></Panel>
        {run.isError && <p className="rounded-xl border border-rose-500/20 bg-rose-500/10 px-4 py-3 text-sm text-rose-400">{run.error.message}</p>}
        <button className={`${primaryButton} w-full py-3`} disabled={run.isPending || (adaptive ? !form.bull_signals?.length || !form.bear_signals?.length : mode === 'single' && !form.signals.length)}>{run.isPending ? <Loader2 size={16} className="animate-spin" /> : <FlaskConical size={16} />}{run.isPending ? 'Running engine…' : mode === 'single' ? 'Run backtest' : 'Test all signal combinations'}</button>
      </form>
      <aside className="space-y-5"><Panel className="p-5"><h2 className="font-medium text-white">Before you run</h2><ul className="mt-4 space-y-3 text-sm leading-5 text-slate-500"><li>Costs apply on both legs of every trade.</li><li>Taxes apply only to net profitable exits.</li><li>A signal sweep ranks in-sample results by Sharpe; validate winners out of sample.</li><li>Fixed position size requires capital at least as large as the position.</li></ul></Panel>{run.data && !run.data.single && <Panel className="overflow-hidden"><div className="border-b border-slate-800 px-4 py-3"><h2 className="font-medium text-white">Top combinations</h2></div>{run.data.results.map((r,i) => <Link key={`${r.signals}-${i}`} to={r.id ? `/backtests/${r.id}` : '#'} className="flex items-center justify-between border-b border-slate-800/70 px-4 py-3 last:border-0 hover:bg-slate-800/30"><div><p className="text-sm text-slate-200">#{i+1} {r.signals}</p><p className="text-xs text-slate-600">Sharpe {r.sharpe.toFixed(2)}</p></div><div className="flex items-center gap-2"><span className={r.total_return >= 0 ? 'text-sm text-emerald-400' : 'text-sm text-rose-400'}>{fmtPct(r.total_return)}</span><ArrowRight size={13} className="text-slate-600" /></div></Link>)}</Panel>}</aside>
    </div>
  </div>
}
