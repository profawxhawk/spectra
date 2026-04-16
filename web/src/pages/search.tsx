import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { format } from 'date-fns'
import { ExternalLink } from 'lucide-react'
import { Shell } from '@/components/layout/shell'
import { QueryBar } from '@/components/shared/query-bar'
import { StatusBadge } from '@/components/shared/status-badge'
import { KindBadge } from '@/components/shared/kind-badge'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { SpanDetailPanel } from '@/components/explorer/span-detail-panel'
import { useSearch } from '@/hooks/use-search'
import type { Span } from '@/types'

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const { data, isLoading, error } = useSearch(query, 100)
  const navigate = useNavigate()
  const spans = data?.spans || []

  return (
    <Shell title="Search">
      <div className="flex h-full gap-4">
        <div className="flex-1 space-y-6 overflow-auto">
          <div>
            <p className="mb-3 text-sm text-muted-foreground">
              Full-text search across span inputs, outputs, and names
            </p>
            <QueryBar onSearch={setQuery} placeholder="Search for 'machine learning', 'error', 'timeout'..." />
          </div>

          {query && isLoading && (
            <div className="flex h-32 items-center justify-center text-muted-foreground">Searching...</div>
          )}

          {error && (
            <div className="flex h-32 items-center justify-center text-red-400">Search failed</div>
          )}

          {query && !isLoading && spans.length === 0 && (
            <div className="flex h-32 items-center justify-center text-muted-foreground">
              No results for "{query}"
            </div>
          )}

          {spans.length > 0 && (
            <div>
              <p className="mb-3 text-sm text-muted-foreground">
                {data?.total_count || spans.length} results
              </p>
              <div className="space-y-2">
                {spans.map((span) => (
                  <Card
                    key={span.span_id}
                    className={`cursor-pointer transition-colors hover:bg-secondary/30 ${selectedSpan?.span_id === span.span_id ? 'ring-1 ring-primary' : ''}`}
                    onClick={() => setSelectedSpan(span)}
                  >
                    <CardContent className="p-4">
                      <div className="flex items-start justify-between gap-4">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="font-medium">{span.name}</span>
                            <KindBadge kind={span.kind} />
                            <StatusBadge status={span.status} />
                          </div>
                          <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
                            <Link
                              to={`/trace/${span.trace_id}`}
                              className="font-mono text-primary hover:underline"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {span.trace_id.slice(0, 16)}
                            </Link>
                            <span>{format(new Date(span.start_time), 'MMM dd HH:mm:ss')}</span>
                            {span.metrics?.latency_ms && (
                              <span className="font-mono">{span.metrics.latency_ms.toFixed(0)}ms</span>
                            )}
                          </div>
                          {span.input && (
                            <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{span.input}</p>
                          )}
                          {span.output && (
                            <p className="mt-1 line-clamp-2 text-sm text-zinc-400">{span.output}</p>
                          )}
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="shrink-0"
                          onClick={(e) => {
                            e.stopPropagation()
                            navigate(`/trace/${span.trace_id}`)
                          }}
                        >
                          <ExternalLink className="mr-1 h-3 w-3" />
                          Trace
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          )}
        </div>

        {selectedSpan && (
          <SpanDetailPanel span={selectedSpan} onClose={() => setSelectedSpan(null)} />
        )}
      </div>
    </Shell>
  )
}
