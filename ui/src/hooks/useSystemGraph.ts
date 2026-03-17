import { useCallback, useEffect, useState } from 'react'
import type { SystemGraphResponse } from '@/types/api'

export function useSystemGraph() {
  const [graph, setGraph] = useState<SystemGraphResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [cache, setCache] = useState('MISS')

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/system/graph')
      setCache(res.headers.get('X-Cache') ?? 'MISS')
      setGraph(await res.json())
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  return { graph, cache, loading, error, reload: load }
}
