package com.otelcontext.graphrag;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.otelcontext.graphrag.Schema.*;
import com.otelcontext.model.LogEntry;
import com.otelcontext.model.Span;
import com.otelcontext.repository.*;
import com.otelcontext.tsdb.RawMetric;
import com.otelcontext.tsdb.TsdbAggregator;
import com.otelcontext.vectordb.TfIdfIndex;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;

@Service
public class GraphRAGService {

    private static final Logger log = LoggerFactory.getLogger(GraphRAGService.class);
    private static final ObjectMapper mapper = new ObjectMapper();

    private final Stores.ServiceStore serviceStore = new Stores.ServiceStore();
    private final Stores.TraceStore traceStore = new Stores.TraceStore(Duration.ofHours(1));
    private final Stores.SignalStore signalStore = new Stores.SignalStore();
    private final Stores.AnomalyStore anomalyStore = new Stores.AnomalyStore();

    private final BlockingQueue<Runnable> eventQueue = new LinkedBlockingQueue<>(10000);
    private final SpanRepository spanRepo;
    private final InvestigationRepository investigationRepo;
    private final GraphSnapshotRepository snapshotRepo;

    public GraphRAGService(SpanRepository spanRepo, InvestigationRepository investigationRepo,
                           GraphSnapshotRepository snapshotRepo) {
        this.spanRepo = spanRepo;
        this.investigationRepo = investigationRepo;
        this.snapshotRepo = snapshotRepo;

        // Start 4 virtual thread event workers
        var executor = Executors.newVirtualThreadPerTaskExecutor();
        for (int i = 0; i < 4; i++) {
            executor.submit(this::eventWorker);
        }
    }

