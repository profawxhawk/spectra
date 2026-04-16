import { useState, useCallback } from 'react'
import { Shell } from '@/components/layout/shell'
import { SpanTable } from '@/components/explorer/span-table'
import { FilterSidebar } from '@/components/explorer/filter-sidebar'
import { FilterBuilder } from '@/components/explorer/filter-builder'
import { SpanDetailPanel } from '@/components/explorer/span-detail-panel'
import { QueryBar } from '@/components/shared/query-bar'
import { TimeRangePicker } from '@/components/shared/time-range-picker'
import { Button } from '@/components/ui/button'
import { useSpans } from '@/hooks/use-spans'
import { useSearch } from '@/hooks/use-search'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import type { Span, SpanKind, SpanStatus, TimeRange, Filter } from '@/types'

const PAGE_SIZE = 50

export default function ExplorerPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedKinds, setSelectedKinds] = useState<Set<SpanKind>>(new Set())
  const [selectedStatuses, setSelectedStatuses] = useState<Set<SpanStatus>>(new Set())
  const [builderFilters, setBuilderFilters] = useState<Filter[]>([])
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)
  const [page, setPage] = useState(0)

  // Merge sidebar quick-toggles + builder filters
  const allFilters: Filter[] = [...builderFilters]
  if (selectedKinds.size > 0) {
    allFilters.push({ field: 'kind', operator: 'in', value: Array.from(selectedKinds).join(',') })
  }
  if (selectedStatuses.size > 0) {
    allFilters.push({ field: 'status', operator: 'in', value: Array.from(selectedStatuses).join(',') })
  }

  const hasSearch = searchQuery.length > 0
  const hasFilters = allFilters.length > 0

  const spanQuery = useSpans(
    {
      filters: hasFilters ? allFilters : [{ field: 'kind', operator: 'in', value: 'llm,tool,agent,retriever,chain,generic' }],
      order_by: 'start_time',
      ascending: false,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    },
    !hasSearch
  )

  const searchResult = useSearch(searchQuery, PAGE_SIZE, hasSearch)

  const result = hasSearch ? searchResult : spanQuery
  const spans = result.data?.spans || []
  const totalCount = result.data?.total_count || 0
  const hasMore = result.data?.has_more || false

  const toggleKind = useCallback((kind: SpanKind) => {
    setSelectedKinds((prev) => {
      const next = new Set(prev)
      if (next.has(kind)) next.delete(kind)
      else next.add(kind)
      return next
    })
    setPage(0)
  }, [])

  const toggleStatus = useCallback((status: SpanStatus) => {
    setSelectedStatuses((prev) => {
      const next = new Set(prev)
      if (next.has(status)) next.delete(status)
      else next.add(status)
      return next
    })
    setPage(0)
  }, [])

  const clearSidebar = useCallback(() => {
    setSelectedKinds(new Set())
    setSelectedStatuses(new Set())
    setPage(0)
  }, [])

  const handleBuilderApply = useCallback((filters: Filter[]) => {
    setBuilderFilters(filters)
    setPage(0)
  }, [])

  return (
    <Shell title="Explorer">
      <div className="flex h-full flex-col gap-4">
        <div className="flex items-center gap-4">
          <div className="flex-1">
            <QueryBar
              onSearch={(q) => { setSearchQuery(q); setPage(0) }}
              placeholder="Search spans by input, output, name..."
            />
          </div>
          <TimeRangePicker value={timeRange} onChange={setTimeRange} />
        </div>

        <FilterBuilder onApply={handleBuilderApply} activeCount={allFilters.length} />

        <div className="flex flex-1 gap-4 overflow-hidden">
          <FilterSidebar
            selectedKinds={selectedKinds}
            selectedStatuses={selectedStatuses}
            onToggleKind={toggleKind}
            onToggleStatus={toggleStatus}
            onClear={clearSidebar}
          />

          <div className="flex-1 overflow-auto rounded-md border border-border">
            {result.isLoading ? (
              <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">Loading...</div>
            ) : (
              <SpanTable
                spans={spans}
                onSelectSpan={setSelectedSpan}
                selectedSpanId={selectedSpan?.span_id}
              />
            )}
          </div>

          {selectedSpan && (
            <SpanDetailPanel span={selectedSpan} onClose={() => setSelectedSpan(null)} />
          )}
        </div>

        <div className="flex items-center justify-between border-t border-border pt-3">
          <span className="text-sm text-muted-foreground">
            {totalCount > 0 ? `${page * PAGE_SIZE + 1}–${Math.min((page + 1) * PAGE_SIZE, totalCount)} of ${totalCount} spans` : 'No results'}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>
              <ChevronLeft className="mr-1 h-4 w-4" /> Previous
            </Button>
            <Button variant="outline" size="sm" disabled={!hasMore} onClick={() => setPage(page + 1)}>
              Next <ChevronRight className="ml-1 h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </Shell>
  )
}
