import type { LatencyPoint, MetricBucket, ServiceMapMetrics, TrafficPoint } from '@/types/api'

interface Props {
  traffic: TrafficPoint[]
  heatmap: LatencyPoint[]
  serviceMap: ServiceMapMetrics | null
  buckets: MetricBucket[]
  loading: boolean
  error: string | null
}

function linePath(values: number[], width: number, height: number) {
  if (!values.length) return ''
  const max = Math.max(...values, 1)
  return values.map((value, index) => {
    const x = (index / Math.max(values.length - 1, 1)) * width
    const y = height - (value / max) * height
    return `${index === 0 ? 'M' : 'L'} ${x} ${y}`
  }).join(' ')
}

export default function MetricsPage({ traffic, heatmap, serviceMap, buckets, loading, error }: Props) {
  const trafficPath = linePath(traffic.map((point) => point.count), 520, 120)
  const latencyPath = linePath(heatmap.map((point) => point.duration_us / 1000), 520, 120)

  return (
    <div style={{ display: 'grid', gridTemplateRows: '1fr auto', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0 }}>
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>Traffic</div>
          {loading && <div style={{ color: 'var(--text-muted)' }}>Loading metrics…</div>}
          {error && <div style={{ color: '#ef4444' }}>{error}</div>}
          <svg viewBox="0 0 520 120" style={{ width: '100%', height: 140 }}>
            <path d={trafficPath} fill="none" stroke="var(--color-accent)" strokeWidth="3" />
          </svg>
          <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>{traffic.slice(-6).map((point, index) => <span key={index} className="badge">{point.count}</span>)}</div>
        </div>
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>Latency</div>
          <svg viewBox="0 0 520 120" style={{ width: '100%', height: 140 }}>
            <path d={latencyPath} fill="none" stroke="#fb923c" strokeWidth="3" />
          </svg>
          <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>{heatmap.slice(-6).map((point, index) => <span key={index} className="badge badge-orange">{Math.round(point.duration_us / 1000)} ms</span>)}</div>
        </div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0 }}>
        <div className="card" style={{ overflow: 'auto' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700, marginBottom: '0.8rem' }}>Service Map Metrics</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            {(serviceMap?.edges ?? []).map((edge, index) => (
              <div key={`${edge.source}-${edge.target}-${index}`} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', padding: '0.75rem', border: '1px solid var(--border)', borderRadius: 10, background: 'var(--bg-card)' }}>
                <div style={{ fontSize: '0.74rem' }}>{edge.source} → {edge.target}</div>
                <div style={{ display: 'flex', gap: '0.35rem', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                  <span className="badge">{edge.call_count} calls</span>
                  <span className="badge badge-blue">{edge.avg_latency_ms.toFixed(1)} ms</span>
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="card" style={{ overflow: 'auto' }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700, marginBottom: '0.8rem' }}>Metric Buckets</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem' }}>
            {buckets.slice(0, 20).map((bucket) => (
              <div key={bucket.id} style={{ padding: '0.75rem', border: '1px solid var(--border)', borderRadius: 10, background: 'var(--bg-card)' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.3rem' }}>
                  <div style={{ fontWeight: 700, fontSize: '0.75rem' }}>{bucket.name}</div>
                  <span className="badge">{bucket.service_name}</span>
                </div>
                <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{bucket.count} samples · min {bucket.min.toFixed(2)} · max {bucket.max.toFixed(2)}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
