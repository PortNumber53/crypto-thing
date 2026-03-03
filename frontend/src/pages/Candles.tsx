import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { createChart, type IChartApi, type ISeriesApi, type UTCTimestamp } from 'lightweight-charts'
import { api, type Candle } from '@/lib/api'
import { daysAgo, today, fmtUSD, fmtDateTime } from '@/lib/utils'
import { Loader2, RefreshCw } from 'lucide-react'

const GRANULARITIES = ['1m', '5m', '15m', '30m', '1h', '2h', '6h', '1d']

function CandleChart({ candles }: { candles: Candle[] }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const candleSeriesRef = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const volumeSeriesRef = useRef<ISeriesApi<'Histogram'> | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    const el = containerRef.current

    const chart = createChart(el, {
      width: el.clientWidth,
      height: 420,
      layout: { background: { color: '#0f172a' }, textColor: '#94a3b8' },
      grid: { vertLines: { color: '#1e293b' }, horzLines: { color: '#1e293b' } },
      crosshair: { mode: 1 },
      rightPriceScale: { borderColor: '#1e293b' },
      timeScale: { borderColor: '#1e293b', timeVisible: true, secondsVisible: false },
    })

    const cs = chart.addCandlestickSeries({
      upColor: '#10b981', downColor: '#f43f5e',
      borderUpColor: '#10b981', borderDownColor: '#f43f5e',
      wickUpColor: '#10b981', wickDownColor: '#f43f5e',
    })

    const vs = chart.addHistogramSeries({
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
    })
    chart.priceScale('vol').applyOptions({ scaleMargins: { top: 0.85, bottom: 0 } })

    chartRef.current = chart
    candleSeriesRef.current = cs
    volumeSeriesRef.current = vs

    const handleResize = () => {
      if (containerRef.current) chart.applyOptions({ width: containerRef.current.clientWidth })
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
    }
  }, [])

  useEffect(() => {
    if (!candleSeriesRef.current || !volumeSeriesRef.current || !candles.length) return
    const cData = candles.map(c => ({
      time: (new Date(c.time).getTime() / 1000) as UTCTimestamp,
      open: c.open, high: c.high, low: c.low, close: c.close,
    }))
    const vData = candles.map(c => ({
      time: (new Date(c.time).getTime() / 1000) as UTCTimestamp,
      value: c.volume,
      color: c.close >= c.open ? '#10b98133' : '#f43f5e33',
    }))
    candleSeriesRef.current.setData(cData)
    volumeSeriesRef.current.setData(vData)
    chartRef.current?.timeScale().fitContent()
  }, [candles])

  return <div ref={containerRef} className="w-full" />
}