    private void eventWorker() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                Runnable task = eventQueue.poll(5, TimeUnit.SECONDS);
                if (task != null) task.run();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } catch (Exception e) {
                log.error("GraphRAG event worker error", e);
            }
        }
    }

    // --- Ingestion Callbacks ---

    public void onSpanIngested(Span span) {
        eventQueue.offer(() -> processSpan(span));
    }

    public void onLogIngested(LogEntry logEntry) {
        eventQueue.offer(() -> processLog(logEntry));
    }

    public void onMetricIngested(RawMetric metric) {
        eventQueue.offer(() -> processMetric(metric));
    }

    private void processSpan(Span span) {
        if (span.getServiceName() == null || span.getServiceName().isEmpty()) return;
        double durationMs = span.getDuration() / 1000.0;
        boolean isError = false; // simplified

        serviceStore.upsertService(span.getServiceName(), durationMs, isError, span.getStartTime());
        if (span.getOperationName() != null && !span.getOperationName().isEmpty()) {
            serviceStore.upsertOperation(span.getServiceName(), span.getOperationName(), durationMs, isError, span.getStartTime());
        }

        traceStore.upsertTrace(span.getTraceId(), span.getServiceName(), "OK", durationMs, span.getStartTime());
        traceStore.upsertSpan(new SpanNode(span.getSpanId(), span.getTraceId(), span.getParentSpanId(),
            span.getServiceName(), span.getOperationName(), durationMs, "OK", isError, span.getStartTime()));

        if (span.getParentSpanId() != null && !span.getParentSpanId().isEmpty()) {
            SpanNode parent = traceStore.getSpan(span.getParentSpanId());
            if (parent != null && !parent.service().equals(span.getServiceName())) {
                serviceStore.upsertCallEdge(parent.service(), span.getServiceName(), durationMs, isError, span.getStartTime());
            }
        }
    }

    private void processLog(LogEntry logEntry) {
        if (logEntry.getServiceName() == null || logEntry.getServiceName().isEmpty()) return;
        String body = logEntry.getBody() != null ? logEntry.getBody() : "";
        String clusterId = String.format("lc_%s_%x", logEntry.getServiceName(), simpleHash(body));
        signalStore.upsertLogCluster(clusterId, body, logEntry.getSeverity(), logEntry.getServiceName(), logEntry.getTimestamp());
    }

    private void processMetric(RawMetric m) {
        if (m.serviceName() == null || m.serviceName().isEmpty()) return;
        signalStore.upsertMetric(m.name(), m.serviceName(), m.value(), m.timestamp());
    }

    // --- Queries ---

    public List<ErrorChainResult> errorChain(String service, Instant since, int limit) {
        if (limit <= 0) limit = 10;
        var errorSpans = traceStore.errorSpans(service, since);
        if (errorSpans.size() > limit) errorSpans = errorSpans.subList(0, limit);

        List<ErrorChainResult> results = new ArrayList<>();
        Set<String> seen = new HashSet<>();
        for (var span : errorSpans) {
            if (seen.contains(span.traceId())) continue;
            seen.add(span.traceId());
            var chain = traceErrorChainUpstream(span);
            if (chain.isEmpty()) continue;
            var root = chain.getLast();
            results.add(new ErrorChainResult(
                new RootCauseInfo(root.service(), root.operation(), "", root.id(), root.traceId()),
                chain, List.of(), List.of(), span.traceId()
            ));
        }
        return results;
    }

    private List<SpanNode> traceErrorChainUpstream(SpanNode span) {
        List<SpanNode> chain = new ArrayList<>();
        Set<String> visited = new HashSet<>();
        SpanNode current = span;
        while (current != null && !visited.contains(current.id())) {
            visited.add(current.id());
            chain.add(current);
            if (current.parentSpanId() == null || current.parentSpanId().isEmpty()) break;
            current = traceStore.getSpan(current.parentSpanId());
        }
        return chain;
    }

    public ImpactResult impactAnalysis(String service, int maxDepth) {
        if (maxDepth <= 0) maxDepth = 5;
        List<AffectedEntry> affected = new ArrayList<>();
        Set<String> visited = new HashSet<>();
        visited.add(service);

        record QueueItem(String svc, int depth) {}
        Queue<QueueItem> queue = new LinkedList<>();
        queue.add(new QueueItem(service, 0));

        while (!queue.isEmpty()) {
            var item = queue.poll();
            if (item.depth >= maxDepth) continue;
            for (var edge : serviceStore.callEdgesFrom(item.svc)) {
                if (visited.contains(edge.toId())) continue;
                visited.add(edge.toId());
                var svc = serviceStore.getService(edge.toId());
                double impact = svc != null ? 1.0 - svc.healthScore() : 1.0;
                affected.add(new AffectedEntry(edge.toId(), item.depth + 1, edge.callCount(), impact));
                queue.add(new QueueItem(edge.toId(), item.depth + 1));
            }
        }
        return new ImpactResult(service, affected, affected.size());
    }

    public List<RankedCause> rootCauseAnalysis(String service, Instant since) {
        var chains = errorChain(service, since, 20);
        var anomalies = anomalyStore.anomaliesForService(service, since);
        Map<String, RankedCauseBuilder> scores = new HashMap<>();

        for (var ec : chains) {
            if (ec.rootCause() == null) continue;
            String key = ec.rootCause().service() + "|" + ec.rootCause().operation();
            var rc = scores.computeIfAbsent(key, k -> new RankedCauseBuilder(ec.rootCause().service(), ec.rootCause().operation()));
            rc.score += 1.0;
            rc.evidence.add("error chain from trace " + ec.traceId());
            if (!ec.spanChain().isEmpty()) rc.errorChain = ec.spanChain();
        }

        for (var a : anomalies) {
            for (var rc : scores.values()) {
                if (rc.service.equals(a.service())) {
                    rc.score += 2.0;
                    rc.anomalies.add(a);
                    rc.evidence.add("anomaly: " + a.evidence());
                }
            }
        }

        return scores.values().stream()
            .map(RankedCauseBuilder::build)
            .sorted((a, b) -> Double.compare(b.score(), a.score()))
            .toList();
    }

    public List<SpanNode> dependencyChain(String traceId) {
        var spans = traceStore.spansForTrace(traceId);
        return spans.stream().sorted(Comparator.comparing(SpanNode::timestamp)).toList();
    }

    public CorrelatedSignalsResult correlatedSignals(String service, Instant since) {
        var logs = signalStore.logClustersForService(service).stream()
            .filter(lc -> lc.lastSeen().isAfter(since)).toList();
        var metrics = signalStore.metricsForService(service);
        var anomalies = anomalyStore.anomaliesForService(service, since);
        var chains = errorChain(service, since, 5);
        return new CorrelatedSignalsResult(service, logs, metrics, anomalies, chains);
    }

    public List<String> shortestPath(String from, String to) {
        // Build adjacency
        Map<String, Map<String, Double>> adj = new HashMap<>();
        for (var edge : serviceStore.allEdges()) {
            if (edge.type() != EdgeType.CALLS) continue;
            double weight = edge.callCount() > 0 ? 1.0 / edge.callCount() : 1.0;
            adj.computeIfAbsent(edge.fromId(), k -> new HashMap<>()).put(edge.toId(), weight);
            adj.computeIfAbsent(edge.toId(), k -> new HashMap<>()).put(edge.fromId(), weight);
        }

        Map<String, Double> dist = new HashMap<>();
        Map<String, String> prev = new HashMap<>();
        Set<String> visited = new HashSet<>();
        dist.put(from, 0.0);

        while (true) {
            String u = null;
            double minDist = Double.MAX_VALUE;
            for (var e : dist.entrySet()) {
                if (!visited.contains(e.getKey()) && e.getValue() < minDist) {
                    u = e.getKey();
                    minDist = e.getValue();
                }
            }
            if (u == null || u.equals(to)) break;
            visited.add(u);

            var neighbors = adj.getOrDefault(u, Map.of());
            for (var e : neighbors.entrySet()) {
                double alt = dist.get(u) + e.getValue();
                if (alt < dist.getOrDefault(e.getKey(), Double.MAX_VALUE)) {
                    dist.put(e.getKey(), alt);
                    prev.put(e.getKey(), u);
                }
            }
        }

        if (!dist.containsKey(to)) return List.of();
        List<String> path = new ArrayList<>();
        for (String at = to; at != null; at = prev.get(at)) {
            path.addFirst(at);
            if (at.equals(from)) break;
        }
        return path.isEmpty() || !path.getFirst().equals(from) ? List.of() : path;
    }

    public List<AnomalyNode> anomalyTimeline(Instant since) {
        return anomalyStore.anomaliesSince(since).stream()
            .sorted((a, b) -> b.timestamp().compareTo(a.timestamp()))
            .toList();
    }

    public List<ServiceMapEntry> serviceMap(int depth) {
        var services = serviceStore.allServices();
        List<ServiceMapEntry> result = new ArrayList<>();
        for (var svc : services) {
            result.add(new ServiceMapEntry(
                svc,
                serviceStore.operationsForService(svc.name()),
                serviceStore.callEdgesFrom(svc.name()),
                serviceStore.callEdgesTo(svc.name())
            ));
        }
        return result;
    }

    // --- Background tasks ---

    @Scheduled(fixedRate = 60000) // 60s refresh
    public void refresh() {
        try {
            rebuildFromDB();
            int pruned = traceStore.prune();
            if (pruned > 0) log.debug("GraphRAG pruned {} spans", pruned);
            anomalyStore.pruneOlderThan(Instant.now().minus(Duration.ofHours(24)));
        } catch (Exception e) { log.error("GraphRAG refresh error", e); }
    }

    @Scheduled(fixedRate = 900000) // 15min snapshot
    public void snapshot() {
        try {
            takeSnapshot();
            snapshotRepo.deleteOlderThan(Instant.now().minus(Duration.ofDays(7)));
        } catch (Exception e) { log.error("GraphRAG snapshot error", e); }
    }

    @Scheduled(fixedRate = 10000) // 10s anomaly detection
    public void detectAnomalies() {
        try {
            var services = serviceStore.allServices();
            Instant now = Instant.now();
            for (var svc : services) {
                double baselineError = 0.02;
                if (svc.errorRate() > baselineError * 2 && svc.errorRate() > 0.05) {
                    var anomaly = new AnomalyNode(
                        "anom_" + svc.name() + "_err_" + now.toEpochMilli(),
                        AnomalyType.error_spike,
                        classifyErrorSeverity(svc.errorRate()),
                        svc.name(),
                        String.format("error rate %.1f%% (baseline ~%.1f%%)", svc.errorRate() * 100, baselineError * 100),
                        now
                    );
                    anomalyStore.addAnomaly(anomaly);
                    correlateWithRecent(anomaly);
                }
                if (svc.avgLatency() > 500 && svc.callCount() > 10) {
                    var anomaly = new AnomalyNode(
                        "anom_" + svc.name() + "_lat_" + now.toEpochMilli(),
                        AnomalyType.latency_spike,
                        classifyLatencySeverity(svc.avgLatency()),
                        svc.name(),
                        String.format("avg latency %.0fms", svc.avgLatency()),
                        now
                    );
                    anomalyStore.addAnomaly(anomaly);
                    correlateWithRecent(anomaly);
                }
            }

            // Metric z-score
            for (var m : signalStore.allMetricsMutable()) {
                if (m.sampleCount < 10) continue;
                double range = m.rollingMax - m.rollingMin;
                if (range > 0) {
                    double deviation = (m.rollingAvg - (m.rollingMin + range / 2)) / (range / 2);
                    if (Math.abs(deviation) > 3.0) {
                        var anomaly = new AnomalyNode(
                            "anom_" + m.service + "_metric_" + now.toEpochMilli(),
                            AnomalyType.metric_zscore,
                            AnomalySeverity.warning,
                            m.service,
                            String.format("metric %s z-score %.1f", m.metricName, deviation),
                            now
                        );
                        anomalyStore.addAnomaly(anomaly);
                    }
                }
            }
        } catch (Exception e) { log.error("Anomaly detection error", e); }
    }

    private void correlateWithRecent(AnomalyNode anomaly) {
        var recent = anomalyStore.anomaliesSince(anomaly.timestamp().minus(Duration.ofSeconds(30)));
        for (var prev : recent) {
            if (!prev.id().equals(anomaly.id())) {
                anomalyStore.addPrecededByEdge(anomaly.id(), prev.id(), anomaly.timestamp());
            }
        }
    }

    private void rebuildFromDB() {
        Instant since = Instant.now().minus(Duration.ofHours(1));
        var spans = spanRepo.findRecentSpans(since);
        if (spans.isEmpty()) return;

        Map<String, String> spanService = new HashMap<>();
        for (var s : spans) spanService.put(s.getSpanId(), s.getServiceName());

        for (var s : spans) {
            double durationMs = s.getDuration() / 1000.0;
            boolean isError = false;
            serviceStore.upsertService(s.getServiceName(), durationMs, isError, s.getStartTime());
            if (s.getOperationName() != null && !s.getOperationName().isEmpty()) {
                serviceStore.upsertOperation(s.getServiceName(), s.getOperationName(), durationMs, isError, s.getStartTime());
            }
            if (s.getParentSpanId() != null && !s.getParentSpanId().isEmpty()) {
                String parentSvc = spanService.get(s.getParentSpanId());
                if (parentSvc != null && !parentSvc.equals(s.getServiceName())) {
                    serviceStore.upsertCallEdge(parentSvc, s.getServiceName(), durationMs, isError, s.getStartTime());
                }
            }
        }
        log.debug("GraphRAG rebuilt from DB: {} spans", spans.size());
    }

    private void takeSnapshot() {
        var services = serviceStore.allServices();
        if (services.isEmpty()) return;
        var edges = serviceStore.allEdges();

        try {
            String nodesJson = mapper.writeValueAsString(services);
            String edgesJson = mapper.writeValueAsString(edges);

            double totalHealth = services.stream().mapToDouble(ServiceNode::healthScore).sum();
            long totalCalls = services.stream().mapToLong(ServiceNode::callCount).sum();

            var snap = new com.otelcontext.model.GraphSnapshot();
            snap.setId("snap_" + System.nanoTime());
            snap.setCreatedAt(Instant.now());
            snap.setNodes(nodesJson);
            snap.setEdges(edgesJson);
            snap.setServiceCount(services.size());
            snap.setTotalCalls(totalCalls);
            snap.setAvgHealthScore(totalHealth / services.size());
            snapshotRepo.save(snap);
        } catch (Exception e) { log.error("Snapshot error", e); }
    }

    // --- Accessors ---
    public Stores.ServiceStore getServiceStore() { return serviceStore; }
    public Stores.TraceStore getTraceStore() { return traceStore; }
    public Stores.SignalStore getSignalStore() { return signalStore; }
    public Stores.AnomalyStore getAnomalyStore() { return anomalyStore; }

    private static AnomalySeverity classifyErrorSeverity(double errorRate) {
        if (errorRate > 0.2) return AnomalySeverity.critical;
        if (errorRate > 0.1) return AnomalySeverity.warning;
        return AnomalySeverity.info;
    }

    private static AnomalySeverity classifyLatencySeverity(double avgMs) {
        if (avgMs > 2000) return AnomalySeverity.critical;
        if (avgMs > 1000) return AnomalySeverity.warning;
        return AnomalySeverity.info;
    }

    private static int simpleHash(String s) {
        int h = 0;
        for (char c : s.toCharArray()) h = h * 31 + c;
        return h;
    }

    private static class RankedCauseBuilder {
        String service, operation;
        double score;
        List<String> evidence = new ArrayList<>();
        List<SpanNode> errorChain = List.of();
        List<AnomalyNode> anomalies = new ArrayList<>();

        RankedCauseBuilder(String service, String operation) {
            this.service = service;
            this.operation = operation;
        }

        RankedCause build() {
            return new RankedCause(service, operation, score, List.copyOf(evidence), List.copyOf(errorChain), List.copyOf(anomalies));
        }
    }
}
