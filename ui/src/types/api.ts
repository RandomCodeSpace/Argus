export interface ServiceError {
  service_name: string
  error_count: number
  total_count: number
  error_rate: number
}

export interface DashboardStats {
  total_traces: number
  total_logs: number
  total_errors: number
  avg_latency_ms: number
  error_rate: number
  active_services: number
  p99_latency: number
  top_failing_services: ServiceError[]
}

export interface Trace {
  id: number
  trace_id: string
  service_name: string
  duration: number
  duration_ms: number
  span_count: number
  operation: string
  status: string
  timestamp: string
  spans?: Span[]
  logs?: LogEntry[]
}

export interface Span {
  id: number
  trace_id: string
  span_id: string
  parent_span_id: string
  operation_name: string
  start_time: string
  end_time: string
  duration: number
  service_name: string
  attributes_json: string
}

export interface LogEntry {
  id: number
  trace_id: string
  span_id: string
  severity: string
  body: string
  service_name: string
  attributes_json: string
  ai_insight?: string
  timestamp: string
}

export interface TracesResponse {
  traces: Trace[]
  total: number
  limit: number
  offset: number
}

export interface LogsResponse {
  logs?: LogEntry[]
  items?: LogEntry[]
  total?: number
  limit?: number
  offset?: number
}

export interface MetricBucket {
  id: number
  name: string
  service_name: string
  time_bucket: string
  min: number
  max: number
  sum: number
  count: number
  attributes_json: string
}

export interface TrafficPoint {
  timestamp: string
  count: number
  error_count: number
}

export interface LatencyPoint {
  timestamp: string
  duration_us: number
}

export interface ServiceMapNode {
  name: string
  total_traces: number
  error_count: number
  avg_latency_ms: number
}

export interface ServiceMapEdge {
  source: string
  target: string
  call_count: number
  avg_latency_ms: number
  error_rate: number
}

export interface ServiceMapMetrics {
  nodes: ServiceMapNode[]
  edges: ServiceMapEdge[]
}

export interface SystemNode {
  id: string
  type: string
  health_score: number
  status: string
  metrics: {
    request_rate_rps: number
    error_rate: number
    avg_latency_ms: number
    p99_latency_ms: number
    span_count_1h: number
  }
  alerts: string[]
}

export interface SystemEdge {
  source: string
  target: string
  call_count: number
  avg_latency_ms: number
  error_rate: number
  status: string
}

export interface SystemGraphResponse {
  timestamp: string
  system: {
    total_services: number
    healthy: number
    degraded: number
    critical: number
    overall_health_score: number
    total_error_rate: number
    avg_latency_ms: number
    uptime_seconds: number
  }
  nodes: SystemNode[]
  edges: SystemEdge[]
}

export interface RepoStats {
  logCount?: number
  traceCount?: number
  serviceCount?: number
  errorCount?: number
  [key: string]: unknown
}

export interface MCPTool {
  name: string
  description: string
  inputSchema?: {
    properties?: Record<string, { type?: string; description?: string }>
    required?: string[]
  }
}
