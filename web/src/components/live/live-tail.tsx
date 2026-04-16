import { useState, useEffect, useRef, useCallback } from 'react'
import { format } from 'date-fns'
import { Pause, Play, Trash2, Circle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { StatusBadge } from '@/components/shared/status-badge'
import { KindBadge } from '@/components/shared/kind-badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { api } from '@/lib/api'
import type { Span } from '@/types'

const MAX_BUFFER = 500
const POLL_INTERVAL = 2000

export function LiveTail() {
  const [spans, setSpans] = useState<Span[]>([])
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const [connected, setConnected] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const lastSeenRef = useRef<string>('')

  const poll = useCallback(async () => {
    try {
      const result = await api.query({
        filters: [{ field: 'kind', operator: 'in', value: 'llm,tool,agent,retriever,chain,generic' }],
        order_by: 'start_time',
        ascending: false,
        limit: 20,
      })
      setConnected(true)

      if (result.spans.length > 0 && result.spans[0].span_id !== lastSeenRef.current) {
        lastSeenRef.current = result.spans[0].span_id
        setSpans((prev) => {
          const existingIds = new Set(prev.map((s) => s.span_id))
          const newSpans = result.spans.filter((s) => !existingIds.has(s.span_id))
          const combined = [...newSpans, ...prev]
          return combined.slice(0, MAX_BUFFER)
        })
      }
    } catch {
      setConnected(false)
    }
  }, [])

  useEffect(() => {
    if (paused) return
    poll()
    const interval = setInterval(poll, POLL_INTERVAL)
    return () => clearInterval(interval)
  }, [paused, poll])

  useEffect(() => {
    if (!paused && scrollRef.current) {
      scrollRef.current.scrollTop = 0
    }
  }, [spans, paused])

  const filteredSpans = filter
    ? spans.filter(
        (s) =>
          s.name.toLowerCase().includes(filter.toLowerCase()) ||
          s.trace_id.toLowerCase().includes(filter.toLowerCase()) ||
          s.kind.includes(filter.toLowerCase())
      )
    : spans

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <Circle className={`h-2.5 w-2.5 fill-current ${connected ? 'text-emerald-500' : 'text-red-500'}`} />
          <span className="text-sm text-muted-foreground">
            {paused ? 'Paused' : connected ? 'Streaming' : 'Disconnected'}
          </span>
        </div>

        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter live spans..."
          className="max-w-xs"
        />

        <div className="ml-auto flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPaused(!paused)}
          >
            {paused ? <Play className="mr-1 h-4 w-4" /> : <Pause className="mr-1 h-4 w-4" />}
            {paused ? 'Resume' : 'Pause'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => { setSpans([]); lastSeenRef.current = '' }}
          >
            <Trash2 className="mr-1 h-4 w-4" /> Clear
          </Button>
        </div>
      </div>

      <div className="text-xs text-muted-foreground">
        {filteredSpans.length} spans in buffer (max {MAX_BUFFER})
      </div>

      <ScrollArea className="flex-1 rounded-md border border-border" ref={scrollRef}>
        <div className="space-y-0.5 p-2">
          {filteredSpans.map((span) => (
            <div
              key={span.span_id}
              className="flex items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-secondary/50 transition-colors"
            >
              <span className="shrink-0 font-mono text-xs text-muted-foreground">
                {format(new Date(span.start_time), 'HH:mm:ss.SSS')}
              </span>
              <KindBadge kind={span.kind} />
              <StatusBadge status={span.status} />
              <span className="truncate font-medium">{span.name}</span>
              <span className="ml-auto shrink-0 font-mono text-xs text-muted-foreground">
                {span.trace_id.slice(0, 10)}
              </span>
              {span.metrics?.latency_ms && (
                <span className="shrink-0 font-mono text-xs text-muted-foreground">
                  {span.metrics.latency_ms.toFixed(0)}ms
                </span>
              )}
            </div>
          ))}
          {filteredSpans.length === 0 && (
            <div className="flex h-32 items-center justify-center text-muted-foreground text-sm">
              {spans.length === 0 ? 'Waiting for spans...' : 'No spans match your filter'}
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
