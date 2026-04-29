import React, { useMemo, useState } from 'react'
import {
  Alert,
  Card,
  Drawer,
  Grid,
  Input,
  PageHeader,
  Space,
  Spin,
  Stat,
} from '@ossrandom/design-system'
import { ServiceMap as DSServiceMap } from '@ossrandom/design-system/charts'
import type { ServiceNode as DSNode, ServiceEdge as DSEdge } from '@ossrandom/design-system/charts'
import { Search } from 'lucide-react'
import ServiceSidePanel from './ServiceSidePanel'
import type { DashboardStats, RepoStats, SystemGraphResponse, SystemNode } from '../../types/api'
import { fmt } from '../../lib/utils'

interface ServicesViewProps {
  graph: SystemGraphResponse | null
  loading: boolean
  error: string | null
  dashboard: DashboardStats | null
  stats: RepoStats | null
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

const ServicesView: React.FC<ServicesViewProps> = ({
  graph,
  loading,
  error,
  dashboard,
  stats,
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
      .map((n) => ({ id: n.id, label: n.id, status: toNodeStatus(n.status) }))
  }, [nodes, search])

  const dsEdges = useMemo<DSEdge[]>(() => {
    if (dsNodes.length === 0) return []
    const allowed = new Set(dsNodes.map((n) => n.id))
    return edges
      .filter((e) => allowed.has(e.source) && allowed.has(e.target))
      .slice(0, 500)
      .map((e) => ({ source: e.source, target: e.target, status: toEdgeStatus(e.status) }))
  }, [edges, dsNodes])

  const totalServices = dashboard?.active_services ?? nodes.length
  const errorRate = dashboard?.error_rate ?? 0
  const totalTraces = dashboard?.total_traces ?? 0
  const totalLogs = dashboard?.total_logs ?? 0

  const handleNodeClick = (node: DSNode) => {
    const match = nodes.find((n) => n.id === node.id)
    setSelectedNode(match ?? null)
  }

  const handleSelectService = (id: string) => {
    const match = nodes.find((n) => n.id === id)
    if (match) setSelectedNode(match)
  }

  return (
    <Space direction="vertical" size="md">
      <PageHeader
        size="md"
        title="Service Topology"
        subtitle="Live dependency map · click a node for details"
      />

      <Grid columns={12} gap="md">
        <Grid.Col span={3}>
          <Card bordered padding="md" radius="md">
            <Stat label="Active services" value={totalServices} />
          </Card>
        </Grid.Col>
        <Grid.Col span={3}>
          <Card bordered padding="md" radius="md">
            <Stat
              label="Error rate"
              value={errorRate.toFixed(1)}
              unit="%"
              delta={errorRate > 0 ? { value: errorRate, direction: 'up', tone: errorRate > 5 ? 'bad' : 'neutral' } : undefined}
            />
          </Card>
        </Grid.Col>
        <Grid.Col span={3}>
          <Card bordered padding="md" radius="md">
            <Stat label="Traces" value={fmt(totalTraces)} />
          </Card>
        </Grid.Col>
        <Grid.Col span={3}>
          <Card bordered padding="md" radius="md">
            <Stat label="Logs" value={fmt(totalLogs)} unit={stats?.db_size_mb != null ? `· ${stats.db_size_mb} MB` : undefined} />
          </Card>
        </Grid.Col>
      </Grid>

      <Card
        bordered
        padding="md"
        radius="md"
        title="Service Map"
        extra={
          <Input
            value={search}
            onChange={(value) => setSearch(value)}
            placeholder="Filter services"
            size="sm"
            prefix={<Search size={12} />}
          />
        }
      >
        {loading && <Spin label="Loading service map" />}
        {error && (
          <Alert severity="danger" title="Service map failed to load">
            {error}
          </Alert>
        )}
        {!loading && !error && nodes.length === 0 && (
          <Alert severity="info">No services discovered yet.</Alert>
        )}
        {!loading && !error && dsNodes.length === 0 && nodes.length > 0 && (
          <Alert severity="info">No services match the filter.</Alert>
        )}
        {!loading && !error && dsNodes.length > 0 && (
          <DSServiceMap
            nodes={dsNodes}
            edges={dsEdges}
            layout="cose-bilkent"
            height={620}
            onNodeClick={handleNodeClick}
          />
        )}
      </Card>

      <Drawer
        open={selectedNode !== null}
        onClose={() => setSelectedNode(null)}
        placement="right"
        width={420}
        title={selectedNode ? <code>{selectedNode.id}</code> : undefined}
        description="Service detail · upstream, downstream, alerts"
      >
        {selectedNode && (
          <ServiceSidePanel
            node={selectedNode}
            edges={edges}
            onClose={() => setSelectedNode(null)}
            onSelectService={handleSelectService}
            onViewTraces={onNavigateToTraces}
            onViewLogs={onNavigateToLogs}
          />
        )}
      </Drawer>
    </Space>
  )
}

export default React.memo(ServicesView)
