import { Badge, Card, IconButton, Space, Tabs } from '@ossrandom/design-system'
import { Moon, Sun } from 'lucide-react'
import { useTheme } from '../../hooks/useTheme'

export type OtelView = 'services' | 'traces' | 'logs' | 'mcp'

interface TopNavProps {
  view: OtelView
  onNavigate: (view: OtelView) => void
  wsConnected: boolean
}

const tabs: { key: OtelView; label: string }[] = [
  { key: 'services', label: 'Service Map' },
  { key: 'traces', label: 'Traces' },
  { key: 'logs', label: 'Logs' },
  { key: 'mcp', label: 'MCP' },
]

export default function TopNav({ view, onNavigate, wsConnected }: TopNavProps) {
  const { theme, toggle } = useTheme()

  return (
    <Card bordered={false} padding="sm" radius="md">
      <Space justify="between" align="center">
        <Space size="md" align="center">
          <strong>OtelContext</strong>
          <Tabs<OtelView>
            items={tabs}
            value={view}
            variant="line"
            onChange={(key) => onNavigate(key)}
          />
        </Space>
        <Space size="sm" align="center">
          <Badge tone={wsConnected ? 'info' : 'danger'} size="sm">
            {wsConnected ? 'live' : 'offline'}
          </Badge>
          <IconButton
            icon={theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
            aria-label="Toggle theme"
            variant="ghost"
            size="sm"
            shape="circle"
            onClick={toggle}
          />
        </Space>
      </Space>
    </Card>
  )
}
