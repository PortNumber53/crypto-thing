import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type Strategy, type StrategyInput, type RunStrategyRequest } from '@/lib/api'
import { fmtPct, daysAgo, today } from '@/lib/utils'
import { Plus, Pencil, Trash2, Play, Loader2, BookMarked, X, TrendingUp, TrendingDown } from 'lucide-react'

const SIGNALS_ALL = ['rsi', 'macd', 'bbands', 'ema_cross']
const COMBOS = ['voting', 'consensus', 'weighted']

const DEFAULT_INPUT: StrategyInput = {
  name: '', description: '',
  signals: 'rsi,macd,bbands,ema_cross',
  combination: 'voting', threshold: 0.5,
  fee_rate: 0.001, slippage: 0.001,
  tax_rate: 0.30, min_edge: 0.005, rr_min: 1.5,
}

function StrategyForm({
  initial, onSave, onCancel, saving,
}: {
  initial: StrategyInput
  onSave: (v: StrategyInput) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<StrategyInput>(initial)

  const toggleSignal = (s: string) => {
    const arr = form.signals.split(',').filter(Boolean)
    const next = arr.includes(s) ? arr.filter(x => x !== s) : [...arr, s]
    setForm(f => ({ ...f, signals: next.join(',') }))
  }

  const active = form.signals.split(',').filter(Boolean)

  return (
    <div className="bg-slate-900 border border-blue-500/30 rounded-xl p-5 space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="md:col-span-2">
          <label className="block text-xs text-slate-500 mb-1">Name *</label>
          <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            placeholder="e.g. Momentum Swing"
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
        </div>
        <div className="md:col-span-2">
          <label className="block text-xs text-slate-500 mb-1">Description</label>
          <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
            placeholder="Optional description"
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
        </div>

        <div>
          <label className="block text-xs text-slate-500 mb-1">Combination</label>
          <select value={form.combination} onChange={e => setForm(f => ({ ...f, combination: e.target.value }))}
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
            {COMBOS.map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>

        <div>
          <label className="block text-xs text-slate-500 mb-1">Threshold: {form.threshold}</label>
          <input type="range" min={0.1} max={1} step={0.1} value={form.threshold}
            onChange={e => setForm(f => ({ ...f, threshold: parseFloat(e.target.value) }))}
            className="w-full accent-blue-500 mt-2" />
        </div>
      </div>

      <div>
        <label className="block text-xs text-slate-500 mb-2">Signals</label>
        <div className="flex flex-wrap gap-2">
          {SIGNALS_ALL.map(s => (
            <button key={s} type="button" onClick={() => toggleSignal(s)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                active.includes(s)
                  ? 'bg-blue-500/20 text-blue-400 border border-blue-500/40'
                  : 'bg-slate-800 text-slate-500 border border-slate-700 hover:border-slate-600'
              }`}>
              {s.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        {([
          ['Fee Rate', 'fee_rate'],
          ['Slippage', 'slippage'],
          ['Tax Rate', 'tax_rate'],
          ['Min Edge', 'min_edge'],
          ['R/R Min', 'rr_min'],
        ] as [string, keyof StrategyInput][]).map(([label, key]) => (
          <div key={key}>
            <label className="block text-xs text-slate-500 mb-1">{label}</label>
            <input type="number" step="0.0001" value={Number(form[key])}
              onChange={e => setForm(f => ({ ...f, [key]: parseFloat(e.target.value) }))}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
          </div>
        ))}
      </div>

      <div className="flex items-center gap-3">
        <button onClick={() => onSave(form)} disabled={saving || !form.name || !active.length}
          className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-400 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
          {saving ? <Loader2 size={13} className="animate-spin" /> : null}
          Save Strategy
        </button>
        <button onClick={onCancel} className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg transition-colors">
          Cancel
        </button>
      </div>
    </div>
  )
}

function RunModal({ strategy, onClose }: { strategy: Strategy; onClose: () => void }) {
  const qc = useQueryClient()
  const { data: products = [] } = useQuery({ queryKey: ['products', ''], queryFn: () => api.products() })
  const [req, setReq] = useState<RunStrategyRequest>({
    product_id: 'BTC-USD', granularity: '1h',
    start: daysAgo(365), end: today(), save: true,
  })

  const run = useMutation({
    mutationFn: () => api.runStrategy(strategy.id, req),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backtests'] }),
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4">
      <div className="bg-slate-900 border border-slate-700 rounded-xl w-full max-w-md p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-slate-200">Run: {strategy.name}</h2>
          <button onClick={onClose} className="text-slate-500 hover:text-slate-300"><X size={16} /></button>
        </div>

        <div>
          <label className="block text-xs text-slate-500 mb-1">Product</label>
          <select value={req.product_id} onChange={e => setReq(r => ({ ...r, product_id: e.target.value }))}
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
            {products.slice(0, 200).map(p => <option key={p.product_id} value={p.product_id}>{p.display_name}</option>)}
          </select>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <div>
            <label className="block text-xs text-slate-500 mb-1">Granularity</label>
            <select value={req.granularity} onChange={e => setReq(r => ({ ...r, granularity: e.target.value }))}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500">
              {['1m','5m','15m','30m','1h','2h','6h','1d'].map(g => <option key={g}>{g}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs text-slate-500 mb-1">Start</label>
            <input type="date" value={req.start} onChange={e => setReq(r => ({ ...r, start: e.target.value }))}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label className="block text-xs text-slate-500 mb-1">End</label>
            <input type="date" value={req.end} onChange={e => setReq(r => ({ ...r, end: e.target.value }))}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500" />
          </div>
        </div>

        <label className="flex items-center gap-2 cursor-pointer">
          <input type="checkbox" checked={req.save} onChange={e => setReq(r => ({ ...r, save: e.target.checked }))} className="accent-blue-500" />
          <span className="text-sm text-slate-400">Save result</span>
        </label>

        {run.isSuccess && (
          <div className="rounded-lg bg-emerald-500/10 border border-emerald-500/20 p-3 space-y-1">
            <div className="flex items-center gap-2 text-emerald-400 text-sm font-medium">
              {run.data.total_return >= 0 ? <TrendingUp size={14} /> : <TrendingDown size={14} />}
              Return: {fmtPct(run.data.total_return)} · Sharpe: {run.data.sharpe.toFixed(2)} · {run.data.num_trades} trades
            </div>
          </div>
        )}
        {run.isError && <p className="text-xs text-rose-400">{run.error?.message}</p>}

        <div className="flex gap-3">
          <button onClick={() => run.mutate()} disabled={run.isPending}
            className="flex-1 flex items-center justify-center gap-2 py-2.5 bg-blue-500 hover:bg-blue-400 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
            {run.isPending ? <Loader2 size={13} className="animate-spin" /> : <Play size={13} />}
            {run.isPending ? 'Running…' : 'Run'}
          </button>
          <button onClick={onClose} className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm rounded-lg">Close</button>
        </div>
      </div>
    </div>
  )
}

export default function Strategies() {
  const qc = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editTarget, setEditTarget] = useState<Strategy | null>(null)
  const [runTarget, setRunTarget] = useState<Strategy | null>(null)

  const { data: strategies = [], isLoading } = useQuery({ queryKey: ['strategies'], queryFn: api.strategies })

  const create = useMutation({
    mutationFn: api.createStrategy,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['strategies'] }); setShowForm(false) },
  })
  const update = useMutation({
    mutationFn: (v: StrategyInput) => api.updateStrategy(editTarget!.id, v),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['strategies'] }); setEditTarget(null) },
  })
  const del = useMutation({
    mutationFn: api.deleteStrategy,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['strategies'] }),
  })

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Strategies</h1>
          <p className="text-sm text-slate-400 mt-0.5">Saved signal configurations you can reuse across backtests</p>
        </div>
        <button onClick={() => { setShowForm(true); setEditTarget(null) }}
          className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-400 text-white text-sm font-medium rounded-lg transition-colors">
          <Plus size={14} /> New Strategy
        </button>
      </div>

      {showForm && !editTarget && (
        <StrategyForm
          initial={DEFAULT_INPUT}
          onSave={v => create.mutate(v)}
          onCancel={() => setShowForm(false)}
          saving={create.isPending}
        />
      )}

      {isLoading ? (
        <div className="flex justify-center py-16"><Loader2 size={20} className="animate-spin text-slate-500" /></div>
      ) : strategies.length === 0 && !showForm ? (
        <div className="bg-slate-900 border border-slate-800 rounded-xl flex flex-col items-center justify-center py-20 gap-3">
          <BookMarked size={32} className="text-slate-700" />
          <p className="text-slate-500 text-sm">No strategies yet</p>
          <button onClick={() => setShowForm(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-400 text-white text-sm font-medium rounded-lg">
            <Plus size={14} /> Create First Strategy
          </button>
        </div>
      ) : (
        <div className="space-y-3">
          {strategies.map(s => (
            editTarget?.id === s.id ? (
              <StrategyForm
                key={s.id}
                initial={{ name: s.name, description: s.description, signals: s.signals, combination: s.combination,
                  threshold: s.threshold, fee_rate: s.fee_rate, slippage: s.slippage, tax_rate: s.tax_rate, min_edge: s.min_edge, rr_min: s.rr_min }}
                onSave={v => update.mutate(v)}
                onCancel={() => setEditTarget(null)}
                saving={update.isPending}
              />
            ) : (
              <div key={s.id} className="bg-slate-900 border border-slate-800 rounded-xl p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="text-sm font-semibold text-slate-200">{s.name}</h3>
                    {s.description && <p className="text-xs text-slate-500 mt-0.5">{s.description}</p>}
                    <div className="flex flex-wrap gap-1.5 mt-2">
                      {s.signals.split(',').map(sig => (
                        <span key={sig} className="px-2 py-0.5 bg-blue-500/10 text-blue-400 rounded text-xs">{sig.toUpperCase()}</span>
                      ))}
                      <span className="px-2 py-0.5 bg-slate-700 text-slate-400 rounded text-xs">{s.combination}</span>
                    </div>
                    <div className="flex flex-wrap gap-3 mt-2 text-xs text-slate-600">
                      <span>Fee {(s.fee_rate * 100).toFixed(2)}%</span>
                      <span>Slip {(s.slippage * 100).toFixed(2)}%</span>
                      <span>Tax {(s.tax_rate * 100).toFixed(0)}%</span>
                      <span>MinEdge {(s.min_edge * 100).toFixed(2)}%</span>
                      <span>R/R {s.rr_min}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <button onClick={() => setRunTarget(s)}
                      className="p-1.5 text-slate-500 hover:text-emerald-400 hover:bg-emerald-500/10 rounded transition-colors" title="Run">
                      <Play size={14} />
                    </button>
                    <button onClick={() => { setEditTarget(s); setShowForm(false) }}
                      className="p-1.5 text-slate-500 hover:text-blue-400 hover:bg-blue-500/10 rounded transition-colors" title="Edit">
                      <Pencil size={14} />
                    </button>
                    <button onClick={() => del.mutate(s.id)}
                      className="p-1.5 text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 rounded transition-colors" title="Delete">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              </div>
            )
          ))}
        </div>
      )}

      {runTarget && <RunModal strategy={runTarget} onClose={() => setRunTarget(null)} />}
    </div>
  )
}
