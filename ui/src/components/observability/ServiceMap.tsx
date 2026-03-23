import React, { useState, useRef, useMemo, useEffect, useCallback } from 'react';
import * as echarts from 'echarts';
import EChart from '../shared/EChart';
import ServiceSidePanel from './ServiceSidePanel';
import { computeLayout, defaultEdgeThreshold } from '../../lib/tierLayout';
import type { SystemGraphResponse, SystemNode } from '../../types/api';
import type { EChartsOption } from 'echarts';

/* ── Props ──────────────────────────────────────────────────────────── */

interface ServiceMapProps {
  graph: SystemGraphResponse | null;
  cache: string;
  loading: boolean;
  error: string | null;
  onNavigateToTraces: (service: string) => void;
  onNavigateToLogs: (service: string) => void;
}

/* ── Status colours ─────────────────────────────────────────────────── */

const STATUS_COLORS: Record<string, { bg: string; border: string; dot: string }> = {
  healthy:  { bg: '#0f2618', border: '#166534', dot: '#22c55e' },
  degraded: { bg: '#1a1207', border: '#854d0e', dot: '#fb923c' },
  critical: { bg: '#1c0707', border: '#991b1b', dot: '#ef4444' },
};

function bgColorForStatus(status: string): string {
  return STATUS_COLORS[status]?.bg ?? '#18181b';
}

function borderColorForStatus(status: string): string {
  return STATUS_COLORS[status]?.border ?? '#27272a';
}

function dotColorForStatus(status: string): string {
  return STATUS_COLORS[status]?.dot ?? '#888';
}

function edgeColorForStatus(status: string): string {
  if (status === 'critical') return '#ef4444';
  if (status === 'degraded') return '#fb923c';
  return '#3f3f46';
}

/* ── Tier labels ────────────────────────────────────────────────────── */

const TIER_NAMES = ['GATEWAY', 'API LAYER', 'SERVICES', 'DATA'];

/* ── Component ──────────────────────────────────────────────────────── */

