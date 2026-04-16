import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'
import type { SpanKind, SpanStatus } from '@/types'

const KINDS: SpanKind[] = ['llm', 'tool', 'agent', 'retriever', 'chain', 'generic']
const STATUSES: SpanStatus[] = ['ok', 'error']

interface FilterSidebarProps {
  selectedKinds: Set<SpanKind>
  selectedStatuses: Set<SpanStatus>
  onToggleKind: (kind: SpanKind) => void
  onToggleStatus: (status: SpanStatus) => void
  onClear: () => void
}

export function FilterSidebar({
  selectedKinds,
  selectedStatuses,
  onToggleKind,
  onToggleStatus,
  onClear,
}: FilterSidebarProps) {
  const hasFilters = selectedKinds.size > 0 || selectedStatuses.size > 0

  return (
    <div className="w-48 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Filters</h3>
        {hasFilters && (
          <Button variant="ghost" size="sm" className="h-6 text-xs" onClick={onClear}>
            Clear
          </Button>
        )}
      </div>

      <div>
        <h4 className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">Kind</h4>
        <div className="space-y-1">
          {KINDS.map((kind) => (
            <button
              key={kind}
              onClick={() => onToggleKind(kind)}
              className={cn(
                'flex w-full items-center rounded-md px-2 py-1.5 text-sm transition-colors',
                selectedKinds.has(kind)
                  ? 'bg-primary/15 text-primary'
                  : 'text-muted-foreground hover:bg-secondary'
              )}
            >
              {kind}
            </button>
          ))}
        </div>
      </div>

      <Separator />

      <div>
        <h4 className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</h4>
        <div className="space-y-1">
          {STATUSES.map((status) => (
            <button
              key={status}
              onClick={() => onToggleStatus(status)}
              className={cn(
                'flex w-full items-center rounded-md px-2 py-1.5 text-sm transition-colors',
                selectedStatuses.has(status)
                  ? 'bg-primary/15 text-primary'
                  : 'text-muted-foreground hover:bg-secondary'
              )}
            >
              {status}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
