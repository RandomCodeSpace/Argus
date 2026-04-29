import { useEffect, useMemo, useRef, useState } from 'react'
import { VariableSizeList, type ListChildComponentProps } from 'react-window'
import { Alert, Badge, Button, Card, IconButton, Input, Space } from '@ossrandom/design-system'
import { Search, X } from 'lucide-react'
import type { LogEntry } from '@/types/api'

interface Props {
  logs: LogEntry[]
  similar: LogEntry[]
  loading: boolean
  error: string | null
  onSimilar: (query: string) => void
  serviceFilter: string | null
  onClearFilter: () => void
}

const LOG_BASE_HEIGHT = 62
const LOG_LINE_HEIGHT = 19
const LOG_CHARS_PER_LINE = 80
const LOG_GAP = 9

function estimateLogHeight(body: string): number {
  const len = body ? body.length : 0
  const lines = Math.max(1, Math.ceil(len / LOG_CHARS_PER_LINE))
  return LOG_BASE_HEIGHT + lines * LOG_LINE_HEIGHT + LOG_GAP
}

interface RowData {
  logs: LogEntry[]
}

function severityTone(severity: string): 'danger' | 'warning' | 'info' {
  if (severity === 'ERROR') return 'danger'
  if (severity === 'WARN') return 'warning'
  return 'info'
}

function LogRow({ index, style, data }: ListChildComponentProps<RowData>) {
  const log = data.logs[index]
  return (
    <div style={{ ...style, paddingBottom: `${LOG_GAP}px`, boxSizing: 'border-box' }}>
      <div
        style={{
          padding: '0.7rem 0.85rem',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-1)',
          background: 'var(--bg-2)',
          height: '100%',
          boxSizing: 'border-box',
          overflow: 'hidden',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
          <Space size="xs" align="center">
            <Badge tone={severityTone(log.severity)} size="sm">{log.severity}</Badge>
            <span style={{ fontSize: '0.72rem', color: 'var(--fg-3)' }}>{log.service_name}</span>
          </Space>
          <span style={{ fontSize: '0.66rem', color: 'var(--fg-4)', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
            {new Date(log.timestamp).toLocaleTimeString()}
          </span>
        </div>
        <div
          style={{
            fontSize: '0.74rem',
            color: 'var(--fg-2)',
            lineHeight: 1.6,
            wordBreak: 'break-word',
            fontFamily: 'var(--font-mono, ui-monospace, monospace)',
          }}
        >
          {log.body}
        </div>
      </div>
    </div>
  )
}

const labelStyle: React.CSSProperties = {
  fontSize: '0.62rem',
  textTransform: 'uppercase',
  letterSpacing: '0.14em',
  color: 'var(--fg-4)',
  fontWeight: 700,
}

const SEVERITIES: { value: string; label: string }[] = [
  { value: '', label: 'all' },
  { value: 'INFO', label: 'info' },
  { value: 'WARN', label: 'warn' },
  { value: 'ERROR', label: 'error' },
]

export default function LogsPage({ logs, similar, loading, error, onSimilar, serviceFilter, onClearFilter }: Props) {
  const [query, setQuery] = useState('')
  const [severity, setSeverity] = useState('')

  const filtered = useMemo(() => {
    let result = logs
    if (serviceFilter) {
      result = result.filter((log) => log.service_name === serviceFilter)
    }
    if (severity) {
      result = result.filter((log) => log.severity === severity)
    }
    return result
  }, [logs, severity, serviceFilter])

  const streamContainerRef = useRef<HTMLDivElement | null>(null)
  const [streamSize, setStreamSize] = useState<{ width: number; height: number }>({ width: 0, height: 0 })
  const listRef = useRef<VariableSizeList<RowData> | null>(null)

  useEffect(() => {
    const el = streamContainerRef.current
    if (!el) return
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect
        setStreamSize({ width, height })
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    listRef.current?.resetAfterIndex(0)
  }, [filtered, streamSize.width])

  const getItemSize = (index: number): number => estimateLogHeight(filtered[index]?.body ?? '')

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'minmax(320px, 420px) minmax(0, 1fr)',
        gap: '1rem',
        minHeight: 0,
        flex: 1,
        padding: '1rem',
        overflow: 'hidden',
      }}
    >
      <Card bordered padding="md" radius="md" style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem', minHeight: 0 }}>
        <div>
          <div style={labelStyle}>Live Log Search</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700, color: 'var(--fg-1)', marginTop: '0.2rem' }}>
            Tail, filter, and query similar incidents
          </div>
        </div>
        {serviceFilter && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '0.4rem',
              padding: '0.3rem 0.5rem',
              background: 'var(--accent-soft)',
              border: '1px solid var(--accent-fg)',
              borderRadius: 'var(--radius-sm)',
              fontSize: '0.7rem',
              color: 'var(--accent-fg)',
            }}
          >
            <span>Filtered: {serviceFilter}</span>
            <IconButton
              icon={<X size={11} />}
              aria-label="Clear filter"
              variant="ghost"
              size="xs"
              onClick={onClearFilter}
            />
          </div>
        )}
        <Input
          value={query}
          onChange={(value) => setQuery(value)}
          placeholder="Find similar logs"
          size="sm"
          prefix={<Search size={12} />}
        />
        <Space size="xs" wrap>
          {SEVERITIES.map((item) => (
            <Button
              key={item.value || 'all'}
              variant={severity === item.value ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => setSeverity(item.value)}
            >
              {item.label}
            </Button>
          ))}
        </Space>
        <Button variant="primary" block disabled={!query.trim()} onClick={() => onSimilar(query)}>
          Run Similarity Search
        </Button>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.55rem', overflow: 'auto', minHeight: 0 }}>
          {similar.map((log) => (
            <div
              key={`similar-${log.id}`}
              style={{
                border: '1px solid var(--border-1)',
                borderRadius: 'var(--radius-md)',
                padding: '0.7rem 0.8rem',
                background: 'var(--bg-2)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
                <div style={{ fontWeight: 700, fontSize: '0.76rem', color: 'var(--fg-1)' }}>{log.service_name}</div>
                <Badge tone={severityTone(log.severity)} size="sm">{log.severity}</Badge>
              </div>
              <div style={{ fontSize: '0.72rem', color: 'var(--fg-2)', lineHeight: 1.5, fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
                {log.body}
              </div>
            </div>
          ))}
          {similar.length === 0 && query.trim() && (
            <div style={{ fontSize: '0.72rem', color: 'var(--fg-3)' }}>No similar logs yet — run search.</div>
          )}
        </div>
      </Card>

      <Card bordered padding="md" radius="md" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.8rem', flexShrink: 0 }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--fg-1)' }}>Stream</div>
          {loading && <Badge tone="subtle" size="sm">Loading</Badge>}
        </div>
        {error && (
          <div style={{ marginBottom: '0.7rem', flexShrink: 0 }}>
            <Alert severity="danger">{error}</Alert>
          </div>
        )}
        <div ref={streamContainerRef} style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          {streamSize.height > 0 && filtered.length > 0 && (
            <VariableSizeList<RowData>
              ref={listRef}
              height={streamSize.height}
              width={streamSize.width}
              itemCount={filtered.length}
              itemSize={getItemSize}
              estimatedItemSize={90}
              itemData={{ logs: filtered }}
              overscanCount={6}
            >
              {LogRow}
            </VariableSizeList>
          )}
          {!loading && filtered.length === 0 && (
            <div style={{ fontSize: '0.75rem', color: 'var(--fg-3)' }}>No logs yet.</div>
          )}
        </div>
      </Card>
    </div>
  )
}
