import { useQuery } from '@tanstack/react-query'
import { api, type SignalInfo } from '@/lib/api'
import { Zap, Loader2 } from 'lucide-react'

const COLORS: Record<string, string> = {
  rsi: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
  macd: 'text-violet-400 bg-violet-500/10 border-violet-500/20',
  bbands: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
  ema_cross: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
}

function SignalCard({ s }: { s: SignalInfo }) {
  const color = COLORS[s.name] ?? 'text-slate-400 bg-slate-800 border-slate-700'
  return (
    <div className={`rounded-xl border p-5 space-y-3 ${color.split(' ').slice(1).join(' ')} border`}>
      <div className="flex items-center gap-3">
        <div className={`w-9 h-9 rounded-lg border flex items-center justify-center ${color}`}>
          <Zap size={16} />
        </div>
        <div>
          <h2 className="text-sm font-bold text-slate-100">{s.display}</h2>
          <code className="text-xs text-slate-500">{s.name}</code>
        </div>
      </div>

      <p className="text-sm text-slate-300 leading-relaxed">{s.description}</p>

      <div>
        <p className="text-xs text-slate-500 mb-2 font-medium uppercase tracking-wide">Default Parameters</p>
        <div className="grid grid-cols-3 gap-2">
          {Object.entries(s.params).map(([k, v]) => (
            <div key={k} className="bg-slate-950/50 rounded-lg px-3 py-2">
              <p className="text-xs text-slate-500">{k}</p>
              <p className="text-sm font-mono font-medium text-slate-200">{v}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default function Signals() {
  const { data: signals = [], isLoading } = useQuery({
    queryKey: ['signals'],
    queryFn: api.signals,
  })

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-5">
      <div>
        <h1 className="text-xl font-bold text-slate-100">Signals</h1>
        <p className="text-sm text-slate-400 mt-0.5">
          Technical indicators used to generate buy/sell signals in backtests
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Loader2 size={20} className="animate-spin text-slate-500" />
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {signals.map(s => <SignalCard key={s.name} s={s} />)}
        </div>
      )}

      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
        <h2 className="text-sm font-semibold text-slate-300">Combination Methods</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          {[
            {
              name: 'Voting',
              desc: 'A trade is entered when at least N% of selected signals agree (controlled by the threshold parameter). More democratic — single conflicting signals are overruled.',
            },
            {
              name: 'Consensus',
              desc: 'ALL selected signals must agree before a trade is entered. Most conservative — highest signal quality, fewest trades.',
            },
            {
              name: 'Weighted',
              desc: 'Each signal vote is tallied and the majority wins. Similar to voting but without a configurable threshold — pure majority rule.',
            },
          ].map(c => (
            <div key={c.name} className="bg-slate-800/50 rounded-lg p-4">
              <p className="text-sm font-medium text-slate-200 mb-1">{c.name}</p>
              <p className="text-xs text-slate-400 leading-relaxed">{c.desc}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3">
        <h2 className="text-sm font-semibold text-slate-300">Profitability Gate</h2>
        <p className="text-sm text-slate-400 leading-relaxed">
          Every trade entry is filtered through a profitability gate before execution. A trade is only taken when:
        </p>
        <ul className="space-y-1.5 text-sm text-slate-400">
          <li className="flex gap-2">
            <span className="text-blue-400 shrink-0">→</span>
            <span>Expected net gain exceeds <strong className="text-slate-300">Min Edge</strong> after fees and slippage</span>
          </li>
          <li className="flex gap-2">
            <span className="text-blue-400 shrink-0">→</span>
            <span>Estimated reward-to-risk ratio exceeds <strong className="text-slate-300">R/R Min</strong></span>
          </li>
          <li className="flex gap-2">
            <span className="text-blue-400 shrink-0">→</span>
            <span>Tax impact on profitable trades is accounted for at the configured <strong className="text-slate-300">Tax Rate</strong></span>
          </li>
        </ul>
      </div>
    </div>
  )
}
