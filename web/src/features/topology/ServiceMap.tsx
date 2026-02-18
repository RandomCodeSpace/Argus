import { useMemo } from 'react'
import { Paper, Title, Stack, Text, Box, LoadingOverlay, Group } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import ReactEChartsCore from 'echarts-for-react/lib/core'
import * as echarts from 'echarts/core'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { ServiceMapMetrics } from '../../types'
import { TimeRangeSelector, useTimeRange } from '../../components/TimeRangeSelector'
import { useFilterParamString } from '../../hooks/useFilterParams'

echarts.use([GraphChart, TooltipComponent, CanvasRenderer])

export function ServiceMap() {
    const tr = useTimeRange('5m')
    const [, setPage] = useFilterParamString('page', 'map')
    const [, setService] = useFilterParamString('service', '')

    const { data: metrics, isLoading } = useQuery<ServiceMapMetrics>({
        queryKey: ['serviceMapMetrics', tr.start, tr.end],
        queryFn: async () => {
            const res = await fetch(`/api/metrics/service-map?start=${tr.start}&end=${tr.end}`)
            return res.json()
        },
        refetchInterval: 30000,
    })

    const chartOption = useMemo(() => {
        if (!metrics) return {}

        const nodes = metrics.nodes.map((n, i) => ({
            id: n.name,
            name: n.name,
            symbolSize: 40 + Math.min(n.total_traces / 10, 60),
            itemStyle: { color: ['#4c6ef5', '#12b886', '#f59f00', '#fa5252', '#7950f2', '#15aabf', '#e64980'][i % 7] },
            label: { show: true, fontSize: 12, fontWeight: 'bold' as const },
            value: n.total_traces, // For tooltip
            // Custom data for tooltip
            data: n,
        }))

        const links = metrics.edges.map(e => ({
            source: e.source,
            target: e.target,
            value: e.call_count,
            lineStyle: {
                width: Math.min(1 + e.call_count / 10, 8),
                curveness: 0.1,
                opacity: 0.6,
                color: e.error_rate > 0.05 ? '#fa5252' : '#868e96'
            },
            label: {
                show: true,
                formatter: `${e.call_count} calls\n${e.avg_latency_ms}ms`,
                fontSize: 10,
                color: '#868e96'
            },
            // Custom data
            data: e,
        }))

        return {
            tooltip: {
                trigger: 'item',
                formatter: (params: any) => {
                    if (params.dataType === 'node') {
                        const n = params.data.data
                        return `
                            <strong>${n.name}</strong><br/>
                            Total Traces: ${n.total_traces}<br/>
                            Errors: ${n.error_count}<br/>
                            Avg Latency: ${n.avg_latency_ms}ms
                        `
                    }
                    const e = params.data.data
                    return `
                        <strong>${e.source} → ${e.target}</strong><br/>
                        Calls: ${e.call_count}<br/>
                        Error Rate: ${(e.error_rate * 100).toFixed(1)}%<br/>
                        Avg Latency: ${e.avg_latency_ms}ms
                    `
                },
            },
            series: [{
                type: 'graph',
                layout: 'force',
                roam: true,
                draggable: true,
                force: {
                    repulsion: 400,
                    edgeLength: [150, 250],
                    gravity: 0.1,
                },
                emphasis: {
                    focus: 'adjacency',
                    lineStyle: { width: 6 },
                },
                data: nodes,
                links: links,
            }],
        }
    }, [metrics])

    const onChartClick = (params: any) => {
        if (params.dataType === 'node') {
            const serviceName = params.data.name
            // Navigate to Logs filtered by this service
            setService(serviceName)
            setPage('logs')
        } else if (params.dataType === 'edge') {
            const e = params.data.data
            // Navigate to Traces filtered by source service (target filtering isn't supported yet, but source is good start)
            setService(e.source)
            setPage('traces')
        }
    }

    return (
        <Stack gap="md">
            <Group justify="space-between">
                <Title order={3}>Service Map</Title>
                <TimeRangeSelector
                    value={tr.timeRange}
                    onChange={tr.setTimeRange}
                />
            </Group>
            <Paper shadow="xs" radius="md" withBorder style={{ position: 'relative' }}>
                <LoadingOverlay visible={isLoading} />
                <Box style={{ height: 'calc(100vh - 150px)' }}>
                    {metrics && metrics.nodes.length > 0 ? (
                        <ReactEChartsCore
                            echarts={echarts}
                            option={chartOption}
                            style={{ height: '100%' }}
                            onEvents={{ click: onChartClick }}
                        />
                    ) : (
                        <Box style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                            <Text c="dimmed">No service data available yet. Start sending traces to see the service map.</Text>
                        </Box>
                    )}
                </Box>
            </Paper>
        </Stack>
    )
}