const ServiceMap: React.FC<ServiceMapProps> = ({
  graph,
  cache: _cache,
  loading,
  error,
  onNavigateToTraces,
  onNavigateToLogs,
}) => {
  const [selectedNode, setSelectedNode] = useState<SystemNode | null>(null);
  const [edgeThreshold, setEdgeThreshold] = useState<number>(10);
  const [searchQuery, setSearchQuery] = useState('');

  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);

  // Debounce search input
  const handleSearchChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setSearchQuery(val);
    if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(() => setDebouncedSearch(val), 300);
  }, []);

  useEffect(() => {
    return () => {
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    };
  }, []);

  // Compute default threshold when data changes
  useEffect(() => {
    if (graph?.edges) {
      setEdgeThreshold(defaultEdgeThreshold(graph.edges));
    }
  }, [graph]);

  const nodes = graph?.nodes ?? [];
  const edges = graph?.edges ?? [];
  const dense = nodes.length >= 70;

  const maxCallCount = useMemo(
    () => Math.max(1, ...edges.map((e) => e.call_count)),
    [edges],
  );

  const filteredEdges = useMemo(
    () => edges.filter((e) => e.call_count >= edgeThreshold).slice(0, 500),
    [edges, edgeThreshold],
  );

  // Layout positions
  const positions = useMemo(() => {
    if (nodes.length === 0) return new Map<string, { x: number; y: number; tier: number }>();
    return computeLayout(
      nodes.map((n) => ({ id: n.id, span_count: n.metrics.span_count_1h })),
      edges.map((e) => ({ source: e.source, target: e.target })),
      { width: 900, height: 600 },
    );
  }, [nodes, edges]);

  // Tier y-positions for graphic labels
  const tierYPositions = useMemo(() => {
    const tierYs = new Map<number, number>();
    for (const pos of positions.values()) {
      if (!tierYs.has(pos.tier) || pos.y < tierYs.get(pos.tier)!) {
        tierYs.set(pos.tier, pos.y);
      }
    }
    return tierYs;
  }, [positions]);

  // Format node label
  const formatNodeLabel = useCallback(
    (node: SystemNode, isDense: boolean) => {
      const name = node.id.length > 16 ? node.id.slice(0, 15) + '\u2026' : node.id;
      const rps = Math.round(node.metrics.request_rate_rps);
      const errPct = (node.metrics.error_rate * 100).toFixed(1);
      if (isDense) {
        return `{dot|●} {name|${name}}`;
      }
      return `{dot|●} {name|${name}}\n{metric|${rps} rps  ${errPct}% err}`;
    },
    [],
  );

  // Build ECharts option
  const chartOption = useMemo((): EChartsOption => {
    const seriesData = nodes.map((node) => {
      const pos = positions.get(node.id) ?? { x: 0, y: 0 };
      const isMatch =
        !debouncedSearch ||
        node.id.toLowerCase().includes(debouncedSearch.toLowerCase());

      return {
        name: node.id,
        x: pos.x,
        y: pos.y,
        symbol: 'roundRect',
        symbolSize: dense ? [90, 36] : [120, 46],
        itemStyle: {
          color: bgColorForStatus(node.status),
          borderColor: borderColorForStatus(node.status),
          borderWidth: 1,
          shadowColor:
            node.status !== 'healthy'
              ? borderColorForStatus(node.status)
              : 'transparent',
          shadowBlur: node.status !== 'healthy' ? 8 : 0,
          opacity: isMatch ? 1 : 0.2,
        },
        label: {
          show: true,
          formatter: () => formatNodeLabel(node, dense),
          rich: {
            dot: {
              fontSize: dense ? 6 : 8,
              color: dotColorForStatus(node.status),
            },
            name: {
              fontSize: dense ? 9 : 11,
              fontWeight: 'bold' as const,
              color: '#e4e4e7',
            },
            metric: {
              fontSize: dense ? 7 : 9,
              color: '#71717a',
              padding: [2, 0, 0, 0],
            },
          },
        },
      };
    });

    const seriesLinks = filteredEdges.map((edge) => ({
      source: edge.source,
      target: edge.target,
      lineStyle: {
        width: Math.max(1, Math.min(4, Math.log10(edge.call_count + 1))),
        color: edgeColorForStatus(edge.status),
        opacity: 0.3 + 0.5 * (edge.call_count / maxCallCount),
        curveness: 0.1,
      },
      symbol: ['none', 'arrow'] as [string, string],
      symbolSize: 6,
    }));

    // Tier label graphic elements
    const tierLabels: echarts.GraphicComponentOption[] = [];
    for (const [tier, y] of tierYPositions) {
      if (tier >= 0 && tier < TIER_NAMES.length) {
        tierLabels.push({
          type: 'text',
          left: 15,
          top: y - 12,
          style: {
            text: TIER_NAMES[tier],
            fontSize: 9,
            fill: '#3f3f46',
            fontWeight: 'bold',
          },
          silent: true,
        });
      }
    }

    return {
      tooltip: {
        trigger: 'item',
        backgroundColor: '#18181b',
        borderColor: '#27272a',
        textStyle: { color: '#e4e4e7', fontSize: 11 },
        formatter: (params: unknown) => {
          const p = params as { dataType?: string; name?: string; data?: { source?: string; target?: string } };
          if (p.dataType === 'node') {
            const node = nodes.find((n) => n.id === p.name);
            if (!node) return '';
            return [
              `<strong>${node.id}</strong>`,
              `Status: ${node.status}`,
              `RPS: ${Math.round(node.metrics.request_rate_rps)}`,
              `Error: ${(node.metrics.error_rate * 100).toFixed(1)}%`,
              `Avg Latency: ${node.metrics.avg_latency_ms}ms`,
            ].join('<br/>');
          }
          if (p.dataType === 'edge' && p.data) {
            const edge = edges.find(
              (e) => e.source === p.data!.source && e.target === p.data!.target,
            );
            if (!edge) return '';
            return [
              `<strong>${edge.source} → ${edge.target}</strong>`,
              `Calls: ${edge.call_count}`,
              `Avg Latency: ${edge.avg_latency_ms}ms`,
              `Error: ${(edge.error_rate * 100).toFixed(1)}%`,
            ].join('<br/>');
          }
          return '';
        },
      },
      graphic: tierLabels,
      series: [
        {
          type: 'graph',
          layout: 'force',
          force: {
            repulsion: dense ? 200 : 350,
            gravity: 0.08,
            edgeLength: dense ? [80, 160] : [120, 250],
            friction: 0.6,
            layoutAnimation: true,
          },
          roam: true,
          data: seriesData,
          links: seriesLinks,
          emphasis: {
            focus: 'adjacency',
          },
          lineStyle: {
            color: 'source',
          },
        },
      ],
    };
  }, [nodes, edges, filteredEdges, positions, tierYPositions, dense, debouncedSearch, maxCallCount, formatNodeLabel]);

  // ECharts event handlers
  const onEvents = useMemo(
    () => ({
      click: (params: unknown) => {
        const p = params as { dataType?: string; name?: string };
        if (p.dataType === 'node' && p.name) {
          const node = nodes.find((n) => n.id === p.name);
          setSelectedNode(node ?? null);
        } else {
          setSelectedNode(null);
        }
      },
    }),
    [nodes],
  );

  // Zoom controls
  const handleZoomIn = useCallback(() => {
    chartInstanceRef.current?.dispatchAction({
      type: 'graphRoam',
      zoom: 1.3,
    });
  }, []);

  const handleZoomOut = useCallback(() => {
    chartInstanceRef.current?.dispatchAction({
      type: 'graphRoam',
      zoom: 0.7,
    });
  }, []);

  const handleFit = useCallback(() => {
    chartInstanceRef.current?.dispatchAction({ type: 'restore' });
  }, []);

  // Side panel: select a connected service
  const handleSelectService = useCallback(
    (id: string) => {
      const node = nodes.find((n) => n.id === id);
      if (node) setSelectedNode(node);
    },
    [nodes],
  );

  /* ── Loading / error states ────────────────────────────────────────── */

  if (loading) {
    return (
      <div className="service-map-container">
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#71717a', fontSize: 13 }}>
          Loading service map...
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="service-map-container">
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#ef4444', fontSize: 13 }}>
          {error}
        </div>
      </div>
    );
  }

  if (!graph || nodes.length === 0) {
    return (
      <div className="service-map-container">
        <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#71717a', fontSize: 13 }}>
          No services discovered yet.
        </div>
      </div>
    );
  }

  /* ── Render ────────────────────────────────────────────────────────── */

  return (
    <div className="service-map-container">
      {/* Toolbar */}
      <div className="service-map-toolbar">
        <div className="search-wrap" style={{ maxWidth: 220 }}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="11" cy="11" r="8" />
            <path d="M21 21l-4.35-4.35" />
          </svg>
          <input
            className="search-input"
            type="text"
            placeholder="Filter services..."
            value={searchQuery}
            onChange={handleSearchChange}
            style={{ paddingLeft: 28, fontSize: 11 }}
          />
        </div>

        <div className="edge-slider">
          <span>Edges &ge;</span>
          <input
            type="range"
            min={1}
            max={maxCallCount}
            value={edgeThreshold}
            onChange={(e) => setEdgeThreshold(Number(e.target.value))}
          />
          <span>{edgeThreshold}</span>
        </div>

        <div style={{ flex: 1 }} />

        <div className="zoom-controls">
          <button className="zoom-btn" onClick={handleZoomIn} title="Zoom in">+</button>
          <button className="zoom-btn" onClick={handleZoomOut} title="Zoom out">&minus;</button>
          <button className="zoom-btn" onClick={handleFit} title="Fit to view">&#8859;</button>
        </div>
      </div>

      {/* Body: chart + optional side panel */}
      <div className="service-map-body">
        <div className="service-map-canvas">
          <EChart
            option={chartOption}
            style={{ width: '100%', height: '100%' }}
            onEvents={onEvents}
            chartRef={chartInstanceRef}
          />
        </div>

        {selectedNode && (
          <div className="side-panel">
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
  );
};

export default React.memo(ServiceMap);
