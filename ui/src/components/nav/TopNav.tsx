import { Moon, Network, Radar, Search, Sun, Terminal } from 'lucide-react'
import type { DashboardStats, RepoStats } from '../../types/api'
import { fmt } from '../../lib/utils'
import { useTheme } from '../../hooks/useTheme'

export type OtelView = 'services' | 'traces' | 'logs' | 'mcp'

interface TopNavProps {
  view: OtelView
  onNavigate: (view: OtelView) => void
  dashboard: DashboardStats | null
  stats: RepoStats | null
  wsConnected: boolean
}

const navItems: { key: OtelView; label: string; icon: typeof Network }[] = [
  { key: 'services', label: 'Service Map', icon: Network },
  { key: 'traces', label: 'Traces', icon: Search },
  { key: 'logs', label: 'Logs', icon: Radar },
  { key: 'mcp', label: 'MCP', icon: Terminal },
]

export default function TopNav({ view, onNavigate, dashboard, stats, wsConnected }: TopNavProps) {
  const { theme, toggle } = useTheme()

  return (
    <nav className="top-nav">
      <a className="logo" href="/">
        <span style={{ color: 'var(--color-accent)', fontSize: '1rem', flexShrink: 0 }}>&#9670;</span>
        <span className="logo-mark">OtelContext</span>
      </a>

      {navItems.map(({ key, label, icon: Icon }) => (
        <button
          key={key}
          className={`nav-link${view === key ? ' active' : ''}`}
          onClick={() => onNavigate(key)}
        >
          <Icon size={13} /> {label}
        </button>
      ))}

      <div className="stats-bar" style={{ marginLeft: 'auto' }}>
        <span>
          Services{' '}
          <b className="stat-healthy">{dashboard?.active_services ?? '--'}</b>
        </span>
        <span>
          Traces{' '}
          <b>{fmt(dashboard?.total_traces ?? 0)}</b>
        </span>
        <span>
          Logs{' '}
          <b>{fmt(dashboard?.total_logs ?? 0)}</b>
        </span>
        <span>
          Error Rate{' '}
          <b className={(dashboard?.error_rate ?? 0) > 5 ? 'stat-error' : ''}>
            {dashboard?.error_rate != null ? `${dashboard.error_rate.toFixed(1)}%` : '--%'}
          </b>
        </span>
        <span>
          DB{' '}
          <b>{stats?.db_size_mb != null ? `${stats.db_size_mb}MB` : '--'}</b>
        </span>
        <span
          className={`ws-dot ${wsConnected ? 'connected' : 'disconnected'}`}
          title={wsConnected ? 'WebSocket connected' : 'WebSocket disconnected'}
        />
      </div>

      <button className="theme-btn" onClick={toggle} title="Toggle theme">
        {theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
      </button>
    </nav>
  )
}
