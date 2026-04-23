import { useEffect, useMemo, useRef, useState } from 'react'
import { VariableSizeList, type ListChildComponentProps } from 'react-window'
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

// Row height estimate — base + per-line cost for wrapped bodies. Kept crude per spec
// (estimate first; measure only if visually off). Includes the 0.55rem inter-row gap.
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

function LogRow({ index, style, data }: ListChildComponentProps<RowData>) {
  const log = data.logs[index]
  return (
    <div style={{ ...style, paddingBottom: `${LOG_GAP}px`, boxSizing: 'border-box' }}>
      <div style={{ padding: '0.75rem 0.9rem', borderRadius: 10, border: '1px solid var(--border)', background: 'var(--bg-card)', height: '100%', boxSizing: 'border-box', overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
          <div style={{ display: 'flex', gap: '0.45rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <span className={`badge ${log.severity === 'ERROR' ? 'badge-red' : log.severity === 'WARN' ? 'badge-orange' : 'badge-blue'}`}>{log.severity}</span>
            <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{log.service_name}</span>
          </div>
          <span style={{ fontSize: '0.68rem', color: 'var(--text-dim)' }}>{new Date(log.timestamp).toLocaleTimeString()}</span>
        </div>
        <div style={{ fontSize: '0.74rem', color: 'var(--text-secondary)', lineHeight: 1.6, wordBreak: 'break-word' }}>{log.body}</div>
      </div>
    </div>
  )
}

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

  // Invalidate cached sizes whenever the list contents or width change.
  useEffect(() => {
    listRef.current?.resetAfterIndex(0)
  }, [filtered, streamSize.width])

  const getItemSize = (index: number): number => estimateLogHeight(filtered[index]?.body ?? '')

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'minmax(320px, 420px) minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem', minHeight: 0 }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Live Log Search</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Tail, filter, and query similar incidents</div>
        </div>
        {serviceFilter && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8, padding: '4px 8px', background: '#1e3a5f', borderRadius: 4, fontSize: 11, color: '#38bdf8' }}>
            <span>Filtered: {serviceFilter}</span>
            <button onClick={onClearFilter} style={{ background: 'none', border: 'none', color: '#38bdf8', cursor: 'pointer', fontSize: 12 }}>×</button>
          </div>
        )}
        <input className="search-input" style={{ paddingLeft: '10px' }} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find similar logs..." spellCheck={false} />
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          {['', 'INFO', 'WARN', 'ERROR'].map((value) => (
            <button key={value || 'all'} className={`mode-pill${severity === value ? ' active' : ''}`} onClick={() => setSeverity(value)}>{value || 'all'}</button>
          ))}
        </div>
        <button className="mc-send-btn" disabled={!query.trim()} onClick={() => onSimilar(query)}>Run Similarity Search</button>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem', overflow: 'auto' }}>
          {similar.map((log) => (
            <div key={`similar-${log.id}`} style={{ border: '1px solid var(--border)', borderRadius: 10, padding: '0.8rem', background: 'var(--bg-card)' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
                <div style={{ fontWeight: 700, fontSize: '0.76rem' }}>{log.service_name}</div>
                <span className={`badge ${log.severity === 'ERROR' ? 'badge-red' : log.severity === 'WARN' ? 'badge-orange' : 'badge-blue'}`}>{log.severity}</span>
              </div>
              <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', lineHeight: 1.5 }}>{log.body}</div>
            </div>
          ))}
        </div>
      </div>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.8rem', flexShrink: 0 }}>
          <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>Stream</div>
          {loading && <span className="badge">Loading…</span>}
        </div>
        {error && <div style={{ color: '#ef4444', marginBottom: '0.8rem', flexShrink: 0 }}>{error}</div>}
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
        </div>
      </div>
    </div>
  )
}