export default function Candles() {
  const [params, setParams] = useSearchParams()
  const [product, setProduct] = useState(params.get('product') || 'BTC-USD')
  const [granularity, setGranularity] = useState('1h')
  const [start, setStart] = useState(daysAgo(90))
  const [end, setEnd] = useState(today())
  const [limit, setLimit] = useState(1000)
  const [submitted, setSubmitted] = useState({ product, granularity, start, end, limit })

  const { data: products = [] } = useQuery({
    queryKey: ['products', ''],
    queryFn: () => api.products(),
  })

  const { data: candles = [], isFetching, isError, error } = useQuery({
    queryKey: ['candles', submitted.product, submitted.granularity, submitted.start, submitted.end, submitted.limit],
    queryFn: () => api.candles(submitted.product, submitted.granularity, submitted.start, submitted.end, submitted.limit),
    enabled: !!submitted.product,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitted({ product, granularity, start, end, limit })
    setParams({ product })
  }

  const last = candles[candles.length - 1]
  const first = candles[0]

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-5">
      <div>
        <h1 className="text-xl font-bold text-slate-100">Candles</h1>
        <p className="text-sm text-slate-400 mt-0.5">Historical OHLCV candlestick data</p>
      </div>

      {/* Controls */}
      <form onSubmit={handleSubmit} className="bg-slate-900 border border-slate-800 rounded-xl p-4">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
          <div className="lg:col-span-2">
            <label className="block text-xs text-slate-500 mb-1">Product</label>
            <select
              value={product}
              onChange={e => setProduct(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
            >
              {products.slice(0, 200).map(p => (
                <option key={p.product_id} value={p.product_id}>{p.display_name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs text-slate-500 mb-1">Granularity</label>
            <select
              value={granularity}
              onChange={e => setGranularity(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
            >
              {GRANULARITIES.map(g => <option key={g} value={g}>{g}</option>)}
            </select>
          </div>

          <div>
            <label className="block text-xs text-slate-500 mb-1">Start</label>
            <input
              type="date" value={start} onChange={e => setStart(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-xs text-slate-500 mb-1">End</label>
            <input
              type="date" value={end} onChange={e => setEnd(e.target.value)}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div className="flex items-end">
            <button
              type="submit"
              className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-blue-500 hover:bg-blue-400 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {isFetching ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
              Load
            </button>
          </div>
        </div>
      </form>

      {/* Stats bar */}
      {candles.length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {[
            { label: 'Candles', value: candles.length.toLocaleString() },
            { label: 'First', value: first ? fmtDateTime(first.time) : '—' },
            { label: 'Last', value: last ? fmtDateTime(last.time) : '—' },
            { label: 'Latest Close', value: last ? fmtUSD(last.close) : '—' },
          ].map(s => (
            <div key={s.label} className="bg-slate-900 border border-slate-800 rounded-lg px-4 py-3">
              <p className="text-xs text-slate-500 mb-1">{s.label}</p>
              <p className="text-sm font-medium text-slate-200">{s.value}</p>
            </div>
          ))}
        </div>
      )}

      {/* Chart */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
        {isFetching ? (
          <div className="flex items-center justify-center h-[420px]">
            <Loader2 size={24} className="animate-spin text-slate-500" />
          </div>
        ) : isError ? (
          <div className="flex items-center justify-center h-[420px] text-rose-400 text-sm">
            {(error as Error)?.message}
          </div>
        ) : candles.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-[420px] text-slate-500 text-sm gap-2">
            <CandlestickChartPlaceholder />
            No candle data found. Run <code className="font-mono text-xs bg-slate-800 px-1 rounded">./cryptool exchange coinbase data fetch</code> first.
          </div>
        ) : (
          <CandleChart candles={candles} />
        )}
      </div>

      {/* Data table */}
      {candles.length > 0 && (
        <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-800">
            <h2 className="text-sm font-semibold text-slate-300">Raw Data (last 50)</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-800">
                  {['Time', 'Open', 'High', 'Low', 'Close', 'Volume'].map(h => (
                    <th key={h} className={`px-4 py-2 text-xs font-medium text-slate-500 ${h === 'Time' ? 'text-left' : 'text-right'}`}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {candles.slice(-50).reverse().map((c, i) => (
                  <tr key={i} className="border-b border-slate-800/50 hover:bg-slate-800/30">
                    <td className="px-4 py-2 text-slate-400 text-xs">{fmtDateTime(c.time)}</td>
                    <td className="px-4 py-2 text-right text-slate-300">{fmtUSD(c.open)}</td>
                    <td className="px-4 py-2 text-right text-emerald-400">{fmtUSD(c.high)}</td>
                    <td className="px-4 py-2 text-right text-rose-400">{fmtUSD(c.low)}</td>
                    <td className={`px-4 py-2 text-right font-medium ${c.close >= c.open ? 'text-emerald-400' : 'text-rose-400'}`}>{fmtUSD(c.close)}</td>
                    <td className="px-4 py-2 text-right text-slate-400">{c.volume.toLocaleString(undefined, { maximumFractionDigits: 4 })}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

function CandlestickChartPlaceholder() {
  return (
    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M8 6v12M8 4v2M8 18v2M16 4v16M16 2v2M16 20v2" />
      <rect x="5" y="8" width="6" height="5" rx="1" />
      <rect x="13" y="7" width="6" height="8" rx="1" />
    </svg>
  )
}
