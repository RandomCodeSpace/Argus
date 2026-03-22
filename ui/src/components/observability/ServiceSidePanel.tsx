import React from 'react';
import type { SystemNode, SystemEdge } from '../../types/api';

interface ServiceSidePanelProps {
  node: SystemNode;
  edges: SystemEdge[];
  onClose: () => void;
  onSelectService: (id: string) => void;
  onViewTraces: (service: string) => void;
  onViewLogs: (service: string) => void;
}

const healthColors: Record<string, string> = {
  healthy: '#22c55e',
  degraded: '#fb923c',
  critical: '#ef4444',
};

function getHealthBarColor(score: number): string {
  if (score < 0.4) return '#ef4444';
  if (score < 0.7) return '#fb923c';
  return '#22c55e';
}

const ServiceSidePanel: React.FC<ServiceSidePanelProps> = ({
  node,
  edges,
  onClose,
  onSelectService,
  onViewTraces,
  onViewLogs,
}) => {
  const statusColor = healthColors[node.status] || '#888';
  const upstream = edges.filter((e) => e.target === node.id);
  const downstream = edges.filter((e) => e.source === node.id);
  const errorRatePercent = (node.metrics.error_rate * 100).toFixed(1);
  const isHighError = node.metrics.error_rate > 0.05;

  return (
    <div
      style={{
        background: '#0a0a0c',
        border: '1px solid #27272a',
        borderRadius: 8,
        padding: 16,
        width: 320,
        fontFamily: 'system-ui, sans-serif',
        color: '#fff',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16, position: 'relative' }}>
        <div
          style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            background: statusColor,
            marginRight: 8,
            flexShrink: 0,
          }}
        />
        <span style={{ fontSize: 13, fontWeight: 'bold', color: '#fff', marginRight: 8 }}>
          {node.id}
        </span>
        <span
          style={{
            fontSize: 9,
            textTransform: 'uppercase',
            background: statusColor,
            color: '#fff',
            padding: '2px 6px',
            borderRadius: 4,
            fontWeight: 600,
          }}
        >
          {node.status}
        </span>
        <button
          onClick={onClose}
          aria-label="Close"
          style={{
            position: 'absolute',
            right: 0,
            top: 0,
            background: 'none',
            border: 'none',
            color: '#888',
            cursor: 'pointer',
            fontSize: 16,
            lineHeight: 1,
            padding: 0,
          }}
        >
          X
        </button>
      </div>

      {/* KPI Grid */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 8,
          marginBottom: 16,
        }}
      >
        <div style={{ background: '#18181b', border: '1px solid #27272a', borderRadius: 6, padding: 10 }}>
          <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>RPS</div>
          <div style={{ fontSize: 16, fontWeight: 'bold' }}>
            {Math.round(node.metrics.request_rate_rps)}
          </div>
        </div>
        <div style={{ background: '#18181b', border: '1px solid #27272a', borderRadius: 6, padding: 10 }}>
          <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>Error Rate</div>
          <div style={{ fontSize: 16, fontWeight: 'bold', color: isHighError ? '#ef4444' : '#fff' }}>
            {errorRatePercent}%
          </div>
        </div>
        <div style={{ background: '#18181b', border: '1px solid #27272a', borderRadius: 6, padding: 10 }}>
          <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>Avg Latency</div>
          <div style={{ fontSize: 16, fontWeight: 'bold' }}>
            {node.metrics.avg_latency_ms}ms
          </div>
        </div>
        <div style={{ background: '#18181b', border: '1px solid #27272a', borderRadius: 6, padding: 10 }}>
          <div style={{ fontSize: 10, color: '#888', marginBottom: 4 }}>P99</div>
          <div style={{ fontSize: 16, fontWeight: 'bold' }}>
            {node.metrics.p99_latency_ms}ms
          </div>
        </div>
      </div>

      {/* Health Score Bar */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, marginBottom: 4 }}>
          <span style={{ color: '#888' }}>Health Score</span>
          <span style={{ color: '#fff' }}>{node.health_score.toFixed(2)}</span>
        </div>
        <div style={{ background: '#27272a', borderRadius: 2, height: 4 }}>
          <div
            style={{
              width: `${node.health_score * 100}%`,
              height: 4,
              borderRadius: 2,
              background: getHealthBarColor(node.health_score),
            }}
          />
        </div>
      </div>

      {/* Connected Services */}
      {upstream.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <div style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', marginBottom: 6 }}>
            Upstream
          </div>
          {upstream.map((edge) => (
            <div
              key={edge.source}
              onClick={() => onSelectService(edge.source)}
              style={{
                background: '#18181b',
                border: '1px solid #27272a',
                borderRadius: 6,
                padding: '6px 10px',
                marginBottom: 4,
                cursor: 'pointer',
                display: 'flex',
                justifyContent: 'space-between',
                fontSize: 12,
              }}
            >
              <span>{edge.source}</span>
              <span style={{ color: '#888' }}>{edge.call_count} calls</span>
            </div>
          ))}
        </div>
      )}
      {downstream.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <div style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', marginBottom: 6 }}>
            Downstream
          </div>
          {downstream.map((edge) => (
            <div
              key={edge.target}
              onClick={() => onSelectService(edge.target)}
              style={{
                background: '#18181b',
                border: '1px solid #27272a',
                borderRadius: 6,
                padding: '6px 10px',
                marginBottom: 4,
                cursor: 'pointer',
                display: 'flex',
                justifyContent: 'space-between',
                fontSize: 12,
              }}
            >
              <span>{edge.target}</span>
              <span style={{ color: '#888' }}>{edge.call_count} calls</span>
            </div>
          ))}
        </div>
      )}

      {/* Alerts */}
      {node.alerts.length > 0 && (
        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 10, color: '#888', textTransform: 'uppercase', marginBottom: 6 }}>
            Alerts
          </div>
          {node.alerts.map((alert, i) => (
            <div
              key={i}
              style={{
                background: '#1c0707',
                border: '1px solid #27272a',
                borderRadius: 6,
                padding: '6px 10px',
                marginBottom: 4,
                fontSize: 11,
                color: '#fca5a5',
              }}
            >
              {alert}
            </div>
          ))}
        </div>
      )}

      {/* Action Links */}
      <div style={{ display: 'flex', gap: 8 }}>
        <button
          onClick={() => onViewTraces(node.id)}
          style={{
            flex: 1,
            background: '#18181b',
            border: '1px solid #27272a',
            borderRadius: 6,
            color: '#fff',
            padding: '8px 0',
            cursor: 'pointer',
            fontSize: 12,
          }}
        >
          View Traces &rarr;
        </button>
        <button
          onClick={() => onViewLogs(node.id)}
          style={{
            flex: 1,
            background: '#18181b',
            border: '1px solid #27272a',
            borderRadius: 6,
            color: '#fff',
            padding: '8px 0',
            cursor: 'pointer',
            fontSize: 12,
          }}
        >
          View Logs &rarr;
        </button>
      </div>
    </div>
  );
};

export default React.memo(ServiceSidePanel);
