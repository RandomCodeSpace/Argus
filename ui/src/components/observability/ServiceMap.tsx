import React, { useMemo, useState } from 'react'
import { Alert, Input, Spin } from '@ossrandom/design-system'
import { ServiceMap as DSServiceMap } from '@ossrandom/design-system/charts'
import type { ServiceNode as DSNode, ServiceEdge as DSEdge } from '@ossrandom/design-system/charts'
import { Search } from 'lucide-react'
import ServiceSidePanel from './ServiceSidePanel'
import type { SystemGraphResponse, SystemNode } from '../../types/api'

interface ServiceMapProps {
  graph: SystemGraphResponse | null
  cache: string
  loading: boolean
  error: string | null
  onNavigateToTraces: (service: string) => void
  onNavigateToLogs: (service: string) => void
}

function toNodeStatus(status: string | undefined): DSNode['status'] {
  if (status === 'healthy' || status === 'degraded') return status
  if (status === 'critical' || status === 'failing') return 'failing'
  return 'unknown'
}

function toEdgeStatus(status: string | undefined): DSEdge['status'] {
  return status === 'critical' || status === 'failing' ? 'failing' : 'healthy'
}

const emptyState = (message: string, tone: 'muted' | 'danger' = 'muted') => (
  <div
    style={{
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: tone === 'danger' ? 'var(--brand-red-500)' : 'var(--fg-3)',
      fontSize: '0.78rem',
      padding: '2rem',
    }}
  >
    {message}
  </div>
)

const ServiceMap: React.FC<ServiceMapProps> = ({
  graph,
  cache: _cache,
  loading,
  error,
  onNavigateToTraces,
  onNavigateToLogs,
}) => {
  const [selectedNode, setSelectedNode] = useState<SystemNode | null>(null)
  const [search, setSearch] = useState('')

  const nodes = graph?.nodes ?? []
  const edges = graph?.edges ?? []

  const dsNodes = useMemo<DSNode[]>(() => {
    const q = search.trim().toLowerCase()
    return nodes
      .filter((n) => !q || n.id.toLowerCase().includes(q))
      .map((n) => ({
        id: n.id,
        label: n.id,
        status: toNodeStatus(n.status),
      }))
  }, [nodes, search])

  const dsEdges = useMemo<DSEdge[]>(() => {
    if (dsNodes.length === 0) return []
    const allowed = new Set(dsNodes.map((n) => n.id))
    return edges
      .filter((e) => allowed.has(e.source) && allowed.has(e.target))
      .slice(0, 500)
      .map((e) => ({
        source: e.source,
        target: e.target,
        status: toEdgeStatus(e.status),
      }))
  }, [edges, dsNodes])

  const handleNodeClick = (node: DSNode) => {
    const match = nodes.find((n) => n.id === node.id)
    setSelectedNode(match ?? null)
  }

  const handleSelectService = (id: string) => {
    const match = nodes.find((n) => n.id === id)
    if (match) setSelectedNode(match)
  }

  if (loading) {
    return (
      <div style={containerStyle}>
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Spin label="Loading service map" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div style={containerStyle}>
        <div style={{ padding: '1rem' }}>
          <Alert severity="danger" title="Service map failed to load">
            {error}
          </Alert>
        </div>
      </div>
    )
  }

  if (!graph || nodes.length === 0) {
    return <div style={containerStyle}>{emptyState('No services discovered yet.')}</div>
  }

  return (
    <div style={containerStyle}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.6rem',
          padding: '0.6rem 1rem',
          borderBottom: '1px solid var(--border-1)',
          background: 'var(--bg-1)',
        }}
      >
        <div style={{ width: 240 }}>
          <Input
            value={search}
            onChange={(value) => setSearch(value)}
            placeholder="Filter services"
            size="sm"
            prefix={<Search size={12} />}
          />
        </div>
        <span style={{ marginLeft: 'auto', fontSize: '0.7rem', color: 'var(--fg-3)' }}>
          {dsNodes.length} of {nodes.length} services · {dsEdges.length} calls
        </span>
      </div>

      <div style={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <div style={{ flex: 1, position: 'relative', minWidth: 0 }}>
          {dsNodes.length === 0 ? (
            emptyState('No services match the filter.')
          ) : (
            <DSServiceMap
              nodes={dsNodes}
              edges={dsEdges}
              layout="cose-bilkent"
              height={undefined}
              onNodeClick={handleNodeClick}
              style={{ width: '100%', height: '100%' }}
            />
          )}
        </div>

        {selectedNode && (
          <div
            style={{
              width: 360,
              flexShrink: 0,
              borderLeft: '1px solid var(--border-1)',
              background: 'var(--bg-1)',
              overflow: 'auto',
            }}
          >
            <ServiceSidePanel
              node={selectedNode}
              edges={edges}
              onClose={() => setSelectedNode(null)}
              onSelectService={handleSelectService}
              onViewTraces={onNavigateToTraces}
              onViewLogs={onNavigateToLogs}
            />
          </div>
        )}
      </div>
    </div>
  )
}

const containerStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  height: '100%',
  minHeight: 0,
  background: 'var(--bg-0)',
}

export default React.memo(ServiceMap)
