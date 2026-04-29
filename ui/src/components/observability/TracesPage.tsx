import { useEffect, useRef, useState } from 'react'
import { FixedSizeList, type ListChildComponentProps } from 'react-window'
import { Alert, Badge, Card, IconButton, Space, Spin } from '@ossrandom/design-system'
import { X } from 'lucide-react'
import type { Trace } from '@/types/api'

interface Props {
  traces: Trace[]
  selected: Trace | null
  loading: boolean
  error: string | null
  onSelect: (traceId: string) => void
  serviceFilter: string | null
  onClearFilter: () => void
}

const ITEM_SIZE = 112

interface RowData {
  traces: Trace[]
  selectedId: string | undefined
  onSelect: (traceId: string) => void
}

function statusTone(status: string): 'danger' | 'info' {
  return status.includes('ERROR') ? 'danger' : 'info'
}

function TraceRow({ index, style, data }: ListChildComponentProps<RowData>) {
  const trace = data.traces[index]
  const isSelected = data.selectedId === trace.trace_id
  return (
    <div style={{ ...style, paddingBottom: '0.65rem', boxSizing: 'border-box' }}>
      <button
        onClick={() => data.onSelect(trace.trace_id)}
        style={{
          textAlign: 'left',
          background: isSelected ? 'var(--accent-soft)' : 'var(--bg-2)',
          border: `1px solid ${isSelected ? 'var(--accent-fg)' : 'var(--border-1)'}`,
          borderRadius: 'var(--radius-md)',
          padding: '0.75rem 0.85rem',
          cursor: 'pointer',
          width: '100%',
          height: '100%',
          display: 'block',
          transition: 'background 120ms ease, border-color 120ms ease',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
          <div style={{ fontWeight: 700, fontSize: '0.78rem', color: 'var(--fg-1)' }}>{trace.service_name}</div>
          <Badge tone={statusTone(trace.status)} size="sm">{trace.status || 'OK'}</Badge>
        </div>
        <div
          style={{
            fontSize: '0.72rem',
            color: 'var(--fg-3)',
            marginBottom: '0.4rem',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            fontFamily: 'var(--font-mono, ui-monospace, monospace)',
          }}
        >
          {trace.operation || trace.trace_id}
        </div>
        <Space size="xs">
          <Badge tone="neutral" size="sm">{trace.span_count} spans</Badge>
          <Badge tone="subtle" size="sm">{trace.duration_ms?.toFixed(1)} ms</Badge>
        </Space>
      </button>
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

export default function TracesPage({ traces, selected, loading, error, onSelect, serviceFilter, onClearFilter }: Props) {
  const filtered = serviceFilter ? traces.filter((t) => t.service_name === serviceFilter) : traces

  const listContainerRef = useRef<HTMLDivElement | null>(null)
  const [size, setSize] = useState<{ width: number; height: number }>({ width: 0, height: 0 })

  useEffect(() => {
    const el = listContainerRef.current
    if (!el) return
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect
        setSize({ width, height })
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  return (
    <div
      style={{
        flex: 1,
        minHeight: 0,
        display: 'grid',
        gridTemplateColumns: 'minmax(320px, 380px) 1fr',
        gap: '1rem',
        padding: '1rem',
        overflow: 'hidden',
      }}
    >
      <Card bordered padding="md" radius="md" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem', minHeight: 0, overflow: 'hidden' }}>
        <div style={{ flexShrink: 0 }}>
          <div style={labelStyle}>Traces</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700, color: 'var(--fg-1)', marginTop: '0.2rem' }}>Recent distributed requests</div>
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
        {loading && <Spin label="Loading traces" />}
        {error && <Alert severity="danger">{error}</Alert>}
        <div ref={listContainerRef} style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
          {size.height > 0 && filtered.length > 0 && (
            <FixedSizeList<RowData>
              height={size.height}
              width={size.width}
              itemCount={filtered.length}
              itemSize={ITEM_SIZE}
              itemData={{ traces: filtered, selectedId: selected?.trace_id, onSelect }}
              overscanCount={6}
            >
              {TraceRow}
            </FixedSizeList>
          )}
          {!loading && filtered.length === 0 && (
            <div style={{ fontSize: '0.75rem', color: 'var(--fg-3)', padding: '1rem 0.25rem' }}>
              No traces yet.
            </div>
          )}
        </div>
      </Card>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', minHeight: 0 }}>
        <Card bordered padding="md" radius="md">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--fg-1)', fontFamily: 'var(--font-mono, ui-monospace, monospace)' }}>
                {selected?.trace_id ?? 'No trace selected'}
              </div>
              <div style={{ fontSize: '0.73rem', color: 'var(--fg-3)', marginTop: '0.2rem' }}>{selected?.service_name}</div>
            </div>
            {selected && <Badge tone={statusTone(selected.status)} size="sm">{selected.status}</Badge>}
          </div>
        </Card>

        <Card bordered padding="md" radius="md" style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
          <div style={{ ...labelStyle, marginBottom: '0.6rem' }}>Span Waterfall</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            {(selected?.spans ?? []).map((span) => {
              const totalUs = Math.max(selected?.duration || 1, 1)
              const widthPct = Math.min(100, Math.max(6, (span.duration / totalUs) * 100))
              return (
                <div
                  key={span.id}
                  style={{
                    border: '1px solid var(--border-1)',
                    borderRadius: 'var(--radius-md)',
                    padding: '0.7rem 0.8rem',
                    background: 'var(--bg-2)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
                    <div style={{ fontWeight: 700, fontSize: '0.78rem', color: 'var(--fg-1)' }}>{span.operation_name}</div>
                    <Badge tone="subtle" size="sm">{(span.duration / 1000).toFixed(1)} ms</Badge>
                  </div>
                  <div
                    style={{
                      height: 8,
                      borderRadius: 999,
                      background: 'var(--bg-3)',
                      overflow: 'hidden',
                      marginBottom: '0.4rem',
                    }}
                  >
                    <div
                      style={{
                        width: `${widthPct}%`,
                        height: '100%',
                        background: 'linear-gradient(90deg, var(--accent-fg), var(--accent-hover))',
                      }}
                    />
                  </div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--fg-3)' }}>{span.service_name}</div>
                </div>
              )
            })}
            {selected && (selected.spans ?? []).length === 0 && (
              <div style={{ fontSize: '0.75rem', color: 'var(--fg-3)' }}>No spans recorded for this trace.</div>
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}
