import { useState } from 'react'

interface Props {
  results: string[]
  loading: boolean
  error: string | null
  onSearch: (type: 'logs' | 'traces' | 'metrics', query: string) => void
}

export default function ArchivePage({ results, loading, error, onSearch }: Props) {
  const [type, setType] = useState<'logs' | 'traces' | 'metrics'>('logs')
  const [query, setQuery] = useState('')

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '320px minmax(0, 1fr)', gap: '1rem', minHeight: 0, flex: 1 }}>
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
        <div>
          <div style={{ fontSize: '0.74rem', textTransform: 'uppercase', letterSpacing: '0.12em', color: 'var(--text-dim)', marginBottom: '0.35rem' }}>Cold Archive</div>
          <div style={{ fontSize: '0.95rem', fontWeight: 700 }}>Search retained telemetry</div>
        </div>
        <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
          {(['logs', 'traces', 'metrics'] as const).map((value) => <button key={value} className={`mode-pill${type === value ? ' active' : ''}`} onClick={() => setType(value)}>{value}</button>)}
        </div>
        <input className="search-input" style={{ paddingLeft: '10px' }} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search archive..." spellCheck={false} />
        <button className="mc-send-btn" disabled={!query.trim()} onClick={() => onSearch(type, query)}>{loading ? 'Searching…' : 'Search Archive'}</button>
        {error && <div style={{ color: '#ef4444' }}>{error}</div>}
      </div>
      <div className="card" style={{ minHeight: 0, overflow: 'auto' }}>
        <div style={{ fontSize: '0.85rem', fontWeight: 700, marginBottom: '0.8rem' }}>Results</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
          {results.length === 0 && <div style={{ color: 'var(--text-muted)' }}>No archive results yet.</div>}
          {results.map((result, index) => <pre key={index} className="mc-code" style={{ margin: 0, padding: '0.8rem', overflow: 'auto', whiteSpace: 'pre-wrap' }}>{result}</pre>)}
        </div>
      </div>
    </div>
  )
}
