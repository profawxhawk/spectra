import type { QueryRequest, QueryResult, SearchRequest, HealthResponse } from '@/types'

const BASE_URL = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`API error ${res.status}: ${body}`)
  }
  return res.json()
}

export const api = {
  health(): Promise<HealthResponse> {
    return request('/healthz')
  },

  getTrace(traceId: string): Promise<QueryResult> {
    return request(`/v1/traces/${encodeURIComponent(traceId)}`)
  },

  search(req: SearchRequest): Promise<QueryResult> {
    return request('/v1/search', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  query(req: QueryRequest): Promise<QueryResult> {
    return request('/v1/query', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },
}
