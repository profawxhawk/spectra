import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useTrace(traceId: string, enabled = true) {
  return useQuery({
    queryKey: ['trace', traceId],
    queryFn: () => api.getTrace(traceId),
    enabled: enabled && !!traceId,
  })
}
