import { format } from 'date-fns'
import { Link } from 'react-router-dom'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatusBadge } from '@/components/shared/status-badge'
import { KindBadge } from '@/components/shared/kind-badge'
import type { Span } from '@/types'

interface SpanTableProps {
  spans: Span[]
  onSelectSpan: (span: Span) => void
  selectedSpanId?: string
}

export function SpanTable({ spans, onSelectSpan, selectedSpanId }: SpanTableProps) {
  if (spans.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        No spans found. Try adjusting your filters or search query.
      </div>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[180px]">Timestamp</TableHead>
          <TableHead className="w-[120px]">Trace ID</TableHead>
          <TableHead>Name</TableHead>
          <TableHead className="w-[100px]">Kind</TableHead>
          <TableHead className="w-[80px]">Status</TableHead>
          <TableHead className="w-[100px] text-right">Latency</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {spans.map((span) => (
          <TableRow
            key={span.span_id}
            className={`cursor-pointer ${selectedSpanId === span.span_id ? 'bg-muted' : ''}`}
            onClick={() => onSelectSpan(span)}
          >
            <TableCell className="font-mono text-xs text-muted-foreground">
              {format(new Date(span.start_time), 'MMM dd HH:mm:ss.SSS')}
            </TableCell>
            <TableCell>
              <Link
                to={`/trace/${span.trace_id}`}
                className="font-mono text-xs text-primary hover:underline"
                onClick={(e) => e.stopPropagation()}
              >
                {span.trace_id.slice(0, 12)}
              </Link>
            </TableCell>
            <TableCell className="font-medium">{span.name}</TableCell>
            <TableCell>
              <KindBadge kind={span.kind} />
            </TableCell>
            <TableCell>
              <StatusBadge status={span.status} />
            </TableCell>
            <TableCell className="text-right font-mono text-sm">
              {span.metrics?.latency_ms ? `${span.metrics.latency_ms.toFixed(0)}ms` : '—'}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
