import { useMemo } from 'react'
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { SPAN_KIND_COLORS, type Span } from '@/types'

interface KindBreakdownProps {
  spans: Span[]
}

export function KindBreakdown({ spans }: KindBreakdownProps) {
  const data = useMemo(() => {
    const counts = new Map<string, number>()
    spans.forEach((s) => counts.set(s.kind, (counts.get(s.kind) || 0) + 1))
    return Array.from(counts.entries())
      .map(([kind, count]) => ({ kind, count }))
      .sort((a, b) => b.count - a.count)
  }, [spans])

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Span Kinds</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-4">
          <div className="h-[200px] w-[200px]">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={data} dataKey="count" nameKey="kind" cx="50%" cy="50%" innerRadius={50} outerRadius={80} paddingAngle={2}>
                  {data.map((entry) => (
                    <Cell key={entry.kind} fill={SPAN_KIND_COLORS[entry.kind as keyof typeof SPAN_KIND_COLORS] || '#6b7280'} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{ backgroundColor: '#18181b', border: '1px solid #27272a', borderRadius: '8px', fontSize: 12 }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="space-y-2">
            {data.map((entry) => (
              <div key={entry.kind} className="flex items-center gap-2 text-sm">
                <div
                  className="h-3 w-3 rounded-full"
                  style={{ backgroundColor: SPAN_KIND_COLORS[entry.kind as keyof typeof SPAN_KIND_COLORS] || '#6b7280' }}
                />
                <span className="text-muted-foreground">{entry.kind}</span>
                <span className="font-medium">{entry.count}</span>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
