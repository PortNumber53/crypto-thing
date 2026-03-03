import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function fmt(n: number, decimals = 2) {
  return n.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
}

export function fmtPct(n: number) {
  const sign = n >= 0 ? '+' : ''
  return `${sign}${fmt(n * 100, 2)}%`
}

export function fmtUSD(n: number) {
  if (n >= 1_000_000_000) return `$${fmt(n / 1_000_000_000, 2)}B`
  if (n >= 1_000_000) return `$${fmt(n / 1_000_000, 2)}M`
  if (n >= 1_000) return `$${fmt(n / 1_000, 2)}K`
  return `$${fmt(n, 2)}`
}

export function fmtDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

export function fmtDateTime(iso: string) {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export function today() {
  return new Date().toISOString().split('T')[0]
}

export function daysAgo(n: number) {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().split('T')[0]
}
