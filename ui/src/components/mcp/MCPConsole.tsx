import { useState } from 'react'
import { Alert, Badge, Button, Space } from '@ossrandom/design-system'
import { Plug, RefreshCw } from 'lucide-react'
import { useMCP } from '@/hooks/useMCP'
import type { MCPTool } from '@/types/api'
import ToolCard from './ToolCard'
import ToolCallModal from './ToolCallModal'
import RPCPopup from './RPCPopup'

const statusTone: Record<string, 'info' | 'warning' | 'danger' | 'neutral'> = {
  idle: 'neutral',
  connecting: 'warning',
  connected: 'info',
  error: 'danger',
}

export default function MCPConsole() {
  const { status, tools, error, call, connect, send } = useMCP()
  const [callTool, setCallTool] = useState<MCPTool | null>(null)
  const [rpcTool, setRpcTool] = useState<MCPTool | null>(null)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.75rem',
          padding: '0.7rem 1rem',
          borderBottom: '1px solid var(--border-1)',
          background: 'var(--bg-1)',
        }}
      >
        <Badge tone={statusTone[status]} size="sm">
          <span style={{ textTransform: 'capitalize' }}>{status}</span>
        </Badge>

        <Space size="xs" align="center">
          <Plug size={11} style={{ opacity: 0.5 }} />
          <code
            style={{
              fontFamily: 'var(--font-mono, ui-monospace, monospace)',
              padding: '0.15rem 0.4rem',
              borderRadius: 'var(--radius-sm)',
              background: 'var(--bg-2)',
              border: '1px solid var(--border-1)',
              color: 'var(--fg-2)',
              fontSize: '0.7rem',
            }}
          >
            {window.location.origin}/mcp
          </code>
        </Space>

        <Badge tone="subtle" size="sm">
          HTTP Streamable MCP · JSON-RPC 2.0
        </Badge>

        <div style={{ marginLeft: 'auto' }}>
          <Button
            variant="ghost"
            size="sm"
            iconLeft={<RefreshCw size={12} />}
            onClick={() => void connect()}
          >
            Reconnect
          </Button>
        </div>
      </div>

      <div
        style={{
          padding: '0.7rem 1rem',
          borderBottom: '1px solid var(--border-1)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <span
          style={{
            fontSize: '0.66rem',
            fontWeight: 700,
            textTransform: 'uppercase',
            letterSpacing: '0.14em',
            color: 'var(--fg-4)',
          }}
        >
          Available Tools
        </span>
        <span style={{ fontSize: '0.72rem', color: 'var(--fg-3)' }}>{tools.length} discovered</span>
      </div>

      <div
        style={{
          flex: 1,
          overflow: 'auto',
          padding: '1rem',
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          gap: '0.8rem',
          alignContent: 'start',
        }}
      >
        {status === 'error' && (
          <div style={{ gridColumn: '1 / -1' }}>
            <Alert severity="danger" title="Connection failed">
              {error || 'Could not reach the MCP endpoint.'}{' '}
              <code style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>MCP_ENABLED=true</code>
            </Alert>
          </div>
        )}
        {status === 'connected' &&
          tools.map((tool, index) => (
            <ToolCard
              key={tool.name}
              tool={tool}
              index={index}
              onCall={(next) => setCallTool(tools[next])}
              onRPC={(next) => setRpcTool(tools[next])}
            />
          ))}
      </div>

      {callTool && (
        <ToolCallModal
          tool={callTool}
          onClose={() => setCallTool(null)}
          onCall={async (name, args) =>
            (await call('tools/call', { name, arguments: args })).result ?? null
          }
        />
      )}
      {rpcTool && <RPCPopup tool={rpcTool} onClose={() => setRpcTool(null)} onSend={send} />}
    </div>
  )
}
