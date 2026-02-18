import { useEffect, useState, useMemo } from 'react'
import {
    Paper,
    Group,
    Title,
    Select,
    Stack,
    SimpleGrid,
    Text,
    Badge,
    ThemeIcon,
    Box,
} from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import ReactEChartsCore from 'echarts-for-react/lib/core'
import * as echarts from 'echarts/core'
import { LineChart, BarChart, HeatmapChart } from 'echarts/charts'
import {
    GridComponent,
    TooltipComponent,
    LegendComponent,
    VisualMapComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { Activity, AlertTriangle, Clock, Layers } from 'lucide-react'
import type { TrafficPoint, ServiceError, DashboardStats } from '../../types'
import { TimeRangeSelector, useTimeRange } from '../../components/TimeRangeSelector'
import { useFilterParam } from '../../hooks/useFilterParams'

echarts.use([
    LineChart, BarChart, HeatmapChart,
    GridComponent, TooltipComponent, LegendComponent, VisualMapComponent,
    CanvasRenderer,
])

export function Dashboard() {
    const tr = useTimeRange('15m')
    const [selectedService, setSelectedService] = useFilterParam('service', null)

    const { data: services } = useQuery<string[]>({
        queryKey: ['services'],
        queryFn: () => fetch('/api/metadata/services').then(r => r.json()),
    })

    const serviceParams = selectedService ? `&service_name=${encodeURIComponent(selectedService)}` : ''

    const { data: traffic } = useQuery<TrafficPoint[]>({
        queryKey: ['traffic', tr.start, tr.end, selectedService],
        queryFn: () => fetch(`/api/metrics/traffic?start=${tr.start}&end=${tr.end}${serviceParams}`).then(r => r.json()),
        refetchInterval: 30000,
    })

    const { data: topFailing } = useQuery<ServiceError[]>({
        queryKey: ['topFailing', tr.start, tr.end],
        queryFn: () => fetch(`/api/metrics/dashboard?start=${tr.start}&end=${tr.end}`).then(r => r.json()),
        refetchInterval: 30000,
        select: (data: any) => data?.top_failing_services || [],
    })

    const { data: stats } = useQuery<DashboardStats>({
        queryKey: ['dashboardStats', tr.start, tr.end, selectedService],
        queryFn: () => fetch(`/api/metrics/dashboard?start=${tr.start}&end=${tr.end}${serviceParams}`).then(r => r.json()),
        refetchInterval: 30000,
    })

    // Auto-refresh time range
    const [, setTick] = useState(0)
    useEffect(() => {
        const interval = setInterval(() => setTick(t => t + 1), 30000)
        return () => clearInterval(interval)
    }, [])

    const trafficChartOption = useMemo(() => ({
        tooltip: { trigger: 'axis' },
        grid: { left: 50, right: 20, top: 20, bottom: 30 },
        xAxis: {
            type: 'time',
            axisLabel: { formatter: (val: number) => new Date(val).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) },
        },
        yAxis: { type: 'value', name: 'Requests' },
        series: [
            {
                name: 'Total',
                type: 'line',
                smooth: true,
                data: (traffic || []).map(p => [new Date(p.timestamp).getTime(), p.count]),
                areaStyle: { opacity: 0.1, color: '#4c6ef5' },
                lineStyle: { color: '#4c6ef5', width: 2 },
                itemStyle: { color: '#4c6ef5' },
            },
            {
                name: 'Errors',
                type: 'line',
                smooth: true,
                data: (traffic || []).map(p => [new Date(p.timestamp).getTime(), p.error_count]),
                areaStyle: { opacity: 0.1, color: '#fa5252' },
                lineStyle: { color: '#fa5252', width: 2 },
                itemStyle: { color: '#fa5252' },
            },
        ],
    }), [traffic])

    const failingChartOption = useMemo(() => ({
        tooltip: { trigger: 'axis' },
        grid: { left: 120, right: 30, top: 10, bottom: 10 },
        xAxis: { type: 'value', name: 'Error Rate %' },
        yAxis: {
            type: 'category',
            data: (topFailing || []).map(s => s.service_name),
        },
        series: [{
            type: 'bar',
            data: (topFailing || []).map(s => ({
                value: +(s.error_rate * 100).toFixed(1),
                itemStyle: { color: s.error_rate > 0.1 ? '#fa5252' : s.error_rate > 0.05 ? '#fd7e14' : '#40c057' },
            })),
            barWidth: 16,
            itemStyle: { borderRadius: [0, 4, 4, 0] },
        }],
    }), [topFailing])

    const statCards = [
        { label: 'Total Traces', value: stats?.total_traces ?? 0, icon: Layers, color: 'indigo' },
        { label: 'Total Logs', value: stats?.total_logs ?? 0, icon: Activity, color: 'cyan' },
        { label: 'Total Errors', value: stats?.total_errors ?? 0, icon: AlertTriangle, color: 'red' },
        { label: 'Avg Latency', value: `${(stats?.avg_latency_ms ?? 0).toFixed(1)}ms`, icon: Clock, color: 'orange' },
    ]

    return (
        <Stack gap="md">
            {/* Header */}
            <Group justify="space-between">
                <Title order={3}>Dashboard</Title>
                <Group gap="sm">
                    <Select
                        size="xs"
                        data={[{ value: '', label: 'All Services' }, ...(services || []).map(s => ({ value: s, label: s }))]}
                        value={selectedService || ''}
                        onChange={(v) => setSelectedService(v || null)}
                        placeholder="Filter by service"
                        clearable
                        styles={{ input: { width: 180 } }}
                    />
                    <TimeRangeSelector
                        value={tr.timeRange}
                        onChange={tr.setTimeRange}
                    />
                </Group>
            </Group>

            {/* Stat Cards */}
            <SimpleGrid cols={{ base: 2, md: 4 }}>
                {statCards.map((s) => (
                    <Paper key={s.label} shadow="xs" p="md" radius="md" withBorder>
                        <Group justify="space-between" align="flex-start">
                            <Box>
                                <Text size="xs" c="dimmed" tt="uppercase" fw={600}>{s.label}</Text>
                                <Title order={3} mt={4}>{typeof s.value === 'number' ? s.value.toLocaleString() : s.value}</Title>
                            </Box>
                            <ThemeIcon variant="light" color={s.color} size="lg" radius="md">
                                <s.icon size={18} />
                            </ThemeIcon>
                        </Group>
                    </Paper>
                ))}
            </SimpleGrid>

            {/* Traffic Chart */}
            <Paper shadow="xs" p="md" radius="md" withBorder>
                <Text fw={600} mb="sm">Traffic Over Time</Text>
                <Box style={{ height: 300 }}>
                    <ReactEChartsCore echarts={echarts} option={trafficChartOption} style={{ height: '100%' }} />
                </Box>
            </Paper>

            {/* Top Failing Services */}
            <Paper shadow="xs" p="md" radius="md" withBorder>
                <Group justify="space-between" mb="sm">
                    <Text fw={600}>Top Failing Services</Text>
                    <Badge variant="light" color="red" size="sm">{(topFailing || []).length} services</Badge>
                </Group>
                <Box style={{ height: 250 }}>
                    {(topFailing || []).length > 0 ? (
                        <ReactEChartsCore echarts={echarts} option={failingChartOption} style={{ height: '100%' }} />
                    ) : (
                        <Group justify="center" align="center" style={{ height: '100%' }}>
                            <Text c="dimmed">No failing services detected</Text>
                        </Group>
                    )}
                </Box>
            </Paper>
        </Stack>
    )
}
