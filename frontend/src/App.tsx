import { Navigate, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Candles from './pages/Candles'
import Signals from './pages/Signals'
import Market from './pages/Market'
import Backtests from './pages/Backtests'
import BacktestNew from './pages/BacktestNew'
import BacktestDetail from './pages/BacktestDetail'
import StrategyList from './pages/StrategyList'
import StrategyEditor from './pages/StrategyEditor'
import StrategyDetail from './pages/StrategyDetail'

export default function App() {
  return <Layout><Routes>
    <Route path="/" element={<Navigate to="/dashboard" replace />} />
    <Route path="/dashboard" element={<Dashboard />} />
    <Route path="/market" element={<Market />} />
    <Route path="/products" element={<Navigate to="/market" replace />} />
    <Route path="/candles" element={<Candles />} />
    <Route path="/backtest" element={<Navigate to="/backtests/new" replace />} />
    <Route path="/backtests" element={<Backtests />} />
    <Route path="/backtests/new" element={<BacktestNew />} />
    <Route path="/backtests/:id" element={<BacktestDetail />} />
    <Route path="/strategies" element={<StrategyList />} />
    <Route path="/strategies/new" element={<StrategyEditor />} />
    <Route path="/strategies/:id" element={<StrategyDetail />} />
    <Route path="/strategies/:id/edit" element={<StrategyEditor />} />
    <Route path="/signals" element={<Signals />} />
    <Route path="*" element={<Navigate to="/dashboard" replace />} />
  </Routes></Layout>
}
