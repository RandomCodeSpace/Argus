import { useCallback, useEffect, useState } from 'react'
import type { LatencyPoint, MetricBucket, ServiceMapMetrics, TrafficPoint } from '@/types/api'

export function useMetrics() {
  const [traffic, setTraffic] = useState<TrafficPoint[]>([])
  const [heatmap, setHeatmap] = useState<LatencyPoint[]>([])
  const [serviceMap, setServiceMap] = useState<ServiceMapMetrics | null>(null)
  const [buckets, setBuckets] = useState<MetricBucket[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      // Fetch time-series endpoints in parallel alongside metric name discovery.
      // /api/metrics requires ?name=, so we first get available names then fetch buckets.
      const [trafficRes, heatmapRes, mapRes, namesRes] = await Promise.all([
        fetch('/api/metrics/traffic'),
        fetch('/api/metrics/latency_heatmap'),
        fetch('/api/metrics/service-map'),
        fetch('/api/metadata/metrics'),
      ])
      const [trafficData, heatmapData, mapData, namesData] = await Promise.all([
        trafficRes.json(),
        heatmapRes.json(),
        mapRes.json(),
        namesRes.ok ? namesRes.json() : Promise.resolve([]),
      ])
      setTraffic(Array.isArray(trafficData) ? trafficData : [])
      setHeatmap(Array.isArray(heatmapData) ? heatmapData : [])
      setServiceMap(mapData)

      // Fetch buckets for first 5 metric names to populate the buckets panel.
      const names: string[] = Array.isArray(namesData) ? namesData.slice(0, 5) : []
      if (names.length > 0) {
        const bucketResults = await Promise.all(
          names.map((name) =>
            fetch(`/api/metrics?name=${encodeURIComponent(name)}`)
              .then((r) => (r.ok ? r.json() : Promise.resolve([])))
              .then((d) => (Array.isArray(d) ? d : []))
          )
        )
        setBuckets(bucketResults.flat())
      } else {
        setBuckets([])
      }
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  return { traffic, heatmap, serviceMap, buckets, loading, error, reload: load }
}
