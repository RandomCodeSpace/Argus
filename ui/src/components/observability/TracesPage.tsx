import { useEffect, useRef, useState } from 'react'
import { FixedSizeList, type ListChildComponentProps } from 'react-window'
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

// Fixed row size matches the original card: padding 0.9rem + 3 text rows + gap.
// 112px accommodates the 0.65rem inter-row gap inside the slot.
const ITEM_SIZE = 112

interface RowData {
  traces: Trace[]
  selectedId: string | undefined
  onSelect: (traceId: string) => void
}

function TraceRow({ index, style, data }: ListChildComponentProps<RowData>) {
  const trace = data.traces[index]
  const isSelected = data.selectedId === trace.trace_id
  return (
    <div style={{ ...style, paddingBottom: '0.65rem', boxSizing: 'border-box' }}>
      <button
        onClick={() => data.onSelect(trace.trace_id)}
        className="card"
        style={{
          textAlign: 'left',
          background: isSelected ? 'var(--nav-active-bg)' : 'var(--bg-card)',
          borderColor: isSelected ? 'var(--color-accent)' : 'var(--border)',
          padding: '0.9rem',
          cursor: 'pointer',
          width: '100%',
          height: '100%',
          display: 'block',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
          <div style={{ fontWeight: 700, fontSize: '0.78rem' }}>{trace.service_name}</div>
          <span className={`badge ${trace.status.includes('ERROR') ? 'badge-red' : 'badge-green'}`}>{trace.status || 'OK'}</span>
        </div>
        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginBottom: '0.3rem', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{trace.operation || trace.trace_id}</div>
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          <span className="badge">{trace.span_count} spans</span>
          <span className="badge">{trace.duration_ms?.toFixed(1)} ms</span>
        </div>
      </button>
    </div>
  )
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
    <div className="traces-layout">
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.8rem', minHeight: 0, overflow: 'hidden' }}>
        <div style={{ flexShrink: 0 }}>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Traces</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Recent distributed requests</div>
        </div>
        {serviceFilter && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8, padding: '4px 8px', background: '#1e3a5f', borderRadius: 4, fontSize: 11, color: '#38bdf8' }}>
            <span>Filtered: {serviceFilter}</span>
            <button onClick={onClearFilter} style={{ background: 'none', border: 'none', color: '#38bdf8', cursor: 'pointer', fontSize: 12 }}>×</button>
          </div>
        )}
        {loading && <div style={{ color: 'var(--text-muted)' }}>Loading traces…</div>}
        {error && <div style={{ color: '#ef4444' }}>{error}</div>}
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
        </div>
      </div>
      <div className="traces-right-col">
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
            <div>
              <div style={{ fontSize: '0.85rem', fontWeight: 700 }}>{selected?.trace_id ?? 'No trace selected'}</div>
              <div style={{ fontSize: '0.73rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>{selected?.service_name}</div>
            </div>
            {selected && <span className={`badge ${selected.status.includes('ERROR') ? 'badge-red' : 'badge-green'}`}>{selected.status}</span>}
          </div>
        </div>
        <div className="card" style={{ overflow: 'auto' }}>
          <div style={{ fontSize: '0.8rem', fontWeight: 700, marginBottom: '0.8rem' }}>Span Waterfall</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.7rem' }}>
            {(selected?.spans ?? []).map((span) => (
              <div key={span.id} style={{ border: '1px solid var(--border)', borderRadius: 10, padding: '0.8rem', background: 'var(--bg-card)' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.75rem', marginBottom: '0.35rem' }}>
                  <div style={{ fontWeight: 700, fontSize: '0.78rem' }}>{span.operation_name}</div>
                  <span className="badge">{(span.duration / 1000).toFixed(1)} ms</span>
                </div>
                <div style={{ height: 8, borderRadius: 999, background: 'var(--bg-base)', overflow: 'hidden', marginBottom: '0.45rem' }}>
                  <div style={{ width: `${Math.min(100, Math.max(6, (span.duration / Math.max(selected?.duration || 1, 1)) * 100))}%`, height: '100%', background: 'linear-gradient(90deg, var(--color-accent), var(--color-accent-hover))' }} />
                </div>
                <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>{span.service_name}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
