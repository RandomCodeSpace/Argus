import { useCallback, useRef, useState } from 'react'
import TopNav, { type OtelView } from '@/components/nav/TopNav'
import OverviewPage from '@/components/observability/OverviewPage'
import TracesPage from '@/components/observability/TracesPage'
import LogsPage from '@/components/observability/LogsPage'
import ServicesPage from '@/components/observability/ServicesPage'
import MetricsPage from '@/components/observability/MetricsPage'
import ArchivePage from '@/components/observability/ArchivePage'
import MCPConsole from '@/components/mcp/MCPConsole'
import { useTheme } from '@/hooks/useTheme'
import { useDashboard } from '@/hooks/useDashboard'
import { useTraces } from '@/hooks/useTraces'
import { useLogs } from '@/hooks/useLogs'
import { useMetrics } from '@/hooks/useMetrics'
import { useSystemGraph } from '@/hooks/useSystemGraph'
import { useArchive } from '@/hooks/useArchive'
import { useWebSocket } from '@/hooks/useWebSocket'
import type { LogEntry } from '@/types/api'

export default function App() {
  const { toggle } = useTheme()
  const { dashboard, stats, loading: dashboardLoading, error: dashboardError } = useDashboard()
  const { traces, selected, loading: tracesLoading, error: tracesError, selectTrace } = useTraces()
  const { logs, similar, loading: logsLoading, error: logsError, runSimilar, setLogs } = useLogs()
  const { traffic, heatmap, serviceMap, buckets, loading: metricsLoading, error: metricsError } = useMetrics()
  const { graph, cache, loading: graphLoading, error: graphError } = useSystemGraph()
  const { results, loading: archiveLoading, error: archiveError, search } = useArchive()
  const [view, setView] = useState<OtelView>('overview')

  const setLogsRef = useRef(setLogs)
  setLogsRef.current = setLogs
  const appendLogs = useCallback((incoming: LogEntry[]) => {
    setLogsRef.current((current) => [...incoming, ...current].slice(0, 200))
  }, [])

  useWebSocket(appendLogs)

  return (
    <>
      <TopNav currentView={view} onViewChange={setView} stats={dashboard} onThemeToggle={toggle} />
      <main className="main-content" style={{ padding: '1rem', gap: '1rem' }}>
        {view === 'overview' && <OverviewPage dashboard={dashboard} stats={stats} loading={dashboardLoading} error={dashboardError} />}
        {view === 'traces' && <TracesPage traces={traces} selected={selected} loading={tracesLoading} error={tracesError} onSelect={(traceId) => void selectTrace(traceId)} />}
        {view === 'logs' && <LogsPage logs={logs} similar={similar} loading={logsLoading} error={logsError} onSimilar={(query) => void runSimilar(query)} />}
        {view === 'services' && <ServicesPage graph={graph} cache={cache} loading={graphLoading} error={graphError} />}
        {view === 'metrics' && <MetricsPage traffic={traffic} heatmap={heatmap} serviceMap={serviceMap} buckets={buckets} loading={metricsLoading} error={metricsError} />}
        {view === 'archive' && <ArchivePage results={results} loading={archiveLoading} error={archiveError} onSearch={(type, query) => void search(type, query)} />}
        {view === 'mcp' && <MCPConsole />}
      </main>
      <footer className="status-bar">
        <div className="status-item"><span className="status-key">WS</span> live log stream</div>
        <div className="status-item"><span className="status-key">API</span> trace + metrics + archive</div>
        <div className="status-item"><span className="status-key">MCP</span> JSON-RPC tool surface</div>
      </footer>
    </>
  )
}
