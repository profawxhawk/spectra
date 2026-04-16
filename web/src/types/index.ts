export type SpanKind = 'llm' | 'tool' | 'agent' | 'retriever' | 'chain' | 'generic'
export type SpanStatus = 'ok' | 'error'
export type Operator = 'eq' | 'ne' | 'gt' | 'gte' | 'lt' | 'lte' | 'contains' | 'in'

export interface SpanMetrics {
  latency_ms: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost_usd: number
}

export interface SpanEvent {
  name: string
  timestamp: string
  data: Record<string, unknown>
}

export interface Span {
  span_id: string
  trace_id: string
  parent_id?: string
  name: string
  kind: SpanKind
  start_time: string
  end_time?: string
  status: SpanStatus
  input?: string
  output?: string
  metadata?: Record<string, string>
  attributes?: Record<string, unknown>
  events?: SpanEvent[]
  metrics?: SpanMetrics
}

export interface Filter {
  field: string
  operator: Operator
  value: unknown
}

export interface QueryRequest {
  filters?: Filter[]
  search?: string
  order_by?: string
  ascending?: boolean
  limit: number
  offset?: number
}

export interface QueryResult {
  spans: Span[]
  total_count: number
  has_more: boolean
}

export interface SearchRequest {
  query: string
  limit: number
}

export interface IngestRequest {
  spans: Span[]
}

export interface IngestResponse {
  ingested: number
}

export interface HealthResponse {
  status: string
}

export type TimeRange = '15m' | '1h' | '6h' | '24h' | '7d' | 'custom'

export const TIME_RANGE_LABELS: Record<TimeRange, string> = {
  '15m': 'Last 15 min',
  '1h': 'Last 1 hour',
  '6h': 'Last 6 hours',
  '24h': 'Last 24 hours',
  '7d': 'Last 7 days',
  custom: 'Custom',
}

export const SPAN_KIND_COLORS: Record<SpanKind, string> = {
  llm: '#3b82f6',
  tool: '#8b5cf6',
  agent: '#f59e0b',
  retriever: '#10b981',
  chain: '#ec4899',
  generic: '#6b7280',
}
