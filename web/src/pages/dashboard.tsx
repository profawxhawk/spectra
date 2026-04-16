import { Shell } from '@/components/layout/shell'
import { StatsCards } from '@/components/dashboard/stats-cards'
import { VolumeChart } from '@/components/dashboard/volume-chart'
import { ErrorChart } from '@/components/dashboard/error-chart'
import { LatencyChart } from '@/components/dashboard/latency-chart'
import { KindBreakdown } from '@/components/dashboard/kind-breakdown'
import { TimeRangePicker } from '@/components/shared/time-range-picker'
import { useRecentSpans } from '@/hooks/use-spans'
import { useState } from 'react'
import type { TimeRange } from '@/types'

export default function DashboardPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')
  const { data, isLoading } = useRecentSpans(200)
  const spans = data?.spans || []

  return (
    <Shell title="Dashboard">
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Overview of your AI trace data
          </p>
          <TimeRangePicker value={timeRange} onChange={setTimeRange} />
        </div>

        {isLoading ? (
          <div className="flex h-64 items-center justify-center text-muted-foreground">
            Loading dashboard...
          </div>
        ) : (
          <>
            <StatsCards spans={spans} />
            <div className="grid gap-4 lg:grid-cols-2">
              <VolumeChart spans={spans} />
              <ErrorChart spans={spans} />
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              <LatencyChart spans={spans} />
              <KindBreakdown spans={spans} />
            </div>
          </>
        )}
      </div>
    </Shell>
  )
}
