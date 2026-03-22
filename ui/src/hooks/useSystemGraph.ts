import { useCallback, useEffect, useRef, useState } from 'react';
import type { SystemGraphResponse } from '../types/api';

export function useSystemGraph(pollInterval = 60_000) {
  const [graph, setGraph] = useState<SystemGraphResponse | null>(null);
  const [cache, setCache] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>(undefined);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/system/graph');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setCache(res.headers.get('X-Cache') ?? '');
      setGraph(await res.json());
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

  return { graph, cache, loading, error, reload: load };
}
