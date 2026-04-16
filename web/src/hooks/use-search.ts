import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

export function useSearch(query: string, limit = 50, enabled = true) {
  return useQuery({
    queryKey: ['search', query, limit],
    queryFn: () => api.search({ query, limit }),
    enabled: enabled && !!query,
  })
}
