import { Group, Box, Text, Badge, UnstyledButton } from '@mantine/core'
import { ChevronRight, ChevronDown } from 'lucide-react'
import type { LogEntry } from '../../../types'

interface LogRowProps {
    log: LogEntry
    isExpanded: boolean
    onToggle: (id: number) => void
}

const SEVERITY_COLORS: Record<string, string> = {
    ERROR: 'red',
    WARN: 'yellow',
    INFO: 'blue',
    DEBUG: 'gray',
}

export function LogRow({ log, isExpanded, onToggle }: LogRowProps) {
    return (
        <UnstyledButton
            onClick={() => onToggle(log.id)}
            style={{
                width: '100%',
                borderBottom: '1px solid var(--mantine-color-gray-2)',
                transition: 'background-color 0.2s ease',
            }}
            bg={isExpanded ? 'var(--mantine-color-blue-0)' : 'transparent'}
        >
            <Group gap={0} px="sm" py={8} style={{ flexWrap: 'nowrap' }}>
                <Box style={{ width: 40, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                </Box>
                <Box style={{ width: 80 }}>
                    <Badge size="xs" color={SEVERITY_COLORS[log.severity] || 'gray'} variant="filled">
                        {log.severity}
                    </Badge>
                </Box>
                <Box style={{ width: 170 }}>
                    <Text size="xs" c="dimmed">
                        {new Date(log.timestamp).toLocaleString()}
                    </Text>
                </Box>
                <Box style={{ width: 140 }}>
                    <Text size="xs" fw={500} truncate>
                        {log.service_name}
                    </Text>
                </Box>
                <Box style={{ flex: 1, minWidth: 0 }}>
                    <Text size="xs" truncate style={{ fontFamily: 'var(--font-mono)' }}>
                        {log.body}
                    </Text>
                </Box>
            </Group>
        </UnstyledButton>
    )
}
