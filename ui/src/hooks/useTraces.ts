import { useCallback, useEffect, useState } from 'react'
import type { Trace, TracesResponse } from '@/types/api'

export function useTraces() {
  const [traces, setTraces] = useState<Trace[]>([])
  const [selected, setSelected] = useState<Trace | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/traces?limit=25&offset=0')
      const data: TracesResponse = await res.json()
      setTraces(data.traces ?? [])
      if (data.traces?.[0]) {
        const detail = await fetch(`/api/traces/${data.traces[0].trace_id}`)
        setSelected((await detail.json()) as Trace)
      }
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load().catch(() => undefined)
  }, [load])

  const selectTrace = async (traceId: string) => {
    const res = await fetch(`/api/traces/${traceId}`)
    setSelected((await res.json()) as Trace)
  }

  return { traces, selected, loading, error, selectTrace, reload: load }
}
