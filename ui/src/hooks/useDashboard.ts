import { useCallback, useEffect, useRef, useState } from 'react';
import type { DashboardStats, RepoStats } from '../types/api';

export function useDashboard(pollInterval = 30_000) {
  const [dashboard, setDashboard] = useState<DashboardStats | null>(null);
  const [stats, setStats] = useState<RepoStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>(undefined);

  const load = useCallback(async () => {
    try {
      const [dRes, sRes] = await Promise.all([
        fetch('/api/metrics/dashboard'),
        fetch('/api/stats'),
      ]);
      if (!dRes.ok || !sRes.ok) throw new Error('fetch failed');
      setDashboard((await dRes.json()) as DashboardStats);
      setStats((await sRes.json()) as RepoStats);
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'fetch failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, pollInterval);
    return () => clearInterval(timerRef.current);
  }, [load, pollInterval]);

  return { dashboard, stats, loading, error, reload: load };
}
