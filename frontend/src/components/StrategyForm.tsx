import type { StrategyInput } from '@/lib/api'
import { Field, fieldClass, Panel } from '@/components/UI'

export const SIGNALS = ['rsi','macd','bbands','ema_cross','sma']
export const defaultStrategy: StrategyInput = { name: '', description: '', signals: 'rsi,macd', bull_signals: 'sma,ema_cross', bear_signals: 'rsi,bbands', combination: 'voting', regime_fast: 50, regime_slow: 200, threshold: .5, fee_rate: .001, slippage: .001, tax_rate: .3, min_edge: .005, rr_min: 1.5, max_loss: 0, profit_gate: false, position_size: 0, capital: 0 }

function Picker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const active = value.split(',').filter(Boolean)
  return <div className="flex flex-wrap gap-2">{SIGNALS.map(s => <button key={s} type="button" onClick={() => onChange((active.includes(s) ? active.filter(x => x !== s) : [...active,s]).join(','))} className={active.includes(s) ? 'rounded-lg border border-cyan-500/40 bg-cyan-500/10 px-3 py-2 text-xs uppercase text-cyan-300' : 'rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-xs uppercase text-slate-500'}>{s}</button>)}</div>
}

export default function StrategyForm({ value, onChange }: { value: StrategyInput; onChange: (v: StrategyInput) => void }) {
  const set = (key: keyof StrategyInput, next: string | number | boolean) => onChange({ ...value, [key]: next })
  const adaptive = value.combination === 'adaptive'
  return <div className="space-y-5">
    <Panel className="p-5"><h2 className="font-medium text-white">Identity</h2><div className="mt-5 grid gap-4 sm:grid-cols-2"><Field label="Strategy name"><input autoFocus className={fieldClass} value={value.name} onChange={e => set('name',e.target.value)} placeholder="Momentum with risk gate" /></Field><Field label="Description"><input className={fieldClass} value={value.description} onChange={e => set('description',e.target.value)} placeholder="What hypothesis does this test?" /></Field></div></Panel>
    <Panel className="p-5"><h2 className="font-medium text-white">Signal logic</h2><div className="mt-5 grid gap-4 sm:grid-cols-3"><Field label="Combination"><select className={fieldClass} value={value.combination} onChange={e => set('combination',e.target.value)}>{['voting','consensus','weighted','adaptive'].map(c => <option key={c}>{c}</option>)}</select></Field><Field label="Threshold"><input className={fieldClass} type="number" step=".05" min=".01" max="1" value={value.threshold} onChange={e => set('threshold',Number(e.target.value))} /></Field>{adaptive && <><Field label="Fast regime period"><input className={fieldClass} type="number" value={value.regime_fast} onChange={e => set('regime_fast',Number(e.target.value))} /></Field><Field label="Slow regime period"><input className={fieldClass} type="number" value={value.regime_slow} onChange={e => set('regime_slow',Number(e.target.value))} /></Field></>}</div><div className="mt-5 grid gap-5 sm:grid-cols-2">{adaptive ? <><Field label="Bull-regime signals"><Picker value={value.bull_signals ?? ''} onChange={v => set('bull_signals',v)} /></Field><Field label="Bear-regime signals"><Picker value={value.bear_signals ?? ''} onChange={v => set('bear_signals',v)} /></Field></> : <Field label="Signals"><Picker value={value.signals} onChange={v => set('signals',v)} /></Field>}</div></Panel>
    <Panel className="p-5"><h2 className="font-medium text-white">Execution & risk defaults</h2><div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{[
      ['Fee rate','fee_rate','.0001'],['Slippage','slippage','.0001'],['Tax rate','tax_rate','.01'],['Minimum edge','min_edge','.001'],['Minimum R/R','rr_min','.1'],['Maximum loss','max_loss','.01'],['Position size ($)','position_size','1'],['Capital ($)','capital','1'],
    ].map(([label,key,step]) => <Field key={key} label={label}><input className={fieldClass} type="number" min="0" step={step} value={Number(value[key as keyof StrategyInput] ?? 0)} onChange={e => set(key as keyof StrategyInput,Number(e.target.value))} /></Field>)}</div><label className="mt-5 flex items-center gap-2 text-sm text-slate-400"><input className="accent-cyan-500" type="checkbox" checked={value.profit_gate ?? false} onChange={e => set('profit_gate',e.target.checked)} />Only exit profitable positions</label></Panel>
  </div>
}
