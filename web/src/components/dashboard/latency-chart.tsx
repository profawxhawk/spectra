import { useMemo } from 'react'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Span } from '@/types'

interface LatencyChartProps {
  spans: Span[]
}

export function LatencyChart({ spans }: LatencyChartProps) {
  const data = useMemo(() => {
    const latencies = spans
      .map((s) => s.metrics?.latency_ms || 0)
      .filter((l) => l > 0)
      .sort((a, b) => a - b)

    if (latencies.length === 0) return []

    const p = (pct: number) => latencies[Math.floor(latencies.length * pct)] || 0

    return [
      { label: 'P50', value: p(0.5) },
      { label: 'P90', value: p(0.9) },
      { label: 'P95', value: p(0.95) },
      { label: 'P99', value: p(0.99) },
    ]
  }, [spans])

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Latency Percentiles</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="h-[200px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data}>
              <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} />
              <YAxis
                tick={{ fontSize: 11, fill: '#71717a' }}
                axisLine={false}
                tickLine={false}
                width={50}
                tickFormatter={(v: number) => `${v}ms`}
              />
              <Tooltip
                contentStyle={{ backgroundColor: '#18181b', border: '1px solid #27272a', borderRadius: '8px', fontSize: 12 }}
                formatter={(value) => [`${Number(value).toFixed(0)}ms`, 'Latency']}
              />
              <Bar dataKey="value" fill="#f59e0b" fillOpacity={0.8} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}
