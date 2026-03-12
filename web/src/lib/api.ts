const BASE = '/api/v1'

async function request<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

// Types
export interface Span {
  trace_id: string
  span_id: string
  parent_span_id?: string
  operation: string
  service: string
  kind: string
  start_us: number
  duration_us: number
  status: string
  tags?: Record<string, string>
  events?: string
}

export interface Trace {
  trace_id: string
  spans: Span[]
}

export interface ServiceInfo {
  name: string
  qps: number
  error_rate: number
  p99_latency_ms: number
}

export interface OperationInfo {
  operation: string
  call_count: number
  error_count: number
  avg_latency_ms: number
  max_latency_ms: number
}

export interface DepNode {
  id: string
  label: string
}

export interface DepEdge {
  source: string
  target: string
  call_count: number
  error_count: number
  avg_latency: number
}

export interface LatencyPoint {
  timestamp: string
  p50: number
  p90: number
  p99: number
  max: number
}

export interface ThroughputPoint {
  timestamp: string
  total: number
  errors: number
  error_rate: number
}

// API functions
export async function getTrace(traceId: string): Promise<Trace> {
  return request<Trace>(`/traces/${traceId}`)
}

export async function searchTraces(params: Record<string, string>): Promise<{ traces: Trace[]; total: number }> {
  const qs = new URLSearchParams(params).toString()
  return request(`/traces?${qs}`)
}

export async function getServices(lookback?: string): Promise<{ services: ServiceInfo[] }> {
  const params = lookback ? `?lookback=${lookback}` : ''
  return request(`/services${params}`)
}

export async function getOperations(service: string): Promise<{ operations: OperationInfo[] }> {
  return request(`/services/${encodeURIComponent(service)}/operations`)
}

export async function getDependencies(start?: string, end?: string): Promise<{ nodes: DepNode[]; edges: DepEdge[] }> {
  const params = new URLSearchParams()
  if (start) params.set('start', start)
  if (end) params.set('end', end)
  const qs = params.toString()
  return request(`/dependencies${qs ? '?' + qs : ''}`)
}

export async function getLatencyStats(params: Record<string, string>): Promise<{ data: LatencyPoint[] }> {
  const qs = new URLSearchParams(params).toString()
  return request(`/stats/latency?${qs}`)
}

export async function getThroughputStats(params: Record<string, string>): Promise<{ data: ThroughputPoint[] }> {
  const qs = new URLSearchParams(params).toString()
  return request(`/stats/throughput?${qs}`)
}
