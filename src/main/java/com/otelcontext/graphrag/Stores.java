package com.otelcontext.graphrag;

import com.otelcontext.graphrag.Schema.*;

import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/** The 4 typed stores for the GraphRAG layered graph. */
public final class Stores {

    // --- Mutable node wrappers (records are immutable, so we use mutable classes internally) ---

    static class MutableServiceNode {
        String id, name;
        Instant firstSeen, lastSeen;
        double healthScore, errorRate, avgLatency, totalMs;
        long callCount, errorCount;

        ServiceNode toRecord() {
            return new ServiceNode(id, name, firstSeen, lastSeen, healthScore, callCount, errorCount, errorRate, avgLatency, totalMs);
        }
    }

    static class MutableOperationNode {
        String id, service, operation;
        Instant firstSeen, lastSeen;
        double healthScore, errorRate, avgLatency, totalMs;
        long callCount, errorCount;

        OperationNode toRecord() {
            return new OperationNode(id, service, operation, firstSeen, lastSeen, healthScore, callCount, errorCount, errorRate, avgLatency, totalMs);
        }
    }

    static class MutableEdge {
        EdgeType type; String fromId, toId;
        double weight, errorRate, avgMs, totalMs;
        long callCount, errorCount;
        Instant updatedAt;

        Edge toRecord() {
            return new Edge(type, fromId, toId, weight, callCount, errorRate, avgMs, totalMs, errorCount, updatedAt);
        }
    }

    static class MutableTraceNode {
        String id, rootService, status; double durationMs; Instant timestamp; int spanCount;
        TraceNode toRecord() { return new TraceNode(id, rootService, durationMs, status, timestamp, spanCount); }
    }

    static class MutableLogCluster {
        String id, template; long count; Instant firstSeen, lastSeen;
        Map<String, Long> severityDist = new HashMap<>();
        LogClusterNode toRecord() { return new LogClusterNode(id, template, count, firstSeen, lastSeen, Map.copyOf(severityDist)); }
    }

    static class MutableMetricNode {
        String id, metricName, service; double rollingMin, rollingMax, rollingAvg; long sampleCount; Instant lastSeen;
        MetricNode toRecord() { return new MetricNode(id, metricName, service, rollingMin, rollingMax, rollingAvg, sampleCount, lastSeen); }
    }

    // ================================ ServiceStore ================================
    public static class ServiceStore {
        final ReentrantReadWriteLock lock = new ReentrantReadWriteLock();
        final Map<String, MutableServiceNode> services = new HashMap<>();
        final Map<String, MutableOperationNode> operations = new HashMap<>();
        final Map<String, MutableEdge> edges = new HashMap<>();

        public void upsertService(String name, double durationMs, boolean isError, Instant ts) {
            lock.writeLock().lock();
            try {
                var svc = services.computeIfAbsent(name, k -> {
                    var s = new MutableServiceNode();
                    s.id = name; s.name = name; s.firstSeen = ts; s.lastSeen = ts;
                    return s;
                });
                svc.callCount++;
                svc.totalMs += durationMs;
                if (isError) svc.errorCount++;
                if (ts.isAfter(svc.lastSeen)) svc.lastSeen = ts;
                if (ts.isBefore(svc.firstSeen)) svc.firstSeen = ts;
                svc.avgLatency = svc.totalMs / svc.callCount;
                svc.errorRate = (double) svc.errorCount / svc.callCount;
                svc.healthScore = Schema.computeHealth(svc.errorRate, svc.avgLatency);
            } finally { lock.writeLock().unlock(); }
        }

        public void upsertOperation(String service, String operation, double durationMs, boolean isError, Instant ts) {
            String key = service + "|" + operation;
            lock.writeLock().lock();
            try {
                var op = operations.computeIfAbsent(key, k -> {
                    var o = new MutableOperationNode();
                    o.id = key; o.service = service; o.operation = operation; o.firstSeen = ts; o.lastSeen = ts;
                    return o;
                });
                op.callCount++;
                op.totalMs += durationMs;
                if (isError) op.errorCount++;
                if (ts.isAfter(op.lastSeen)) op.lastSeen = ts;
                op.avgLatency = op.totalMs / op.callCount;
                op.errorRate = (double) op.errorCount / op.callCount;
                op.healthScore = Schema.computeHealth(op.errorRate, op.avgLatency);

                String ek = Schema.edgeKey(EdgeType.EXPOSES, service, key);
                edges.computeIfAbsent(ek, k -> {
                    var e = new MutableEdge(); e.type = EdgeType.EXPOSES; e.fromId = service; e.toId = key; e.updatedAt = ts;
                    return e;
                });
            } finally { lock.writeLock().unlock(); }
        }

