import { useCallback, useEffect, useRef, useState } from 'react'
import type { MutableRefObject } from 'react'
import type { LogEntry } from '@/types/api'

interface HubBatch {
  type: string
  data: unknown
}

export type WebSocketStatus =
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'reconnecting'

/**
 * Returned ref preserves the original `ws.current` API while also
 * exposing the live `status` for UI consumers that want an accurate
 * connection indicator.
 */
export type WebSocketRef = MutableRefObject<WebSocket | null> & {
  status: WebSocketStatus
}

const INITIAL_BACKOFF_MS = 100
const MAX_BACKOFF_MS = 10_000
const HEARTBEAT_INTERVAL_MS = 30_000
const HEARTBEAT_TIMEOUT_MS = 35_000

/**
 * Hardened WebSocket hook:
 *   - Exponential backoff reconnect (100ms → 10s cap, reset on open).
 *   - 30s ping heartbeat with 35s dead-connection detection.
 *   - Reconnect on `visibilitychange` (tab foregrounded) and `online`.
 *   - Cleans up all timers + listeners on unmount (StrictMode-safe).
 *
 * The returned ref keeps the original `.current` accessor so existing
 * callers (e.g. `!!ws.current`) continue to work. A new `.status` field
 * is attached for richer state.
 */
export function useWebSocket(onLogs: (logs: LogEntry[]) => void): WebSocketRef {
  const socketRef = useRef<WebSocket | null>(null) as WebSocketRef
  const onLogsRef = useRef(onLogs)
  const [status, setStatus] = useState<WebSocketStatus>('connecting')
  // Mirror state onto the ref so consumers can do `ws.status` directly.
  socketRef.status = status

  // Mutable bookkeeping — never trigger re-renders / effect re-runs.
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimerRef = useRef<number | null>(null)
  const heartbeatIntervalRef = useRef<number | null>(null)
  const heartbeatTimeoutRef = useRef<number | null>(null)
  const unmountedRef = useRef(false)
  // Break the connect ↔ scheduleReconnect cycle without retriggering
  // the mount effect under StrictMode.
  const connectRef = useRef<() => void>(() => {})

  useEffect(() => {
    onLogsRef.current = onLogs
  }, [onLogs])

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const clearHeartbeat = useCallback(() => {
    if (heartbeatIntervalRef.current !== null) {
      window.clearInterval(heartbeatIntervalRef.current)
      heartbeatIntervalRef.current = null
    }
    if (heartbeatTimeoutRef.current !== null) {
      window.clearTimeout(heartbeatTimeoutRef.current)
      heartbeatTimeoutRef.current = null
    }
  }, [])

  const scheduleReconnect = useCallback(() => {
    if (unmountedRef.current) return
    clearReconnectTimer()
    const attempt = reconnectAttemptsRef.current
    const delay = Math.min(
      INITIAL_BACKOFF_MS * 2 ** attempt,
      MAX_BACKOFF_MS,
    )
    reconnectAttemptsRef.current = attempt + 1
    setStatus('reconnecting')
    reconnectTimerRef.current = window.setTimeout(() => {
      reconnectTimerRef.current = null
      connectRef.current()
    }, delay)
  }, [clearReconnectTimer])

  const startHeartbeat = useCallback(() => {
    clearHeartbeat()
    heartbeatIntervalRef.current = window.setInterval(() => {
      const ws = socketRef.current
      if (!ws || ws.readyState !== WebSocket.OPEN) return
      try {
        ws.send(JSON.stringify({ type: 'ping' }))
      } catch {
        // Send failed — close/error handlers will drive recovery.
        return
      }
      // Any inbound message within the timeout clears this watchdog;
      // otherwise the connection is treated as dead.
      if (heartbeatTimeoutRef.current !== null) {
        window.clearTimeout(heartbeatTimeoutRef.current)
      }
      heartbeatTimeoutRef.current = window.setTimeout(() => {
        heartbeatTimeoutRef.current = null
        const stale = socketRef.current
        if (stale) {
          try {
            stale.close()
          } catch {
            // noop
          }
        }
      }, HEARTBEAT_TIMEOUT_MS)
    }, HEARTBEAT_INTERVAL_MS)
  }, [clearHeartbeat])

  const connect = useCallback(() => {
    if (unmountedRef.current) return
    clearReconnectTimer()
    clearHeartbeat()

    // Detach any prior socket to prevent its handlers from triggering
    // a duplicate reconnect when we close it below.
    const existing = socketRef.current
    if (existing) {
      existing.onopen = null
      existing.onmessage = null
      existing.onerror = null
      existing.onclose = null
      try {
        existing.close()
      } catch {
        // noop
      }
      socketRef.current = null
    }

    setStatus(reconnectAttemptsRef.current === 0 ? 'connecting' : 'reconnecting')

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    let ws: WebSocket
    try {
      ws = new WebSocket(`${protocol}//${window.location.host}/ws`)
    } catch {
      scheduleReconnect()
      return
    }
    socketRef.current = ws

    ws.onopen = () => {
      if (unmountedRef.current) return
      reconnectAttemptsRef.current = 0
      setStatus('connected')
      startHeartbeat()
    }

    ws.onmessage = (event: MessageEvent<string>) => {
      // Any message proves liveness — clear the dead-connection watchdog.
      if (heartbeatTimeoutRef.current !== null) {
        window.clearTimeout(heartbeatTimeoutRef.current)
        heartbeatTimeoutRef.current = null
      }
      try {
        const payload = JSON.parse(event.data) as HubBatch
        if (payload.type === 'logs' && Array.isArray(payload.data)) {
          onLogsRef.current(payload.data as LogEntry[])
        }
      } catch {
        // Ignore malformed or non-JSON frames (incl. server ping echoes).
      }
    }

    ws.onerror = () => {
      // `onclose` always follows — it owns reconnect scheduling.
    }

    ws.onclose = () => {
      if (unmountedRef.current) return
      if (socketRef.current === ws) {
        socketRef.current = null
      }
      clearHeartbeat()
      setStatus('disconnected')
      scheduleReconnect()
    }
  }, [clearHeartbeat, clearReconnectTimer, scheduleReconnect, startHeartbeat])

  useEffect(() => {
    connectRef.current = connect
  }, [connect])

  useEffect(() => {
    unmountedRef.current = false
    connectRef.current = connect
    connect()

    const handleVisibility = () => {
      if (document.visibilityState !== 'visible') return
      const ws = socketRef.current
      const dead =
        !ws ||
        ws.readyState === WebSocket.CLOSED ||
        ws.readyState === WebSocket.CLOSING
      if (dead) {
        reconnectAttemptsRef.current = 0
        clearReconnectTimer()
        connectRef.current()
      }
    }

    const handleOnline = () => {
      reconnectAttemptsRef.current = 0
      clearReconnectTimer()
      connectRef.current()
    }

    document.addEventListener('visibilitychange', handleVisibility)
    window.addEventListener('online', handleOnline)

    return () => {
      unmountedRef.current = true
      document.removeEventListener('visibilitychange', handleVisibility)
      window.removeEventListener('online', handleOnline)
      clearReconnectTimer()
      clearHeartbeat()
      const ws = socketRef.current
      if (ws) {
        ws.onopen = null
        ws.onmessage = null
        ws.onerror = null
        ws.onclose = null
        try {
          ws.close()
        } catch {
          // noop
        }
        socketRef.current = null
      }
    }
    // Connect once per mount. `connect` is stable (refs + setState).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return socketRef
}
