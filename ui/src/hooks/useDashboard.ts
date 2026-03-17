import { useCallback, useEffect, useState } from 'react'
import type { DashboardStats, RepoStats } from '@/types/api'

export function useDashboard() {
  const [dashboard, setDashboard] = useState<DashboardStats | null>(null)
  const [stats, setStats] = useState<RepoStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [dashboardRes, statsRes] = await Promise.all([
        fetch('/api/metrics/dashboard'),
        fetch('/api/stats'),
      ])
      const [dashboardData, statsData] = await Promise.all([
        dashboardRes.json(),
        statsRes.json(),
      ])
      setDashboard(dashboardData)
      setStats(statsData)
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  return { dashboard, stats, loading, error, reload: load }
}
