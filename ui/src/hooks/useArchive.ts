import { useState } from 'react'

export function useArchive() {
  const [results, setResults] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const search = async (type: 'logs' | 'traces' | 'metrics', query: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(`/api/archive/search?type=${type}&q=${encodeURIComponent(query)}`)
      const text = await res.text()
      setResults(text.split('\n').filter(Boolean))
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }

  return { results, loading, error, search }
}
