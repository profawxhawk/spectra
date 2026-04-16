import { useMemo } from 'react'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Span } from '@/types'

interface ErrorChartProps {
  spans: Span[]
}

export function ErrorChart({ spans }: ErrorChartProps) {
  const data = useMemo(() => {
    const ok = spans.filter((s) => s.status === 'ok').length
    const error = spans.filter((s) => s.status === 'error').length
    return [
      { status: 'OK', count: ok },
      { status: 'Error', count: error },
    ]
  }, [spans])

  const colors = ['#10b981', '#ef4444']

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Status Breakdown</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="h-[200px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data}>
              <XAxis dataKey="status" tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} width={40} />
              <Tooltip
                contentStyle={{ backgroundColor: '#18181b', border: '1px solid #27272a', borderRadius: '8px', fontSize: 12 }}
              />
              <Bar dataKey="count" radius={[4, 4, 0, 0]}>
                {data.map((_entry, index) => (
                  <Cell key={index} fill={colors[index]} fillOpacity={0.8} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}
