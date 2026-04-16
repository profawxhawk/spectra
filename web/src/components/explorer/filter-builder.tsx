import { useState } from 'react'
import { Plus, X, Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { Filter, Operator } from '@/types'

const FIELDS = [
  { value: 'kind', label: 'Kind', type: 'enum', options: ['llm', 'tool', 'agent', 'retriever', 'chain', 'generic'] },
  { value: 'status', label: 'Status', type: 'enum', options: ['ok', 'error'] },
  { value: 'name', label: 'Name', type: 'string' },
  { value: 'trace_id', label: 'Trace ID', type: 'string' },
  { value: 'span_id', label: 'Span ID', type: 'string' },
  { value: 'parent_id', label: 'Parent ID', type: 'string' },
  { value: 'input', label: 'Input', type: 'string' },
  { value: 'output', label: 'Output', type: 'string' },
] as const

const STRING_OPS: { value: Operator; label: string }[] = [
  { value: 'eq', label: '=' },
  { value: 'ne', label: '!=' },
  { value: 'contains', label: 'contains' },
]

const ENUM_OPS: { value: Operator; label: string }[] = [
  { value: 'eq', label: '=' },
  { value: 'ne', label: '!=' },
  { value: 'in', label: 'in' },
]

interface FilterRow {
  id: number
  field: string
  operator: Operator
  value: string
}

interface FilterBuilderProps {
  onApply: (filters: Filter[]) => void
  activeCount: number
}

let nextId = 1

export function FilterBuilder({ onApply, activeCount }: FilterBuilderProps) {
  const [rows, setRows] = useState<FilterRow[]>([])
  const [expanded, setExpanded] = useState(false)

  const addRow = () => {
    setRows([...rows, { id: nextId++, field: 'kind', operator: 'eq', value: '' }])
    setExpanded(true)
  }

  const removeRow = (id: number) => {
    const next = rows.filter((r) => r.id !== id)
    setRows(next)
    if (next.length === 0) {
      onApply([])
      setExpanded(false)
    }
  }

  const updateRow = (id: number, patch: Partial<FilterRow>) => {
    setRows(rows.map((r) => {
      if (r.id !== id) return r
      const updated = { ...r, ...patch }
      // Reset operator when field type changes
      if (patch.field) {
        const fieldDef = FIELDS.find((f) => f.value === patch.field)
        if (fieldDef?.type === 'enum' && !ENUM_OPS.some((o) => o.value === r.operator)) {
          updated.operator = 'eq'
        }
        updated.value = ''
      }
      return updated
    }))
  }

  const apply = () => {
    const filters: Filter[] = rows
      .filter((r) => r.value.trim() !== '')
      .map((r) => ({ field: r.field, operator: r.operator, value: r.value }))
    onApply(filters)
  }

  const clearAll = () => {
    setRows([])
    setExpanded(false)
    onApply([])
  }

  const getFieldDef = (field: string) => FIELDS.find((f) => f.value === field)
  const getOps = (field: string) => {
    const def = getFieldDef(field)
    return def?.type === 'enum' ? ENUM_OPS : STRING_OPS
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={addRow}>
          <Plus className="mr-1 h-3 w-3" /> Add Filter
        </Button>
        {activeCount > 0 && (
          <span className="rounded-md bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">
            {activeCount} active
          </span>
        )}
        {rows.length > 0 && (
          <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={clearAll}>
            Clear All
          </Button>
        )}
      </div>

      {expanded && rows.length > 0 && (
        <div className="space-y-2 rounded-md border border-border p-3">
          {rows.map((row) => {
            const fieldDef = getFieldDef(row.field)
            const ops = getOps(row.field)
            return (
              <div key={row.id} className="flex items-center gap-2">
                <Select value={row.field} onValueChange={(v) => updateRow(row.id, { field: v })}>
                  <SelectTrigger className="w-32">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FIELDS.map((f) => (
                      <SelectItem key={f.value} value={f.value}>{f.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                <Select value={row.operator} onValueChange={(v) => updateRow(row.id, { operator: v as Operator })}>
                  <SelectTrigger className="w-28">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ops.map((o) => (
                      <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>

                {fieldDef?.type === 'enum' && fieldDef.options ? (
                  <Select value={row.value} onValueChange={(v) => updateRow(row.id, { value: v })}>
                    <SelectTrigger className="w-40">
                      <SelectValue placeholder="Select value..." />
                    </SelectTrigger>
                    <SelectContent>
                      {fieldDef.options.map((opt) => (
                        <SelectItem key={opt} value={opt}>{opt}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Input
                    value={row.value}
                    onChange={(e) => updateRow(row.id, { value: e.target.value })}
                    placeholder="Value..."
                    className="w-40"
                    onKeyDown={(e) => e.key === 'Enter' && apply()}
                  />
                )}

                <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => removeRow(row.id)}>
                  <X className="h-3 w-3" />
                </Button>
              </div>
            )
          })}
          <Button size="sm" onClick={apply}>
            <Search className="mr-1 h-3 w-3" /> Apply Filters
          </Button>
        </div>
      )}
    </div>
  )
}
