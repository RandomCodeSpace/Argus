import { Badge, Button, Card, Space } from '@ossrandom/design-system'
import { Play, Terminal } from 'lucide-react'
import type { MCPTool } from '@/types/api'

interface Props {
  tool: MCPTool
  index: number
  onCall: (index: number) => void
  onRPC: (index: number) => void
}

export default function ToolCard({ tool, index, onCall, onRPC }: Props) {
  const props = tool.inputSchema?.properties || {}
  const req = tool.inputSchema?.required || []
  const paramCount = Object.keys(props).length

  return (
    <Card bordered hoverable padding="md" radius="md">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem', height: '100%' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '0.5rem' }}>
          <span style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)', fontSize: '0.8rem', fontWeight: 700, color: 'var(--fg-1)' }}>
            {tool.name}
          </span>
          {paramCount > 0 && (
            <Badge tone="neutral" size="sm">
              {paramCount}p
            </Badge>
          )}
        </div>

        <p style={{ fontSize: '0.72rem', color: 'var(--fg-3)', lineHeight: 1.55, margin: 0, minHeight: '3.2em' }}>
          {tool.description || 'No description provided.'}
        </p>

        {paramCount > 0 && (
          <Space size="xs" wrap>
            {Object.entries(props).map(([key, value]) => (
              <Badge key={key} tone={req.includes(key) ? 'danger' : 'neutral'} size="sm">
                <span style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
                  {key}
                  <span style={{ opacity: 0.55, marginLeft: 2 }}>:{value.type ?? 'any'}</span>
                </span>
              </Badge>
            ))}
          </Space>
        )}

        <div style={{ marginTop: 'auto' }}>
          <Space size="xs">
            <Button variant="primary" size="sm" iconLeft={<Play size={10} />} onClick={() => onCall(index)}>
              Call
            </Button>
            <Button variant="ghost" size="sm" iconLeft={<Terminal size={10} />} onClick={() => onRPC(index)}>
              JSON-RPC
            </Button>
          </Space>
        </div>
      </div>
    </Card>
  )
}
