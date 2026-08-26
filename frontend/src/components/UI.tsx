import type { ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'

export const fieldClass = 'w-full rounded-lg border border-slate-700 bg-slate-950/70 px-3 py-2 text-sm text-slate-100 outline-none transition focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/10'
export const primaryButton = 'inline-flex items-center justify-center gap-2 rounded-lg bg-cyan-500 px-4 py-2 text-sm font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-50'
export const secondaryButton = 'inline-flex items-center justify-center gap-2 rounded-lg border border-slate-700 bg-slate-900 px-4 py-2 text-sm font-medium text-slate-200 transition hover:border-slate-600 hover:bg-slate-800'

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description: string; actions?: ReactNode }) {
  return <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
    <div>
      {eyebrow && <p className="mb-2 text-xs font-semibold uppercase tracking-[0.2em] text-cyan-400">{eyebrow}</p>}
      <h1 className="text-2xl font-semibold tracking-tight text-white sm:text-3xl">{title}</h1>
      <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-400">{description}</p>
    </div>
    {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
  </div>
}

export function Panel({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <section className={cn('rounded-2xl border border-slate-800 bg-slate-900/70 shadow-xl shadow-black/10 backdrop-blur', className)}>{children}</section>
}

export function Metric({ label, value, tone = 'neutral', hint }: { label: string; value: string; tone?: 'positive' | 'negative' | 'neutral'; hint?: string }) {
  return <Panel className="p-4">
    <p className="text-xs font-medium uppercase tracking-wider text-slate-500">{label}</p>
    <p className={cn('mt-2 text-xl font-semibold tabular-nums', tone === 'positive' ? 'text-emerald-400' : tone === 'negative' ? 'text-rose-400' : 'text-slate-100')}>{value}</p>
    {hint && <p className="mt-1 text-xs text-slate-500">{hint}</p>}
  </Panel>
}

export function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return <label className="block">
    <span className="mb-1.5 block text-xs font-medium text-slate-400">{label}</span>
    {children}
    {hint && <span className="mt-1 block text-xs text-slate-600">{hint}</span>}
  </label>
}

export function Loading({ label = 'Loading' }: { label?: string }) {
  return <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-slate-500"><Loader2 size={16} className="animate-spin" />{label}</div>
}

export function Empty({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return <Panel className="flex min-h-56 flex-col items-center justify-center p-8 text-center">
    <h2 className="font-medium text-slate-200">{title}</h2>
    <p className="mt-2 max-w-md text-sm text-slate-500">{body}</p>
    {action && <div className="mt-5">{action}</div>}
  </Panel>
}
