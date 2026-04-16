import { Badge } from '@/components/ui/badge'
import type { SpanStatus } from '@/types'

export function StatusBadge({ status }: { status: SpanStatus }) {
  return (
    <Badge
      variant={status === 'error' ? 'destructive' : 'secondary'}
      className={status === 'ok' ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20' : ''}
    >
      {status}
    </Badge>
  )
}
