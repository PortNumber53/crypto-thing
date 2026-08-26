import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Loader2, Save } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, type StrategyInput } from '@/lib/api'
import StrategyForm, { defaultStrategy } from '@/components/StrategyForm'
import { PageHeader, primaryButton, secondaryButton } from '@/components/UI'

export default function StrategyEditor() {
  const id = Number(useParams().id)
  const editing = Number.isFinite(id)
  const [form, setForm] = useState<StrategyInput>(defaultStrategy)
  const navigate = useNavigate()
  const qc = useQueryClient()
  const current = useQuery({ queryKey: ['strategy',id], queryFn: () => api.strategy(id), enabled: editing })
  useEffect(() => { if (current.data) { const { id: _id, created_at: _created, updated_at: _updated, ...input } = current.data; void _id; void _created; void _updated; setForm(input) } }, [current.data])
  const save = useMutation({ mutationFn: () => editing ? api.updateStrategy(id,form) : api.createStrategy(form), onSuccess: result => { qc.invalidateQueries({ queryKey: ['strategies'] }); navigate(`/strategies/${result.id}`) } })
  return <div className="space-y-6"><PageHeader eyebrow={editing ? 'Edit configuration' : 'New configuration'} title={editing ? `Edit ${current.data?.name ?? 'strategy'}` : 'Create a strategy'} description="Capture signals, regime behavior, trading costs, and risk rules as one repeatable configuration." actions={<Link className={secondaryButton} to={editing ? `/strategies/${id}` : '/strategies'}><ArrowLeft size={15} />Cancel</Link>} /><form onSubmit={e => { e.preventDefault(); save.mutate() }} className="space-y-5"><StrategyForm value={form} onChange={setForm} />{save.isError && <p className="text-sm text-rose-400">{save.error.message}</p>}<button className={primaryButton} disabled={save.isPending || !form.name}>{save.isPending ? <Loader2 className="animate-spin" size={15} /> : <Save size={15} />}{editing ? 'Save changes' : 'Create strategy'}</button></form></div>
}
