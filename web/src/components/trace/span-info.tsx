import { format } from 'date-fns'
import { KindBadge } from '@/components/shared/kind-badge'
import { StatusBadge } from '@/components/shared/status-badge'
import { JsonViewer } from '@/components/shared/json-viewer'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { Span } from '@/types'

interface SpanInfoProps {
  span: Span
}

export function SpanInfo({ span }: SpanInfoProps) {
  return (
    <ScrollArea className="h-full">
      <div className="space-y-4 p-4">
        <div>
          <h3 className="text-lg font-semibold">{span.name}</h3>
          <p className="mt-1 font-mono text-xs text-muted-foreground">{span.span_id}</p>
        </div>

        <div className="flex items-center gap-3">
          <KindBadge kind={span.kind} />
          <StatusBadge status={span.status} />
        </div>

        <div className="grid grid-cols-2 gap-3 text-sm">
          <div>
            <span className="text-xs text-muted-foreground">Start</span>
            <p className="font-mono text-xs">{format(new Date(span.start_time), 'HH:mm:ss.SSS')}</p>
          </div>
          {span.end_time && (
            <div>
              <span className="text-xs text-muted-foreground">End</span>
              <p className="font-mono text-xs">{format(new Date(span.end_time), 'HH:mm:ss.SSS')}</p>
            </div>
          )}
          <div>
            <span className="text-xs text-muted-foreground">Duration</span>
            <p className="font-mono text-xs">{span.metrics?.latency_ms ? `${span.metrics.latency_ms}ms` : '—'}</p>
          </div>
          {span.parent_id && (
            <div>
              <span className="text-xs text-muted-foreground">Parent</span>
              <p className="font-mono text-xs">{span.parent_id}</p>
            </div>
          )}
        </div>

        {span.metrics && (span.metrics.prompt_tokens > 0 || span.metrics.total_tokens > 0) && (
          <>
            <Separator />
            <div>
              <h4 className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">Tokens</h4>
              <div className="grid grid-cols-3 gap-2 text-center">
                <div className="rounded-md bg-zinc-900 p-2">
                  <p className="text-lg font-bold">{span.metrics.prompt_tokens}</p>
                  <p className="text-xs text-muted-foreground">prompt</p>
                </div>
                <div className="rounded-md bg-zinc-900 p-2">
                  <p className="text-lg font-bold">{span.metrics.completion_tokens}</p>
                  <p className="text-xs text-muted-foreground">completion</p>
                </div>
                <div className="rounded-md bg-zinc-900 p-2">
                  <p className="text-lg font-bold">{span.metrics.total_tokens}</p>
                  <p className="text-xs text-muted-foreground">total</p>
                </div>
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
      </div>
    </ScrollArea>
  )
}
