import type { SystemGraphResponse } from '@/types/api'

interface Props {
  graph: SystemGraphResponse | null
  cache: string
  loading: boolean
  error: string | null
}

export default function ServicesPage({ graph, cache, loading, error }: Props) {
  const nodes = graph?.nodes ?? []
  const edges = graph?.edges ?? []
  const centerX = 360
  const centerY = 240
  const radius = 170

  const positioned = nodes.map((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(nodes.length, 1)
    return {
      ...node,
      x: centerX + Math.cos(angle) * radius,
      y: centerY + Math.sin(angle) * radius,
    }
  })

  const byId = new Map(positioned.map((node) => [node.id, node]))

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '320px minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>System Graph</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Current service topology</div>
        </div>
        <div style={{ display: 'grid', gap: '0.65rem' }}>
          <div className="badge">Cache {cache}</div>
          <div className="badge">{graph?.system.total_services ?? 0} services</div>
          <div className="badge">{graph?.system.healthy ?? 0} healthy</div>
          <div className="badge badge-orange">{graph?.system.degraded ?? 0} degraded</div>
          <div className="badge badge-red">{graph?.system.critical ?? 0} critical</div>
        </div>
        {loading && <div style={{ color: 'var(--text-muted)' }}>Loading graph…</div>}
        {error && <div style={{ color: '#ef4444' }}>{error}</div>}
      </div>
      <div className="card" style={{ minHeight: 0, overflow: 'hidden' }}>
        <div style={{ height: '100%', minHeight: 480, borderRadius: 12, border: '1px solid var(--border)', background: 'radial-gradient(circle at top, rgba(56,189,248,0.12), transparent 35%), linear-gradient(180deg, var(--bg-card), var(--bg-base))' }}>
          {positioned.length === 0 ? (
            <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)' }}>No service graph available.</div>
          ) : (
            <svg viewBox="0 0 720 480" style={{ width: '100%', height: '100%' }}>
              {edges.map((edge, index) => {
                const from = byId.get(edge.source)
                const to = byId.get(edge.target)
                if (!from || !to) return null
                return (
                  <g key={`${edge.source}-${edge.target}-${index}`}>
                    <line x1={from.x} y1={from.y} x2={to.x} y2={to.y} stroke="var(--border-hover)" strokeWidth={Math.max(1, edge.call_count / 200)} />
                    <text x={(from.x + to.x) / 2} y={(from.y + to.y) / 2 - 6} fontSize="10" fill="var(--text-muted)" textAnchor="middle">{edge.call_count}</text>
                  </g>
                )
              })}
              {positioned.map((node) => (
                <g key={node.id}>
                  <circle cx={node.x} cy={node.y} r={18} fill={node.status === 'critical' ? '#ef4444' : node.status === 'degraded' ? '#fb923c' : '#38bdf8'} stroke="var(--bg-base)" strokeWidth={3} />
                  <text x={node.x} y={node.y + 34} fill="var(--text-primary)" fontSize="11" textAnchor="middle">{node.id}</text>
                  <text x={node.x} y={node.y + 48} fill="var(--text-muted)" fontSize="9" textAnchor="middle">{(node.metrics.error_rate * 100).toFixed(1)}% err</text>
                </g>
              ))}
            </svg>
          )}
        </div>
      </div>
    </div>
  )
}