        public void upsertCallEdge(String source, String target, double durationMs, boolean isError, Instant ts) {
            String ek = Schema.edgeKey(EdgeType.CALLS, source, target);
            lock.writeLock().lock();
            try {
                var e = edges.computeIfAbsent(ek, k -> {
                    var edge = new MutableEdge(); edge.type = EdgeType.CALLS; edge.fromId = source; edge.toId = target;
                    return edge;
                });
                e.callCount++;
                e.totalMs += durationMs;
                if (isError) e.errorCount++;
                e.avgMs = e.totalMs / e.callCount;
                e.errorRate = (double) e.errorCount / e.callCount;
                e.weight = e.callCount;
                e.updatedAt = ts;
            } finally { lock.writeLock().unlock(); }
        }

        public ServiceNode getService(String name) {
            lock.readLock().lock();
            try { var s = services.get(name); return s != null ? s.toRecord() : null; }
            finally { lock.readLock().unlock(); }
        }

        public List<ServiceNode> allServices() {
            lock.readLock().lock();
            try { return services.values().stream().map(MutableServiceNode::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<Edge> allEdges() {
            lock.readLock().lock();
            try { return edges.values().stream().map(MutableEdge::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<Edge> callEdgesFrom(String service) {
            lock.readLock().lock();
            try { return edges.values().stream().filter(e -> e.type == EdgeType.CALLS && e.fromId.equals(service)).map(MutableEdge::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<Edge> callEdgesTo(String service) {
            lock.readLock().lock();
            try { return edges.values().stream().filter(e -> e.type == EdgeType.CALLS && e.toId.equals(service)).map(MutableEdge::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<OperationNode> operationsForService(String service) {
            lock.readLock().lock();
            try { return operations.values().stream().filter(o -> o.service.equals(service)).map(MutableOperationNode::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }
    }

    // ================================ TraceStore ================================
    public static class TraceStore {
        final ReentrantReadWriteLock lock = new ReentrantReadWriteLock();
        final Map<String, MutableTraceNode> traces = new HashMap<>();
        final Map<String, SpanNode> spans = new ConcurrentHashMap<>();
        final Map<String, MutableEdge> edges = new HashMap<>();
        final Duration ttl;

        public TraceStore(Duration ttl) { this.ttl = ttl; }

        public void upsertTrace(String traceId, String rootService, String status, double durationMs, Instant ts) {
            lock.writeLock().lock();
            try {
                var t = traces.computeIfAbsent(traceId, k -> {
                    var tn = new MutableTraceNode(); tn.id = traceId; tn.rootService = rootService; tn.status = status; tn.durationMs = durationMs; tn.timestamp = ts;
                    return tn;
                });
                t.spanCount++;
                if (durationMs > t.durationMs) t.durationMs = durationMs;
                if ("STATUS_CODE_ERROR".equals(status)) t.status = status;
            } finally { lock.writeLock().unlock(); }
        }

        public void upsertSpan(SpanNode span) {
            spans.put(span.id(), span);
            lock.writeLock().lock();
            try {
                String ck = Schema.edgeKey(EdgeType.CONTAINS, span.traceId(), span.id());
                edges.computeIfAbsent(ck, k -> {
                    var e = new MutableEdge(); e.type = EdgeType.CONTAINS; e.fromId = span.traceId(); e.toId = span.id(); e.updatedAt = span.timestamp();
                    return e;
                });
                if (span.parentSpanId() != null && !span.parentSpanId().isEmpty()) {
                    String pk = Schema.edgeKey(EdgeType.CHILD_OF, span.id(), span.parentSpanId());
                    edges.computeIfAbsent(pk, k -> {
                        var e = new MutableEdge(); e.type = EdgeType.CHILD_OF; e.fromId = span.id(); e.toId = span.parentSpanId(); e.updatedAt = span.timestamp();
                        return e;
                    });
                }
            } finally { lock.writeLock().unlock(); }
        }

        public SpanNode getSpan(String spanId) { return spans.get(spanId); }

        public List<SpanNode> errorSpans(String service, Instant since) {
            return spans.values().stream()
                .filter(s -> s.isError() && s.service().equals(service) && s.timestamp().isAfter(since))
                .toList();
        }

        public List<SpanNode> spansForTrace(String traceId) {
            return spans.values().stream().filter(s -> s.traceId().equals(traceId)).toList();
        }

        public int prune() {
            Instant cutoff = Instant.now().minus(ttl);
            lock.writeLock().lock();
            try {
                int pruned = 0;
                var spanIter = spans.entrySet().iterator();
                while (spanIter.hasNext()) {
                    if (spanIter.next().getValue().timestamp().isBefore(cutoff)) { spanIter.remove(); pruned++; }
                }
                traces.entrySet().removeIf(e -> e.getValue().timestamp.isBefore(cutoff));
                edges.entrySet().removeIf(e -> e.getValue().updatedAt.isBefore(cutoff));
                return pruned;
            } finally { lock.writeLock().unlock(); }
        }
    }

    // ================================ SignalStore ================================
    public static class SignalStore {
        final ReentrantReadWriteLock lock = new ReentrantReadWriteLock();
        final Map<String, MutableLogCluster> logClusters = new HashMap<>();
        final Map<String, MutableMetricNode> metrics = new HashMap<>();
        final Map<String, MutableEdge> edges = new HashMap<>();

        public void upsertLogCluster(String id, String template, String severity, String service, Instant ts) {
            lock.writeLock().lock();
            try {
                var lc = logClusters.computeIfAbsent(id, k -> {
                    var c = new MutableLogCluster(); c.id = id; c.template = template; c.firstSeen = ts; c.lastSeen = ts;
                    return c;
                });
                lc.count++;
                lc.severityDist.merge(severity, 1L, Long::sum);
                if (ts.isAfter(lc.lastSeen)) lc.lastSeen = ts;

                String ek = Schema.edgeKey(EdgeType.EMITTED_BY, id, service);
                edges.computeIfAbsent(ek, k -> {
                    var e = new MutableEdge(); e.type = EdgeType.EMITTED_BY; e.fromId = id; e.toId = service; e.updatedAt = ts;
                    return e;
                });
            } finally { lock.writeLock().unlock(); }
        }

        public void upsertMetric(String metricName, String service, double value, Instant ts) {
            String key = metricName + "|" + service;
            lock.writeLock().lock();
            try {
                var m = metrics.computeIfAbsent(key, k -> {
                    var mn = new MutableMetricNode();
                    mn.id = key; mn.metricName = metricName; mn.service = service;
                    mn.rollingMin = value; mn.rollingMax = value; mn.rollingAvg = value; mn.lastSeen = ts;

                    String ek = Schema.edgeKey(EdgeType.MEASURED_BY, key, service);
                    edges.put(ek, new MutableEdge() {{ type = EdgeType.MEASURED_BY; fromId = key; toId = service; updatedAt = ts; }});
                    return mn;
                });
                m.sampleCount++;
                if (value < m.rollingMin) m.rollingMin = value;
                if (value > m.rollingMax) m.rollingMax = value;
                m.rollingAvg = m.rollingAvg * 0.9 + value * 0.1;
                m.lastSeen = ts;
            } finally { lock.writeLock().unlock(); }
        }

        public List<LogClusterNode> logClustersForService(String service) {
            lock.readLock().lock();
            try {
                List<LogClusterNode> out = new ArrayList<>();
                for (var e : edges.values()) {
                    if (e.type == EdgeType.EMITTED_BY && e.toId.equals(service)) {
                        var lc = logClusters.get(e.fromId);
                        if (lc != null) out.add(lc.toRecord());
                    }
                }
                return out;
            } finally { lock.readLock().unlock(); }
        }

        public List<MetricNode> metricsForService(String service) {
            lock.readLock().lock();
            try { return metrics.values().stream().filter(m -> m.service.equals(service)).map(MutableMetricNode::toRecord).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<MutableMetricNode> allMetricsMutable() {
            lock.readLock().lock();
            try { return new ArrayList<>(metrics.values()); }
            finally { lock.readLock().unlock(); }
        }
    }

    // ================================ AnomalyStore ================================
    public static class AnomalyStore {
        final ReentrantReadWriteLock lock = new ReentrantReadWriteLock();
        final Map<String, AnomalyNode> anomalies = new HashMap<>();
        final Map<String, MutableEdge> edges = new HashMap<>();

        public void addAnomaly(AnomalyNode anomaly) {
            lock.writeLock().lock();
            try {
                anomalies.put(anomaly.id(), anomaly);
                String ek = Schema.edgeKey(EdgeType.TRIGGERED_BY, anomaly.id(), anomaly.service());
                var e = new MutableEdge();
                e.type = EdgeType.TRIGGERED_BY; e.fromId = anomaly.id(); e.toId = anomaly.service(); e.updatedAt = anomaly.timestamp();
                edges.put(ek, e);
            } finally { lock.writeLock().unlock(); }
        }

        public void addPrecededByEdge(String anomalyId, String precedingId, Instant ts) {
            lock.writeLock().lock();
            try {
                String ek = Schema.edgeKey(EdgeType.PRECEDED_BY, anomalyId, precedingId);
                var e = new MutableEdge();
                e.type = EdgeType.PRECEDED_BY; e.fromId = anomalyId; e.toId = precedingId; e.updatedAt = ts;
                edges.put(ek, e);
            } finally { lock.writeLock().unlock(); }
        }

        public List<AnomalyNode> anomaliesSince(Instant since) {
            lock.readLock().lock();
            try { return anomalies.values().stream().filter(a -> a.timestamp().isAfter(since)).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public List<AnomalyNode> anomaliesForService(String service, Instant since) {
            lock.readLock().lock();
            try { return anomalies.values().stream().filter(a -> a.service().equals(service) && a.timestamp().isAfter(since)).toList(); }
            finally { lock.readLock().unlock(); }
        }

        public void pruneOlderThan(Instant cutoff) {
            lock.writeLock().lock();
            try {
                anomalies.entrySet().removeIf(e -> e.getValue().timestamp().isBefore(cutoff));
                edges.entrySet().removeIf(e -> e.getValue().updatedAt.isBefore(cutoff));
            } finally { lock.writeLock().unlock(); }
        }
    }
}
