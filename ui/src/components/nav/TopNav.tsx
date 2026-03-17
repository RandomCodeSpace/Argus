import { Activity, Database, LayoutDashboard, Moon, Radar, Search, Sun, Workflow } from 'lucide-react'
import type { DashboardStats } from '@/types/api'
import { fmt } from '@/lib/utils'

export type OtelView = 'overview' | 'traces' | 'logs' | 'services' | 'metrics' | 'archive' | 'mcp'

interface Props {
  currentView: OtelView
  onViewChange: (view: OtelView) => void
  stats: DashboardStats | null
  onThemeToggle: () => void
}

const items: { view: OtelView; label: string; icon: typeof Activity }[] = [
  { view: 'overview', label: 'Overview', icon: LayoutDashboard },
  { view: 'traces', label: 'Traces', icon: Activity },
  { view: 'logs', label: 'Logs', icon: Search },
  { view: 'services', label: 'Services', icon: Radar },
  { view: 'metrics', label: 'Metrics', icon: Database },
  { view: 'archive', label: 'Archive', icon: Database },
  { view: 'mcp', label: 'MCP Console', icon: Workflow },
]

export default function TopNav({ currentView, onViewChange, stats, onThemeToggle }: Props) {
  return (
    <nav className="top-nav">
      <a className="logo" href="/">
        <Activity size={17} style={{ color: 'var(--color-accent)', flexShrink: 0 }} />
        <span className="logo-mark">OTELCONTEXT</span>
        <span className="logo-ver">observability mesh</span>
      </a>

      {items.map(({ view, label, icon: Icon }) => (
        <button key={view} className={`nav-link${currentView === view ? ' active' : ''}`} onClick={() => onViewChange(view)}>
          <Icon size={13} /> {label}
        </button>
      ))}

      <div className="stats" style={{ marginLeft: 'auto' }}>
        <div className="stat"><span className="stat-val">{fmt(stats?.total_traces ?? 0)}</span><span className="stat-lbl">Traces</span></div>
        <div className="stat"><span className="stat-val">{fmt(stats?.total_logs ?? 0)}</span><span className="stat-lbl">Logs</span></div>
        <div className="stat"><span className="stat-val">{fmt(stats?.active_services ?? 0)}</span><span className="stat-lbl">Services</span></div>
      </div>

      <button className="theme-btn" onClick={onThemeToggle} title="Toggle theme" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Sun size={15} className="icon-sun" />
        <Moon size={15} className="icon-moon" />
      </button>
    </nav>
  )
}
