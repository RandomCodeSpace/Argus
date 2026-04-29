import { useRef, useState } from 'react'
import { Modal, Tabs } from '@ossrandom/design-system'
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

export default function RPCPopup({ tool, onClose, onSend }: Props) {
  const name = tool?.name ?? 'tool_name'
  const args = tool ? buildDefaultArgs(tool) : {}
  const [method, setMethod] = useState<RpcMethod>('tools/call')
  const [requestText, setRequestText] = useState(JSON.stringify(templates['tools/call'](name, args), null, 2))
  const [responseHTML, setResponseHTML] = useState('')
  const [timing, setTiming] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const copyRef = useRef<HTMLButtonElement | null>(null)

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
    if (!copyRef.current) return
    const old = copyRef.current.innerHTML
    copyRef.current.textContent = 'Copied'
    window.setTimeout(() => {
      if (copyRef.current) copyRef.current.innerHTML = old
    }, 1200)
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
    <span style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
      <Terminal size={14} style={{ color: 'var(--color-accent)' }} />
      <span style={{ fontFamily: 'ui-monospace, monospace' }}>{name}</span>
      <span className="mc-badge">{method}</span>
    </span>
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
      {error && <div style={{ padding: '0.6rem 1.25rem', background: 'rgba(239,68,68,0.08)', borderBottom: '1px solid rgba(239,68,68,0.2)', color: '#ef4444', fontSize: '0.72rem', marginTop: '0.75rem' }}>{error}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', flex: 1, minHeight: 0, marginTop: '0.75rem' }}>
        <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0, borderRight: '1px solid var(--border)' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0.6rem 0.9rem', borderBottom: '1px solid var(--border)' }}>
            <span style={{ fontSize: '0.62rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', fontWeight: 700 }}>Request</span>
            <button ref={copyRef} className="mc-copy-btn" onClick={handleCopy}><Copy size={11} /> Copy</button>
          </div>
          <div style={{ padding: '0.75rem', flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
            <textarea className="mc-textarea" style={{ flex: 1, minHeight: 0 }} value={requestText} onChange={(event) => setRequestText(event.target.value)} spellCheck={false} />
          </div>
          <div style={{ padding: '0 0.75rem 0.75rem' }}>
            <button className="mc-send-btn" disabled={sending} onClick={handleSend} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.45rem' }}><SendHorizontal size={12} /> {sending ? 'Sending…' : 'Send'}</button>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0.6rem 0.9rem', borderBottom: '1px solid var(--border)' }}>
            <span style={{ fontSize: '0.62rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', fontWeight: 700 }}>Response</span>
            {timing && <span className="mc-badge">{timing}</span>}
          </div>
          <pre className="mc-code" style={{ margin: '0.75rem', flex: 1, minHeight: 0, overflow: 'auto', padding: '0.9rem' }} dangerouslySetInnerHTML={{ __html: responseHTML || '<span style="color:var(--text-dim)">—</span>' }} />
        </div>
      </div>
    </Modal>
  )
}
