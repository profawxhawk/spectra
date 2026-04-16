import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { QueryRequest } from '@/types'

export function useSpans(req: QueryRequest, enabled = true) {
  return useQuery({
    queryKey: ['spans', req],
    queryFn: () => api.query(req),
    enabled,
    refetchInterval: 30_000,
  })
}

export function useRecentSpans(limit = 50) {
  return useQuery({
    queryKey: ['spans', 'recent', limit],
    queryFn: () =>
      api.query({
        filters: [{ field: 'kind', operator: 'in', value: 'llm,tool,agent,retriever,chain,generic' }],
        order_by: 'start_time',
        ascending: false,
        limit,
      }),
    refetchInterval: 10_000,
  })
}
