import { useState } from 'react'
import { Link } from 'react-router-dom'
import { format } from 'date-fns'
import { Shell } from '@/components/layout/shell'
import { QueryBar } from '@/components/shared/query-bar'
import { StatusBadge } from '@/components/shared/status-badge'
import { KindBadge } from '@/components/shared/kind-badge'
import { Card, CardContent } from '@/components/ui/card'
import { useSearch } from '@/hooks/use-search'

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const { data, isLoading, error } = useSearch(query, 100)
  const spans = data?.spans || []

  return (
    <Shell title="Search">
      <div className="space-y-6">
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
                <Card key={span.span_id} className="transition-colors hover:bg-secondary/30">
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
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        )}
      </div>
    </Shell>
  )
}
