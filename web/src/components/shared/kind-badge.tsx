import { Badge } from '@/components/ui/badge'
import { SPAN_KIND_COLORS, type SpanKind } from '@/types'

export function KindBadge({ kind }: { kind: SpanKind }) {
  const color = SPAN_KIND_COLORS[kind] || SPAN_KIND_COLORS.generic
  return (
    <Badge
      variant="outline"
      className="font-mono text-xs"
      style={{ borderColor: color, color }}
    >
      {kind}
    </Badge>
  )
}
