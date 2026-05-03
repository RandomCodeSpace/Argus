import { useCallback, useEffect, useState } from 'react'
import type { LogEntry, LogsResponse } from '@/types/api'

function normalizeLogs(data: LogsResponse | LogEntry[]): LogEntry[] {
  if (Array.isArray(data)) return data
  if (Array.isArray(data.logs)) return data.logs
  if (Array.isArray(data.items)) return data.items
  return []
}

export function useLogs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [similar, setSimilar] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/logs?limit=100&offset=0')
      const data: LogsResponse | LogEntry[] = await res.json()
      setLogs(normalizeLogs(data))
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load().catch(() => undefined)
  }, [load])

  const runSimilar = async (query: string) => {
    if (!query.trim()) return
    const res = await fetch(`/api/logs/similar?q=${encodeURIComponent(query)}&limit=8`)
    const data: LogsResponse | LogEntry[] = await res.json()
    setSimilar(normalizeLogs(data))
  }

  return { logs, similar, loading, error, runSimilar, setLogs, reload: load }
}
