import { Badge, Button, IconButton, Space } from '@ossrandom/design-system'
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

const NAV_HEIGHT = 48

export default function TopNav({ view, onNavigate, dashboard, stats, wsConnected }: TopNavProps) {
  const { theme, toggle } = useTheme()
  const errorRate = dashboard?.error_rate ?? 0

  return (
    <nav
      style={{
        height: NAV_HEIGHT,
        flexShrink: 0,
        background: 'var(--bg-1)',
        borderBottom: '1px solid var(--border-1)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 1rem',
        gap: '0.25rem',
        position: 'relative',
        zIndex: 10,
        overflowX: 'auto',
      }}
    >
      <a
        href="/"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          paddingRight: '1rem',
          marginRight: '0.25rem',
          borderRight: '1px solid var(--border-1)',
          textDecoration: 'none',
          color: 'inherit',
          flexShrink: 0,
        }}
      >
        <span style={{ color: 'var(--accent-fg)', fontSize: '1rem' }}>&#9670;</span>
        <span style={{ fontSize: '0.7rem', fontWeight: 700, letterSpacing: '0.12em' }}>OtelContext</span>
      </a>

      {navItems.map(({ key, label, icon: Icon }) => (
        <Button
          key={key}
          variant={view === key ? 'secondary' : 'ghost'}
          size="sm"
          iconLeft={<Icon size={13} />}
          onClick={() => onNavigate(key)}
        >
          {label}
        </Button>
      ))}

      <div style={{ marginLeft: 'auto' }}>
        <Space size="md" align="center">
          <StatPill label="Services" value={dashboard?.active_services?.toString() ?? '--'} tone="info" />
          <StatPill label="Traces" value={fmt(dashboard?.total_traces ?? 0)} />
          <StatPill label="Logs" value={fmt(dashboard?.total_logs ?? 0)} />
          <StatPill
            label="Error Rate"
            value={dashboard?.error_rate != null ? `${dashboard.error_rate.toFixed(1)}%` : '--%'}
            tone={errorRate > 5 ? 'danger' : 'neutral'}
          />
          <StatPill label="DB" value={stats?.db_size_mb != null ? `${stats.db_size_mb}MB` : '--'} />
          <Badge tone={wsConnected ? 'info' : 'danger'} size="sm">
            {wsConnected ? 'WS' : 'WS · off'}
          </Badge>
        </Space>
      </div>

      <IconButton
        icon={theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
        aria-label="Toggle theme"
        variant="ghost"
        size="sm"
        round
        onClick={toggle}
      />
    </nav>
  )
}

function StatPill({
  label,
  value,
  tone,
}: {
  label: string
  value: string
  tone?: 'info' | 'danger' | 'neutral'
}) {
  const valueColor =
    tone === 'danger' ? 'var(--brand-red-500)' :
    'var(--fg-1)'
  return (
    <span style={{ display: 'inline-flex', alignItems: 'baseline', gap: '0.3rem', fontSize: '0.7rem', color: 'var(--fg-3)', whiteSpace: 'nowrap' }}>
      {label}
      <b style={{ color: valueColor, fontWeight: 600, fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>{value}</b>
    </span>
  )
}
