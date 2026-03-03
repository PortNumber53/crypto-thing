import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard, Coins, CandlestickChart,
  FlaskConical, BookMarked, Zap, Menu, X,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useState } from 'react'

const NAV = [
  { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/products',  icon: Coins,            label: 'Products' },
  { to: '/candles',   icon: CandlestickChart, label: 'Candles' },
  { to: '/backtest',  icon: FlaskConical,     label: 'Backtest' },
  { to: '/strategies',icon: BookMarked,       label: 'Strategies' },
  { to: '/signals',   icon: Zap,              label: 'Signals' },
]

export default function Layout({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)

  return (
    <div className="flex h-screen bg-slate-950 text-slate-200 overflow-hidden">
      {/* Sidebar */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-40 flex flex-col w-60 bg-slate-900 border-r border-slate-800 transition-transform duration-200',
          open ? 'translate-x-0' : '-translate-x-full',
          'lg:static lg:translate-x-0 lg:flex',
        )}
      >
        {/* Logo */}
        <div className="flex items-center gap-2 px-5 py-4 border-b border-slate-800">
          <div className="w-8 h-8 rounded-lg bg-blue-500 flex items-center justify-center">
            <CandlestickChart size={16} className="text-white" />
          </div>
          <span className="font-bold text-slate-100 tracking-tight">CryptoTool</span>
        </div>

        {/* Nav */}
        <nav className="flex-1 py-4 space-y-0.5 px-2 overflow-y-auto">
          {NAV.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              onClick={() => setOpen(false)}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors',
                  isActive
                    ? 'bg-blue-500/10 text-blue-400 font-medium'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800',
                )
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>

        <div className="px-5 py-3 border-t border-slate-800">
          <p className="text-xs text-slate-600">crypto-thing v0.1</p>
        </div>
      </aside>

      {/* Overlay */}
      {open && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={() => setOpen(false)}
        />
      )}

      {/* Main */}
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden">
        {/* Mobile top bar */}
        <header className="lg:hidden flex items-center gap-3 px-4 py-3 border-b border-slate-800 bg-slate-900">
          <button
            onClick={() => setOpen(!open)}
            className="text-slate-400 hover:text-slate-200"
          >
            {open ? <X size={20} /> : <Menu size={20} />}
          </button>
          <span className="font-semibold text-slate-100">CryptoTool</span>
        </header>

        <main className="flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  )
}
