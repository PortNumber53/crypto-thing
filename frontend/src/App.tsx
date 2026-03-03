import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Products from './pages/Products'
import Candles from './pages/Candles'
import Backtest from './pages/Backtest'
import Strategies from './pages/Strategies'
import Signals from './pages/Signals'

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/products" element={<Products />} />
        <Route path="/candles" element={<Candles />} />
        <Route path="/backtest" element={<Backtest />} />
        <Route path="/strategies" element={<Strategies />} />
        <Route path="/signals" element={<Signals />} />
      </Routes>
    </Layout>
  )
}
