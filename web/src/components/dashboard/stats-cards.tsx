import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Activity, AlertTriangle, Clock, Layers } from 'lucide-react'
import type { Span } from '@/types'

interface StatsCardsProps {
  spans: Span[]
}

export function StatsCards({ spans }: StatsCardsProps) {
  const totalSpans = spans.length
  const uniqueTraces = new Set(spans.map((s) => s.trace_id)).size
  const errorCount = spans.filter((s) => s.status === 'error').length
  const avgLatency =
    spans.length > 0
      ? spans.reduce((sum, s) => sum + (s.metrics?.latency_ms || 0), 0) / spans.length
      : 0

  const stats = [
    { label: 'Total Spans', value: totalSpans.toLocaleString(), icon: Layers, color: 'text-blue-400' },
    { label: 'Unique Traces', value: uniqueTraces.toLocaleString(), icon: Activity, color: 'text-purple-400' },
    { label: 'Errors', value: errorCount.toLocaleString(), icon: AlertTriangle, color: 'text-red-400' },
    { label: 'Avg Latency', value: `${avgLatency.toFixed(0)}ms`, icon: Clock, color: 'text-amber-400' },
  ]

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {stats.map((stat) => (
        <Card key={stat.label}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{stat.label}</CardTitle>
            <stat.icon className={`h-4 w-4 ${stat.color}`} />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stat.value}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
