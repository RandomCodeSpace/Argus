import { useState } from 'react'
import { Alert, Badge, Button, Modal, Space, Tabs, Textarea } from '@ossrandom/design-system'
import { Copy, SendHorizontal, Terminal } from 'lucide-react'
import type { MCPTool } from '@/types/api'
import { colorJSON } from '@/lib/utils'

interface Props {
  tool: MCPTool | null
  onClose: () => void
  onSend: (body: unknown) => Promise<{ data: unknown; status: number; ms: number }>
}

type RpcMethod = 'tools/call' | 'tools/list' | 'initialize' | 'ping' | 'resources/list' | 'custom'

function buildDefaultArgs(tool: MCPTool): Record<string, unknown> {
  const args: Record<string, unknown> = {}
  const props = tool.inputSchema?.properties || {}
  const req = tool.inputSchema?.required || []
  for (const [key, value] of Object.entries(props)) {
    args[key] = req.includes(key) ? (value.type === 'number' ? 0 : value.type === 'boolean' ? false : '') : null
  }
  return args
}

const templates: Record<Exclude<RpcMethod, 'custom'>, (name?: string, args?: Record<string, unknown>) => object> = {
  'tools/call': (name, args) => ({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name, arguments: args } }),
  'tools/list': () => ({ jsonrpc: '2.0', id: 1, method: 'tools/list' }),
  initialize: () => ({ jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2024-11-05', capabilities: {}, clientInfo: { name: 'otelcontext-ui', version: '1.0.0' } } }),
  ping: () => ({ jsonrpc: '2.0', id: 1, method: 'ping' }),
  'resources/list': () => ({ jsonrpc: '2.0', id: 1, method: 'resources/list' }),
}

const labelStyle: React.CSSProperties = {
  fontSize: '0.62rem',
  textTransform: 'uppercase',
  letterSpacing: '0.12em',
  color: 'var(--fg-4)',
  fontWeight: 700,
}

export default function RPCPopup({ tool, onClose, onSend }: Props) {
  const name = tool?.name ?? 'tool_name'
  const args = tool ? buildDefaultArgs(tool) : {}
  const [method, setMethod] = useState<RpcMethod>('tools/call')
  const [requestText, setRequestText] = useState(JSON.stringify(templates['tools/call'](name, args), null, 2))
  const [responseHTML, setResponseHTML] = useState('')
  const [timing, setTiming] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)

  const selectMethod = (next: RpcMethod) => {
    setMethod(next)
    if (next === 'custom') return
    const template = next === 'tools/call' ? templates[next](name, args) : templates[next]()
    setRequestText(JSON.stringify(template, null, 2))
  }

  const handleSend = async () => {
    let body: unknown
    try {
      body = JSON.parse(requestText)
    } catch (e) {
      setError(`Invalid JSON: ${String(e)}`)
      return
    }
    setSending(true)
    setError('')
    try {
      const { data, status, ms } = await onSend(body)
      setResponseHTML(colorJSON(data))
      setTiming(`${ms}ms · HTTP ${status}`)
    } catch (e) {
      setResponseHTML('')
      setError(String(e))
    } finally {
      setSending(false)
    }
  }

  const handleCopy = async () => {
    await navigator.clipboard.writeText(requestText)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1200)
  }

  const methods: { value: RpcMethod; label: string }[] = [
    { value: 'tools/call', label: 'call' },
    { value: 'tools/list', label: 'list' },
    { value: 'initialize', label: 'init' },
    { value: 'ping', label: 'ping' },
    { value: 'resources/list', label: 'resources' },
    { value: 'custom', label: 'custom' },
  ]

  const title = (
    <Space size="xs" align="center">
      <Terminal size={14} style={{ color: 'var(--accent-fg)' }} />
      <span style={{ fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>{name}</span>
      <Badge tone="subtle" size="sm">{method}</Badge>
    </Space>
  )

  return (
    <Modal
      open
      onClose={onClose}
      title={title}
      description={tool?.description || 'Manual JSON-RPC request builder'}
      size="lg"
    >
      <Tabs<RpcMethod>
        items={methods.map((item) => ({ key: item.value, label: item.label }))}
        value={method}
        variant="line"
        onChange={(key) => selectMethod(key)}
      />

      {error && <div style={{ marginTop: '0.75rem' }}><Alert severity="danger">{error}</Alert></div>}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0, marginTop: '0.75rem' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <label style={labelStyle}>Request</label>
            <Button variant="ghost" size="sm" iconLeft={<Copy size={11} />} onClick={handleCopy}>
              {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
          <Textarea
            value={requestText}
            onChange={(value) => setRequestText(value)}
            rows={14}
          />
          <Button variant="primary" block loading={sending} disabled={sending} iconLeft={<SendHorizontal size={12} />} onClick={handleSend}>
            {sending ? 'Sending' : 'Send'}
          </Button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', minHeight: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <label style={labelStyle}>Response</label>
            {timing && (
              <Badge tone="subtle" size="sm">
                {timing}
              </Badge>
            )}
          </div>
          <pre
            style={{
              flex: 1,
              minHeight: '16rem',
              overflow: 'auto',
              padding: '0.9rem',
              margin: 0,
              borderRadius: 'var(--radius-md)',
              background: 'var(--bg-3)',
              border: '1px solid var(--border-1)',
              color: 'var(--fg-2)',
              fontFamily: 'var(--font-mono, ui-monospace, monospace)',
              fontSize: '0.72rem',
              lineHeight: 1.55,
            }}
            dangerouslySetInnerHTML={{
              __html: responseHTML || '<span style="color:var(--fg-4)">—</span>',
            }}
          />
        </div>
      </div>
    </Modal>
  )
}
