package com.otelcontext.graphrag;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/** Node and edge type definitions for the layered graph. */
public final class Schema {

    private Schema() {}

    // --- Node Types ---
    public record ServiceNode(String id, String name, Instant firstSeen, Instant lastSeen,
                              double healthScore, long callCount, long errorCount, double errorRate,
                              double avgLatency, double totalMs) {}

    public record OperationNode(String id, String service, String operation, Instant firstSeen, Instant lastSeen,
                                double healthScore, long callCount, long errorCount, double errorRate,
                                double avgLatency, double totalMs) {}

    public record TraceNode(String id, String rootService, double durationMs, String status,
                            Instant timestamp, int spanCount) {}

    public record SpanNode(String id, String traceId, String parentSpanId, String service,
                           String operation, double durationMs, String statusCode, boolean isError,
                           Instant timestamp) {}

    public record LogClusterNode(String id, String template, long count, Instant firstSeen,
                                 Instant lastSeen, Map<String, Long> severityDist) {}

    public record MetricNode(String id, String metricName, String service, double rollingMin,
                             double rollingMax, double rollingAvg, long sampleCount, Instant lastSeen) {}

    public enum AnomalySeverity { critical, warning, info }
    public enum AnomalyType { error_spike, latency_spike, metric_zscore }

    public record AnomalyNode(String id, AnomalyType type, AnomalySeverity severity,
                               String service, String evidence, Instant timestamp) {}

    // --- Edge Types ---
    public enum EdgeType {
        CALLS, EXPOSES, CONTAINS, CHILD_OF, EMITTED_BY, LOGGED_DURING, MEASURED_BY, PRECEDED_BY, TRIGGERED_BY
    }

    public record Edge(EdgeType type, String fromId, String toId, double weight,
                       long callCount, double errorRate, double avgMs, double totalMs,
                       long errorCount, Instant updatedAt) {}

    // --- Query Result Types ---
    public record RootCauseInfo(String service, String operation, String errorMessage, String spanId, String traceId) {}

    public record ErrorChainResult(RootCauseInfo rootCause, List<SpanNode> spanChain,
                                   List<LogClusterNode> correlatedLogs, List<MetricNode> anomalousMetrics,
                                   String traceId) {}

    public record AffectedEntry(String service, int depth, long callCount, double impactScore) {}
    public record ImpactResult(String service, List<AffectedEntry> affectedServices, int totalDownstream) {}

    public record RankedCause(String service, String operation, double score, List<String> evidence,
                              List<SpanNode> errorChain, List<AnomalyNode> anomalies) {}

    public record CorrelatedSignalsResult(String service, List<LogClusterNode> errorLogs,
                                          List<MetricNode> metrics, List<AnomalyNode> anomalies,
                                          List<ErrorChainResult> errorChains) {}

    public record ServiceMapEntry(ServiceNode service, List<OperationNode> operations,
                                  List<Edge> callsTo, List<Edge> calledBy) {}

    public static double computeHealth(double errorRate, double avgLatencyMs) {
        double latencyDev = Math.max(0, (avgLatencyMs - 100) / 100);
        double score = 1.0 - (errorRate * 5) - (latencyDev * 0.1);
        return Math.max(0, Math.min(1, score));
    }

    public static String edgeKey(EdgeType type, String from, String to) {
        return type.name() + "|" + from + "|" + to;
    }
}
