import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { Coins, CandlestickChart, FlaskConical, BookMarked, RefreshCw, AlertCircle } from 'lucide-react'
import { Link } from 'react-router-dom'

function StatCard({ label, value, icon: Icon, to, color }: {
  label: string; value: number | string; icon: React.ElementType; to: string; color: string
}) {
  return (
    <Link to={to} className="block bg-slate-900 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition-colors">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-slate-400">{label}</span>
        <div className={`w-8 h-8 rounded-lg ${color} flex items-center justify-center`}>
          <Icon size={15} className="text-white" />
        </div>
      </div>
      <p className="text-2xl font-bold text-slate-100">{value?.toLocaleString()}</p>
    </Link>
  )
}

export default function Dashboard() {
  const { data: stats, isLoading, isError, refetch } = useQuery({
    queryKey: ['stats'],
    queryFn: api.stats,
  })

  const { data: backtests } = useQuery({
    queryKey: ['backtests', '', 5],
    queryFn: () => api.backtests('', 5),
  })

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Dashboard</h1>
          <p className="text-sm text-slate-400 mt-0.5">Overview of your crypto backtesting workspace</p>
        </div>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-3 py-1.5 text-sm text-slate-400 hover:text-slate-200 bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors"
        >
          <RefreshCw size={13} />
          Refresh
        </button>
      </div>

      {isError && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm">
          <AlertCircle size={15} />
          Cannot reach API server. Make sure <code className="font-mono text-xs bg-slate-800 px-1 rounded">./cryptool serve</code> is running.
        </div>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Products" value={isLoading ? '…' : stats?.products ?? 0} icon={Coins} to="/products" color="bg-blue-500" />
        <StatCard label="Candles" value={isLoading ? '…' : stats?.candles ?? 0} icon={CandlestickChart} to="/candles" color="bg-violet-500" />
        <StatCard label="Backtests" value={isLoading ? '…' : stats?.backtests ?? 0} icon={FlaskConical} to="/backtest" color="bg-emerald-500" />
        <StatCard label="Strategies" value={isLoading ? '…' : stats?.strategies ?? 0} icon={BookMarked} to="/strategies" color="bg-amber-500" />
      </div>

      {/* Quick actions */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <h2 className="text-sm font-semibold text-slate-300 mb-4">Quick Actions</h2>
        <div className="flex flex-wrap gap-3">
          <Link to="/products" className="px-4 py-2 bg-blue-500 hover:bg-blue-400 text-white text-sm font-medium rounded-lg transition-colors">
            Sync Products
          </Link>
          <Link to="/candles" className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-medium rounded-lg transition-colors">
            View Candles
          </Link>
          <Link to="/backtest" className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-medium rounded-lg transition-colors">
            Run Backtest
          </Link>
          <Link to="/strategies" className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-medium rounded-lg transition-colors">
            New Strategy
          </Link>
        </div>
      </div>

      {/* Recent backtests */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-slate-300">Recent Backtests</h2>
          <Link to="/backtest" className="text-xs text-blue-400 hover:text-blue-300">View all →</Link>
        </div>
        {!backtests || backtests.length === 0 ? (
          <p className="text-sm text-slate-500 text-center py-6">No backtests yet. <Link to="/backtest" className="text-blue-400 hover:underline">Run your first one →</Link></p>
        ) : (
          <div className="space-y-2">
            {backtests.map((bt, i) => (
              <div key={bt.id ?? i} className="flex items-center justify-between py-2 border-b border-slate-800 last:border-0">
                <div>
                  <span className="text-sm font-medium text-slate-200">{bt.product_id}</span>
                  <span className="ml-2 text-xs text-slate-500">{bt.granularity} · {bt.signals}</span>
                </div>
                <div className="flex items-center gap-4 text-sm">
                  <span className={bt.total_return >= 0 ? 'text-emerald-400' : 'text-rose-400'}>
                    {bt.total_return >= 0 ? '+' : ''}{(bt.total_return * 100).toFixed(1)}%
                  </span>
                  <span className="text-slate-500 text-xs">{bt.num_trades} trades</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
