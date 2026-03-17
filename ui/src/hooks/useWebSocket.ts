import { useEffect, useRef } from 'react'
import type { LogEntry } from '@/types/api'

interface HubBatch {
  type: string
  data: unknown
}

export function useWebSocket(onLogs: (logs: LogEntry[]) => void) {
  const socketRef = useRef<WebSocket | null>(null)
  const onLogsRef = useRef(onLogs)

  // Update the callback ref on every render, but don't trigger reconnects.
  useEffect(() => {
    onLogsRef.current = onLogs
  }, [onLogs])

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws`)
    socketRef.current = ws

    ws.onmessage = (event) => {
      try {
        const payload: HubBatch = JSON.parse(event.data)
        if (payload.type === 'logs' && Array.isArray(payload.data)) {
          onLogsRef.current(payload.data as LogEntry[])
        }
      } catch {
        // Ignore malformed frames.
      }
    }

    return () => {
      ws.close()
    }
  }, []) // No dependencies — connect once on mount

  return socketRef
}
