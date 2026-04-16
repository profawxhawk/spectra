import { format } from 'date-fns'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/shared/status-badge'
import { KindBadge } from '@/components/shared/kind-badge'
import { JsonViewer } from '@/components/shared/json-viewer'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { Span } from '@/types'
import { Link } from 'react-router-dom'

interface SpanDetailPanelProps {
  span: Span
  onClose: () => void
}

export function SpanDetailPanel({ span, onClose }: SpanDetailPanelProps) {
  return (
    <div className="w-[400px] border-l border-border bg-zinc-950">
      <div className="flex items-center justify-between border-b border-border p-4">
        <h3 className="font-semibold truncate">{span.name}</h3>
        <Button variant="ghost" size="icon" onClick={onClose}>
          <X className="h-4 w-4" />
        </Button>
      </div>
      <ScrollArea className="h-[calc(100vh-8rem)]">
        <div className="space-y-4 p-4">
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <span className="text-xs text-muted-foreground">Span ID</span>
              <p className="font-mono text-xs">{span.span_id}</p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Trace ID</span>
              <Link to={`/trace/${span.trace_id}`} className="block font-mono text-xs text-primary hover:underline">
                {span.trace_id}
              </Link>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Kind</span>
              <div className="mt-1"><KindBadge kind={span.kind} /></div>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Status</span>
              <div className="mt-1"><StatusBadge status={span.status} /></div>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Start Time</span>
              <p className="font-mono text-xs">{format(new Date(span.start_time), 'yyyy-MM-dd HH:mm:ss.SSS')}</p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Latency</span>
              <p className="font-mono text-xs">{span.metrics?.latency_ms ? `${span.metrics.latency_ms}ms` : '—'}</p>
            </div>
          </div>

          {span.metrics && (
            <>
              <Separator />
              <div>
                <h4 className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">Metrics</h4>
                <div className="grid grid-cols-2 gap-2 text-sm">
                  {span.metrics.prompt_tokens > 0 && (
                    <div><span className="text-muted-foreground text-xs">Prompt tokens:</span> <span className="font-mono text-xs">{span.metrics.prompt_tokens}</span></div>
                  )}
                  {span.metrics.completion_tokens > 0 && (
                    <div><span className="text-muted-foreground text-xs">Completion tokens:</span> <span className="font-mono text-xs">{span.metrics.completion_tokens}</span></div>
                  )}
                  {span.metrics.total_tokens > 0 && (
                    <div><span className="text-muted-foreground text-xs">Total tokens:</span> <span className="font-mono text-xs">{span.metrics.total_tokens}</span></div>
                  )}
                  {span.metrics.cost_usd > 0 && (
                    <div><span className="text-muted-foreground text-xs">Cost:</span> <span className="font-mono text-xs">${span.metrics.cost_usd.toFixed(4)}</span></div>
                  )}
                </div>
              </div>
            </>
          )}

          {span.input && (
            <>
              <Separator />
              <JsonViewer data={span.input} label="Input" collapsed={false} />
            </>
          )}

          {span.output && (
            <>
              <Separator />
              <JsonViewer data={span.output} label="Output" collapsed={false} />
            </>
          )}

          {span.metadata && Object.keys(span.metadata).length > 0 && (
            <>
              <Separator />
              <JsonViewer data={span.metadata} label="Metadata" />
            </>
          )}

          {span.attributes && Object.keys(span.attributes).length > 0 && (
            <>
              <Separator />
              <JsonViewer data={span.attributes} label="Attributes" />
            </>
          )}

          {span.events && span.events.length > 0 && (
            <>
              <Separator />
              <div>
                <h4 className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">Events ({span.events.length})</h4>
                {span.events.map((event, i) => (
                  <div key={i} className="mb-2 rounded-md bg-zinc-900 p-2">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-medium">{event.name}</span>
                      <span className="text-muted-foreground">{format(new Date(event.timestamp), 'HH:mm:ss.SSS')}</span>
                    </div>
                    {event.data && <JsonViewer data={event.data} collapsed />}
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </ScrollArea>
    </div>
  )
}
