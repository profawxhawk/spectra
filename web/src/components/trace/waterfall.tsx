import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { SPAN_KIND_COLORS, type Span } from '@/types'

interface WaterfallProps {
  spans: Span[]
  selectedSpanId?: string
  onSelectSpan: (span: Span) => void
}

export function Waterfall({ spans, selectedSpanId, onSelectSpan }: WaterfallProps) {
  const { rows, totalDuration } = useMemo(() => {
    if (spans.length === 0) return { rows: [], totalDuration: 0 }

    const starts = spans.map((s) => new Date(s.start_time).getTime())
    const ends = spans.map((s) => {
      if (s.end_time) return new Date(s.end_time).getTime()
      return new Date(s.start_time).getTime() + (s.metrics?.latency_ms || 0)
    })

    const minStart = Math.min(...starts)
    const maxEnd = Math.max(...ends)
    const total = maxEnd - minStart || 1

    const rows = spans.map((s, i) => {
      const start = starts[i] - minStart
      const duration = ends[i] - starts[i]
      return {
        span: s,
        leftPct: (start / total) * 100,
        widthPct: Math.max((duration / total) * 100, 0.5),
        durationMs: duration,
      }
    })

    return { rows, totalDuration: total }
  }, [spans])

  if (rows.length === 0) return null

  const timeMarkers = [0, 25, 50, 75, 100]

  return (
    <div className="relative">
      <div className="mb-1 flex text-xs text-muted-foreground">
        {timeMarkers.map((pct) => (
          <span key={pct} className="absolute" style={{ left: `${pct}%`, transform: 'translateX(-50%)' }}>
            {((totalDuration * pct) / 100).toFixed(0)}ms
          </span>
        ))}
      </div>
      <div className="mt-5 space-y-1">
        {rows.map(({ span, leftPct, widthPct, durationMs }) => (
          <button
            key={span.span_id}
            className={cn(
              'group relative flex h-7 w-full items-center rounded-sm transition-colors',
              selectedSpanId === span.span_id ? 'bg-primary/10' : 'hover:bg-secondary/50'
            )}
            onClick={() => onSelectSpan(span)}
          >
            <div
              className="absolute h-5 rounded-sm opacity-80 transition-opacity group-hover:opacity-100"
              style={{
                left: `${leftPct}%`,
                width: `${widthPct}%`,
                backgroundColor: SPAN_KIND_COLORS[span.kind] || SPAN_KIND_COLORS.generic,
              }}
            />
            <span
              className="absolute text-xs font-medium truncate"
              style={{
                left: `${leftPct + widthPct + 0.5}%`,
                maxWidth: `${100 - leftPct - widthPct - 1}%`,
              }}
            >
              {span.name} <span className="text-muted-foreground">{durationMs.toFixed(0)}ms</span>
            </span>
          </button>
        ))}
      </div>
    </div>
  )
}
