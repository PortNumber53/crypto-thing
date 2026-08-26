import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { Activity, BarChart3, BookOpen, CandlestickChart, FlaskConical, LayoutDashboard, Menu, X, Zap } from 'lucide-react'
import { cn } from '@/lib/utils'

const NAV = [
  { to: '/dashboard', icon: LayoutDashboard, label: 'Overview' },
  { to: '/market', icon: BarChart3, label: 'Market' },
  { to: '/candles', icon: CandlestickChart, label: 'Charts' },
  { to: '/backtests', icon: FlaskConical, label: 'Backtests' },
  { to: '/strategies', icon: BookOpen, label: 'Strategies' },
  { to: '/signals', icon: Zap, label: 'Signals' },
]

export default function Layout({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  return <div className="min-h-screen bg-slate-950 text-slate-200">
    <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_70%_-20%,rgba(6,182,212,.13),transparent_38%),radial-gradient(circle_at_0%_50%,rgba(99,102,241,.08),transparent_32%)]" />
    <aside className={cn('fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-800/80 bg-slate-950/95 backdrop-blur-xl transition-transform lg:translate-x-0', open ? 'translate-x-0' : '-translate-x-full')}>
      <div className="flex items-center gap-3 border-b border-slate-800/80 px-5 py-4">
        <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-cyan-400 to-blue-600 shadow-lg shadow-cyan-500/20"><Activity size={18} className="text-white" /></div>
        <div><p className="font-semibold tracking-tight text-white">Crypto Thing</p><p className="text-[10px] uppercase tracking-[.18em] text-slate-500">Research console</p></div>
      </div>
      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-5">
        <p className="mb-3 px-3 text-[10px] font-semibold uppercase tracking-[.2em] text-slate-600">Workspace</p>
        {NAV.map(({ to, icon: Icon, label }) => <NavLink key={to} to={to} onClick={() => setOpen(false)} className={({ isActive }) => cn('flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm transition', isActive ? 'bg-cyan-500/10 font-medium text-cyan-300 ring-1 ring-cyan-500/15' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-100')}>
          <Icon size={16} />{label}
        </NavLink>)}
      </nav>
      <div className="border-t border-slate-800/80 p-4"><div className="rounded-xl bg-slate-900 p-3"><div className="flex items-center gap-2 text-xs text-emerald-400"><span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />Local research mode</div><p className="mt-1 text-[11px] text-slate-600">Historical analysis only</p></div></div>
    </aside>
    {open && <button aria-label="Close navigation" className="fixed inset-0 z-30 bg-black/70 lg:hidden" onClick={() => setOpen(false)} />}
    <div className="relative lg:pl-64">
      <header className="sticky top-0 z-20 flex h-16 items-center justify-between border-b border-slate-800/70 bg-slate-950/80 px-4 backdrop-blur-xl lg:px-8">
        <button aria-label="Open navigation" onClick={() => setOpen(!open)} className="rounded-lg p-2 text-slate-400 hover:bg-slate-900 lg:hidden">{open ? <X size={20} /> : <Menu size={20} />}</button>
        <div className="hidden text-xs text-slate-500 sm:block">Data-driven strategy research</div>
        <div className="ml-auto flex items-center gap-2 rounded-full border border-slate-800 bg-slate-900/80 px-3 py-1.5 text-xs text-slate-400"><span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />API connected</div>
      </header>
      <main className="mx-auto max-w-[1500px] p-4 sm:p-6 lg:p-8">{children}</main>
    </div>
  </div>
}
