import { Stack, Group, Text, Box, Code, Title, Paper } from '@mantine/core'
import { Cpu, Terminal, Sparkles } from 'lucide-react'
import type { LogEntry } from '../../../types'

interface LogDetailsProps {
    log: LogEntry
}

export function LogDetails({ log }: LogDetailsProps) {
    let attributes = {}
    try {
        attributes = JSON.parse(log.attributes_json || '{}')
    } catch (e) {
        console.error('Failed to parse log attributes', e)
    }

    return (
        <Box p="md" bg="var(--mantine-color-gray-0)" style={{ borderBottom: '1px solid var(--mantine-color-gray-2)' }}>
            <Stack gap="md">
                <Box>
                    <Group gap="xs" mb={4}>
                        <Terminal size={14} />
                        <Title order={6} size="xs" c="dimmed">FULL MESSAGE</Title>
                    </Group>
                    <Text size="sm" ff="var(--font-mono)" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                        {log.body}
                    </Text>
                </Box>

                {log.ai_insight && (
                    <Paper p="sm" radius="md" bg="violet.0" withBorder style={{ borderColor: 'var(--mantine-color-violet-2)' }}>
                        <Group gap="xs" mb={4}>
                            <Sparkles size={14} color="var(--mantine-color-violet-6)" />
                            <Title order={6} size="xs" c="violet.7">AI INSIGHT</Title>
                        </Group>
                        <Text size="sm">{log.ai_insight}</Text>
                    </Paper>
                )}

                <Box>
                    <Group gap="xs" mb={4}>
                        <Cpu size={14} />
                        <Title order={6} size="xs" c="dimmed">ATTRIBUTES</Title>
                    </Group>
                    <Code block>{JSON.stringify(attributes, null, 2)}</Code>
                </Box>

                <Group gap="xl">
                    <Box>
                        <Text size="xs" fw={700} c="dimmed">TRACE ID</Text>
                        <Text size="xs" ff="var(--font-mono)">{log.trace_id || '—'}</Text>
                    </Box>
                    <Box>
                        <Text size="xs" fw={700} c="dimmed">SPAN ID</Text>
                        <Text size="xs" ff="var(--font-mono)">{log.span_id || '—'}</Text>
                    </Box>
                </Group>
            </Stack>
        </Box>
    )
}
