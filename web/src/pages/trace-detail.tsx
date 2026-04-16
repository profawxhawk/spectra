import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { Shell } from '@/components/layout/shell'
import { SpanTree } from '@/components/trace/span-tree'
import { Waterfall } from '@/components/trace/waterfall'
import { SpanInfo } from '@/components/trace/span-info'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useTrace } from '@/hooks/use-traces'
import type { Span } from '@/types'

export default function TraceDetailPage() {
  const { traceId } = useParams<{ traceId: string }>()
  const { data, isLoading, error } = useTrace(traceId || '')
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)

  const spans = data?.spans || []

  return (
    <Shell title={`Trace: ${traceId?.slice(0, 16) || ''}`}>
      <div className="flex h-full flex-col gap-4">
        <div className="flex items-center gap-3">
          <Link to="/explorer">
            <Button variant="ghost" size="sm">
              <ArrowLeft className="mr-1 h-4 w-4" /> Back
            </Button>
          </Link>
          <div>
            <h2 className="font-mono text-sm">{traceId}</h2>
            <p className="text-xs text-muted-foreground">{spans.length} spans</p>
          </div>
        </div>

        {isLoading ? (
          <div className="flex h-64 items-center justify-center text-muted-foreground">Loading trace...</div>
        ) : error ? (
          <div className="flex h-64 items-center justify-center text-red-400">Failed to load trace</div>
        ) : (
          <div className="flex flex-1 gap-4 overflow-hidden">
            <div className="flex-1 overflow-auto">
              <Tabs defaultValue="waterfall">
                <TabsList>
                  <TabsTrigger value="waterfall">Waterfall</TabsTrigger>
                  <TabsTrigger value="tree">Span Tree</TabsTrigger>
                </TabsList>
                <TabsContent value="waterfall" className="mt-4">
                  <Waterfall
                    spans={spans}
                    selectedSpanId={selectedSpan?.span_id}
                    onSelectSpan={setSelectedSpan}
                  />
                </TabsContent>
                <TabsContent value="tree" className="mt-4">
                  <SpanTree
                    spans={spans}
                    selectedSpanId={selectedSpan?.span_id}
                    onSelectSpan={setSelectedSpan}
                  />
                </TabsContent>
              </Tabs>
            </div>

            {selectedSpan && (
              <div className="w-[380px] border-l border-border">
                <SpanInfo span={selectedSpan} />
              </div>
            )}
          </div>
        )}
      </div>
    </Shell>
  )
}
