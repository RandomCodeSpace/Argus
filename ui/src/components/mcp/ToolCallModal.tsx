import { useState } from 'react'
import { Alert, Badge, Button, Modal, Textarea } from '@ossrandom/design-system'
import { Play } from 'lucide-react'
import type { MCPTool } from '@/types/api'
import { colorJSON } from '@/lib/utils'

interface Props {
  tool: MCPTool
  onClose: () => void
  onCall: (name: string, args: Record<string, unknown>) => Promise<unknown>
}

function buildDefaultArgs(tool: MCPTool): Record<string, unknown> {
  const args: Record<string, unknown> = {}
  const props = tool.inputSchema?.properties || {}
  const req = tool.inputSchema?.required || []
  for (const [key, value] of Object.entries(props)) {
    args[key] = req.includes(key) ? (value.type === 'number' ? 0 : value.type === 'boolean' ? false : '') : null
  }
  return args
}

const labelStyle: React.CSSProperties = {
  fontSize: '0.62rem',
  textTransform: 'uppercase',
  letterSpacing: '0.12em',
  color: 'var(--fg-4)',
  fontWeight: 700,
}

export default function ToolCallModal({ tool, onClose, onCall }: Props) {
  const [argsText, setArgsText] = useState(() => JSON.stringify(buildDefaultArgs(tool), null, 2))
  const [resultHTML, setResultHTML] = useState('')
  const [calling, setCalling] = useState(false)
  const [timing, setTiming] = useState('')
  const [error, setError] = useState('')

  const handleCall = async () => {
    let args: Record<string, unknown>
    try {
      args = JSON.parse(argsText || '{}')
    } catch (e) {
      setError(`Invalid JSON: ${String(e)}`)
      return
    }
    setCalling(true)
    setError('')
    const t0 = performance.now()
    try {
      const result = await onCall(tool.name, args)
      setResultHTML(colorJSON(result))
      setTiming(`${Math.round(performance.now() - t0)}ms`)
    } catch (e) {
      setResultHTML('')
      setError(String(e))
    } finally {
      setCalling(false)
    }
  }

  const title = (
    <span style={{ display: 'flex', alignItems: 'center', gap: '0.45rem' }}>
      <Play size={12} style={{ color: 'var(--accent-fg)' }} />
      <span>Call</span>
      <code style={{ background: 'transparent', padding: 0, color: 'var(--accent-fg)' }}>{tool.name}</code>
    </span>
  )

  return (
    <Modal open onClose={onClose} title={title} description={tool.description} size="lg">
      {error && <Alert severity="danger">{error}</Alert>}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', minHeight: 0, marginTop: error ? '0.75rem' : 0 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem' }}>
          <label style={labelStyle}>Arguments</label>
          <Textarea
            value={argsText}
            onChange={(value) => setArgsText(value)}
            rows={14}
          />
          <Button variant="primary" block loading={calling} disabled={calling} onClick={handleCall}>
            {calling ? 'Executing' : 'Execute Tool'}
          </Button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', minHeight: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <label style={labelStyle}>Result</label>
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
              __html: resultHTML || '<span style="color:var(--fg-4)">—</span>',
            }}
          />
        </div>
      </div>
    </Modal>
  )
}
