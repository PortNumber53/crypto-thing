const BASE = import.meta.env.VITE_API_URL ?? ''

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `HTTP ${res.status}`)
  }
  return res.json()
}

// ─── Types ───────────────────────────────────────────────────────────────────

export interface Stats {
  products: number
  candles: number
  backtests: number
  strategies: number
  best_backtest: BacktestResult | null
  recent_backtests: BacktestResult[]
}

export interface Product {
  exchange: string
  product_id: string
  display_name: string
  base_currency_id: string
  quote_currency_id: string
  price: number
  price_change_24h: number
  volume_24h: number
  status: string
  watched: boolean
  new_at: string | null
  candle_count: number
}

export interface Candle {
  time: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface SignalInfo {
  name: string
  display: string
  description: string
  params: Record<string, number>
}

export interface BacktestResult {
  id: number
  strategy_id: number | null
  exchange: string
  product_id: string
  granularity: string
  start_time: string
  end_time: string
  signals: string
  bull_signals: string
  bear_signals: string
  regime_fast: number
  regime_slow: number
  combination: string
  threshold: number
  fee_rate: number
  slippage: number
  tax_rate: number
  min_edge: number
  rr_min: number
  max_loss: number
  profit_gate: boolean
  position_size: number
  capital: number
  total_return: number
  sharpe: number
  sortino: number
  max_drawdown: number
  calmar: number
  win_rate: number
  num_trades: number
  created_at: string
  equity_curve?: EquityPoint[]
  trades?: BacktestTrade[]
}

export interface EquityPoint { time: string; equity: number; price: number }
export interface BacktestTrade {
  entry_time: string
  exit_time: string
  direction: 'long' | 'short'
  entry_price: number
  exit_price: number
  net_return: number
  profit: boolean
}

export interface Strategy {
  id: number
  name: string
  description: string
  signals: string
  bull_signals: string
  bear_signals: string
  combination: string
  regime_fast: number
  regime_slow: number
  threshold: number
  fee_rate: number
  slippage: number
  tax_rate: number
  min_edge: number
  rr_min: number
  max_loss: number
  profit_gate: boolean
  position_size: number
  capital: number
  created_at: string
  updated_at: string
}

export interface RunBacktestRequest {
  exchange?: string
  product_id: string
  granularity: string
  start: string
  end: string
  signals: string[]
  bull_signals?: string[]
  bear_signals?: string[]
  combination: string
  regime_fast?: number
  regime_slow?: number
  threshold: number
  fee_rate: number
  slippage: number
  tax_rate: number
  min_edge: number
  rr_min: number
  max_loss?: number
  profit_gate?: boolean
  position_size?: number
  capital?: number
  save: boolean
  top?: number
}

export interface RunStrategyRequest {
  exchange?: string
  product_id: string
  granularity: string
  start: string
  end: string
  save: boolean
}

export interface StrategyInput {
  name: string
  description: string
  signals: string
  bull_signals?: string
  bear_signals?: string
  combination: string
  regime_fast?: number
  regime_slow?: number
  threshold: number
  fee_rate: number
  slippage: number
  tax_rate: number
  min_edge: number
  rr_min: number
  max_loss?: number
  profit_gate?: boolean
  position_size?: number
  capital?: number
}

// ─── API calls ───────────────────────────────────────────────────────────────

export const api = {
  health: () => req<{ status: string; db: string }>('/api/health'),
  stats: () => req<Stats>('/api/stats'),

  products: (exchange = 'coinbase', q = '') =>
    req<Product[]>(`/api/products?exchange=${exchange}&q=${encodeURIComponent(q)}`),
  syncProducts: () => req<{ synced: number; total: number }>('/api/products/sync', { method: 'POST' }),

  candles: (product: string, granularity: string, start: string, end: string, limit = 500) =>
    req<Candle[]>(`/api/candles?product=${product}&granularity=${granularity}&start=${start}&end=${end}&limit=${limit}`),

  signals: () => req<SignalInfo[]>('/api/signals'),

  backtests: (product = '', limit = 50, strategyId?: number) => {
    const query = new URLSearchParams({ product, limit: String(limit) })
    if (strategyId != null) query.set('strategy_id', String(strategyId))
    return req<BacktestResult[]>(`/api/backtest/results?${query}`)
  },
  getBacktest: (id: number) => req<BacktestResult>(`/api/backtest/results/${id}`),
  deleteBacktest: (id: number) => req<{ deleted: boolean }>(`/api/backtest/results/${id}`, { method: 'DELETE' }),
  runBacktest: (body: RunBacktestRequest) =>
    req<BacktestResult>('/api/backtest/run', { method: 'POST', body: JSON.stringify(body) }),
  runCombinations: (body: RunBacktestRequest) =>
    req<BacktestResult[]>('/api/backtest/combo', { method: 'POST', body: JSON.stringify(body) }),

  strategies: () => req<Strategy[]>('/api/strategies'),
  strategy: (id: number) => req<Strategy>(`/api/strategies/${id}`),
  createStrategy: (body: StrategyInput) =>
    req<Strategy>('/api/strategies', { method: 'POST', body: JSON.stringify(body) }),
  updateStrategy: (id: number, body: StrategyInput) =>
    req<Strategy>(`/api/strategies/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteStrategy: (id: number) =>
    req<{ deleted: boolean }>(`/api/strategies/${id}`, { method: 'DELETE' }),
  runStrategy: (id: number, body: RunStrategyRequest) =>
    req<BacktestResult>(`/api/strategies/${id}/run`, { method: 'POST', body: JSON.stringify(body) }),
}
