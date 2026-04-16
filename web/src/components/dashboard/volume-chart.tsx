import { useMemo } from 'react'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Span } from '@/types'

interface VolumeChartProps {
  spans: Span[]
}

export function VolumeChart({ spans }: VolumeChartProps) {
  const data = useMemo(() => {
    const buckets = new Map<string, number>()
    spans.forEach((s) => {
      const d = new Date(s.start_time)
      const key = `${d.getHours().toString().padStart(2, '0')}:${(Math.floor(d.getMinutes() / 5) * 5).toString().padStart(2, '0')}`
      buckets.set(key, (buckets.get(key) || 0) + 1)
    })
    return Array.from(buckets.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([time, count]) => ({ time, count }))
  }, [spans])

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Span Volume</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="h-[200px]">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data}>
              <XAxis dataKey="time" tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} width={40} />
              <Tooltip
                contentStyle={{ backgroundColor: '#18181b', border: '1px solid #27272a', borderRadius: '8px', fontSize: 12 }}
                labelStyle={{ color: '#a1a1aa' }}
              />
              <Area type="monotone" dataKey="count" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.15} strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  )
}
