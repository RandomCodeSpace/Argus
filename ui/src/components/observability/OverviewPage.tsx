import { AlertTriangle, Gauge, ListTree, Siren, Waves } from 'lucide-react'
import JsonViewer from '@/components/shared/JsonViewer'
import type { DashboardStats, RepoStats } from '@/types/api'
import { fmt } from '@/lib/utils'

interface Props {
  dashboard: DashboardStats | null
  stats: RepoStats | null
  loading: boolean
  error: string | null
}

const cards = [
  { key: 'total_traces', label: 'Traces', icon: ListTree },
  { key: 'total_logs', label: 'Logs', icon: Waves },
  { key: 'total_errors', label: 'Errors', icon: AlertTriangle },
  { key: 'active_services', label: 'Services', icon: Gauge },
  { key: 'p99_latency', label: 'P99 ms', icon: Siren },
] as const

export default function OverviewPage({ dashboard, stats, loading, error }: Props) {
  return (
    <div style={{ display: 'grid', gridTemplateRows: 'auto auto 1fr', gap: '1rem', minHeight: 0 }}>
      <section className="card" style={{ display: 'grid', gridTemplateColumns: '1.3fr 0.7fr', gap: '1rem', position: 'relative', overflow: 'hidden' }}>
        <div style={{ position: 'absolute', inset: 0, background: 'radial-gradient(circle at top left, rgba(56,189,248,0.14), transparent 40%), linear-gradient(180deg, transparent, rgba(255,255,255,0.02))', pointerEvents: 'none' }} />
        <div style={{ position: 'relative', zIndex: 1 }}>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.5rem' }}>Unified Console</div>
          <h1 style={{ fontSize: '2rem', lineHeight: 1.05, marginBottom: '0.75rem' }}>Observe traces, logs, service health, and MCP actions from a single operational cockpit.</h1>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.7, maxWidth: 620 }}>OtelContext now follows the same MCP Console language as the other tools, but tuned for observability workflows: incident scanning, trace drill-down, live log tailing, service topology, and archive lookup.</p>
        </div>
        <div style={{ position: 'relative', zIndex: 1, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
          {[
            ['Latency', `${dashboard?.avg_latency_ms?.toFixed(1) ?? '0'} ms avg`],
            ['Error Rate', `${dashboard?.error_rate?.toFixed(2) ?? '0'}%`],
            ['P99', `${fmt(dashboard?.p99_latency ?? 0)} ms`],
            ['Signals', `${fmt(stats?.traceCount as number ?? 0)} traces tracked`],
          ].map(([title, copy]) => (
            <div key={title} style={{ border: '1px solid var(--border)', borderRadius: 12, padding: '0.9rem', background: 'rgba(255,255,255,0.02)' }}>
              <div style={{ fontSize: '0.76rem', fontWeight: 700, marginBottom: '0.35rem' }}>{title}</div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', lineHeight: 1.5 }}>{copy}</div>
            </div>
          ))}
        </div>
      </section>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '0.9rem' }}>
        {cards.map(({ key, label, icon: Icon }) => (
          <div key={key} className="card" style={{ position: 'relative', overflow: 'hidden' }}>
            <div style={{ position: 'absolute', inset: '0 auto auto 0', width: 56, height: 56, borderRadius: '50%', background: 'radial-gradient(circle, var(--accent-glow), transparent 70%)', transform: 'translate(-22%, -22%)' }} />
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}><span className="badge">{label}</span><Icon size={15} style={{ color: 'var(--color-accent)' }} /></div>
            <div style={{ fontSize: '1.8rem', fontWeight: 700, lineHeight: 1 }}>{loading ? '…' : fmt(Number(dashboard?.[key] ?? 0))}</div>
          </div>
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0 }}>
        <JsonViewer title="Dashboard metrics" value={dashboard ?? {}} defaultOpen />
        <JsonViewer title="Repository stats" value={{ ...(stats ?? {}), error }} defaultOpen />
      </div>
    </div>
  )
}
