import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { Circle } from 'lucide-react'
import { cn } from '@/lib/utils'

export function Header({ title }: { title: string }) {
  const health = useQuery({
    queryKey: ['health'],
    queryFn: () => api.health(),
    refetchInterval: 15_000,
    retry: 1,
  })

  const isHealthy = health.data?.status === 'ok'

  return (
    <header className="flex h-14 items-center justify-between border-b border-border px-6">
      <h1 className="text-lg font-semibold">{title}</h1>
      <div className="flex items-center gap-2 text-sm">
        <Circle
          className={cn(
            'h-2.5 w-2.5 fill-current',
            isHealthy ? 'text-emerald-500' : 'text-red-500'
          )}
        />
        <span className="text-muted-foreground">
          {health.isLoading ? 'Connecting...' : isHealthy ? 'Connected' : 'Disconnected'}
        </span>
      </div>
    </header>
  )
}
