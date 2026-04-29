import React from 'react'
import { Alert, Badge, Button, IconButton, Space } from '@ossrandom/design-system'
import { ArrowRight, X } from 'lucide-react'
import type { SystemNode, SystemEdge } from '../../types/api'

interface ServiceSidePanelProps {
  node: SystemNode
  edges: SystemEdge[]
  onClose: () => void
  onSelectService: (id: string) => void
  onViewTraces: (service: string) => void
  onViewLogs: (service: string) => void
}

function statusTone(status: string): 'info' | 'warning' | 'danger' | 'neutral' {
  if (status === 'healthy') return 'info'
  if (status === 'degraded') return 'warning'
  if (status === 'critical' || status === 'failing') return 'danger'
  return 'neutral'
}

function healthBarColor(score: number): string {
  if (score < 0.4) return 'var(--brand-red-500)'
  if (score < 0.7) return 'var(--amber-500)'
  return 'var(--accent-fg)'
}

const labelStyle: React.CSSProperties = {
  fontSize: '0.6rem',
  textTransform: 'uppercase',
  letterSpacing: '0.14em',
  color: 'var(--fg-4)',
  fontWeight: 700,
}

const kpiCardStyle: React.CSSProperties = {
  background: 'var(--bg-2)',
  border: '1px solid var(--border-1)',
  borderRadius: 'var(--radius-md)',
  padding: '0.6rem 0.7rem',
}

const linkRowStyle: React.CSSProperties = {
  background: 'var(--bg-2)',
  border: '1px solid var(--border-1)',
  borderRadius: 'var(--radius-md)',
  padding: '0.4rem 0.6rem',
  marginBottom: '0.3rem',
  cursor: 'pointer',
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  fontSize: '0.72rem',
  color: 'var(--fg-2)',
  transition: 'border-color 120ms ease',
}

const ServiceSidePanel: React.FC<ServiceSidePanelProps> = ({
  node,
  edges,
  onClose,
  onSelectService,
  onViewTraces,
  onViewLogs,
}) => {
  const upstream = edges.filter((e) => e.target === node.id)
  const downstream = edges.filter((e) => e.source === node.id)
  const errorRatePercent = (node.metrics.error_rate * 100).toFixed(1)
  const isHighError = node.metrics.error_rate > 0.05

  return (
    <div style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <span
          style={{
            fontSize: '0.85rem',
            fontWeight: 700,
            color: 'var(--fg-1)',
            fontFamily: 'var(--font-mono, ui-monospace, monospace)',
            flex: 1,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {node.id}
        </span>
        <Badge tone={statusTone(node.status)} size="sm">{node.status}</Badge>
        <IconButton
          icon={<X size={13} />}
          aria-label="Close"
          variant="ghost"
          size="sm"
          onClick={onClose}
        />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
        <div style={kpiCardStyle}>
          <div style={labelStyle}>RPS</div>
          <div style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--fg-1)', marginTop: '0.2rem', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
            {Math.round(node.metrics.request_rate_rps)}
          </div>
        </div>
        <div style={kpiCardStyle}>
          <div style={labelStyle}>Error Rate</div>
          <div
            style={{
              fontSize: '1rem',
              fontWeight: 700,
              color: isHighError ? 'var(--brand-red-500)' : 'var(--fg-1)',
              marginTop: '0.2rem',
              fontFamily: 'var(--font-mono, ui-monospace, monospace)',
            }}
          >
            {errorRatePercent}%
          </div>
        </div>
        <div style={kpiCardStyle}>
          <div style={labelStyle}>Avg Latency</div>
          <div style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--fg-1)', marginTop: '0.2rem', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
            {node.metrics.avg_latency_ms}ms
          </div>
        </div>
        <div style={kpiCardStyle}>
          <div style={labelStyle}>P99</div>
          <div style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--fg-1)', marginTop: '0.2rem', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
            {node.metrics.p99_latency_ms}ms
          </div>
        </div>
      </div>

      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.3rem', alignItems: 'baseline' }}>
          <span style={labelStyle}>Health Score</span>
          <span style={{ fontSize: '0.72rem', color: 'var(--fg-1)', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
            {node.health_score.toFixed(2)}
          </span>
        </div>
        <div style={{ background: 'var(--bg-3)', borderRadius: 999, height: 4, overflow: 'hidden' }}>
          <div
            style={{
              width: `${node.health_score * 100}%`,
              height: '100%',
              background: healthBarColor(node.health_score),
              transition: 'width 200ms ease',
            }}
          />
        </div>
      </div>

      {upstream.length > 0 && (
        <div>
          <div style={{ ...labelStyle, marginBottom: '0.4rem' }}>Upstream</div>
          {upstream.map((edge) => (
            <div
              key={edge.source}
              role="button"
              tabIndex={0}
              onClick={() => onSelectService(edge.source)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onSelectService(edge.source)
                }
              }}
              style={linkRowStyle}
            >
              <span style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>{edge.source}</span>
              <span style={{ color: 'var(--fg-3)', fontSize: '0.68rem' }}>{edge.call_count} calls</span>
            </div>
          ))}
        </div>
      )}

      {downstream.length > 0 && (
        <div>
          <div style={{ ...labelStyle, marginBottom: '0.4rem' }}>Downstream</div>
          {downstream.map((edge) => (
            <div
              key={edge.target}
              role="button"
              tabIndex={0}
              onClick={() => onSelectService(edge.target)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onSelectService(edge.target)
                }
              }}
              style={linkRowStyle}
            >
              <span style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>{edge.target}</span>
              <span style={{ color: 'var(--fg-3)', fontSize: '0.68rem' }}>{edge.call_count} calls</span>
            </div>
          ))}
        </div>
      )}

      {node.alerts.length > 0 && (
        <div>
          <div style={{ ...labelStyle, marginBottom: '0.4rem' }}>Alerts</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
            {node.alerts.map((alert, i) => (
              <Alert key={i} severity="danger">
                {alert}
              </Alert>
            ))}
          </div>
        </div>
      )}

      <Space size="xs">
        <Button
          variant="secondary"
          size="sm"
          block
          iconRight={<ArrowRight size={11} />}
          onClick={() => onViewTraces(node.id)}
        >
          Traces
        </Button>
        <Button
          variant="secondary"
          size="sm"
          block
          iconRight={<ArrowRight size={11} />}
          onClick={() => onViewLogs(node.id)}
        >
          Logs
        </Button>
      </Space>
    </div>
  )
}

export default React.memo(ServiceSidePanel)
