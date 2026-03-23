package com.otelcontext.api;

import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.repository.*;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.*;

@RestController
@RequestMapping("/api")
public class MetricsController {

    private final MetricBucketRepository metricRepo;
    private final TraceRepository traceRepo;
    private final LogRepository logRepo;
    private final SpanRepository spanRepo;
    private final GraphRAGService graphRAG;

    public MetricsController(MetricBucketRepository metricRepo, TraceRepository traceRepo,
                             LogRepository logRepo, SpanRepository spanRepo, GraphRAGService graphRAG) {
        this.metricRepo = metricRepo;
        this.traceRepo = traceRepo;
        this.logRepo = logRepo;
        this.spanRepo = spanRepo;
        this.graphRAG = graphRAG;
    }

    @GetMapping("/metrics")
    public ResponseEntity<?> getMetrics(
            @RequestParam(required = false) String name,
            @RequestParam(required = false) String service,
            @RequestParam(required = false) String start,
            @RequestParam(required = false) String end) {
        Instant s = start != null ? Instant.parse(start) : Instant.now().minusSeconds(3600);
        Instant e = end != null ? Instant.parse(end) : Instant.now();
        return ResponseEntity.ok(metricRepo.findFiltered(name, service, s, e));
    }

    @GetMapping("/metrics/traffic")
    public ResponseEntity<?> getTraffic(@RequestParam(required = false) String start, @RequestParam(required = false) String end) {
        Instant s = start != null ? Instant.parse(start) : Instant.now().minusSeconds(3600);
        Instant e = end != null ? Instant.parse(end) : Instant.now();
        return ResponseEntity.ok(Map.of(
            "total_requests", traceRepo.countByTimestampBetween(s, e),
            "error_count", traceRepo.countErrorsBetween(s, e)
        ));
    }

    @GetMapping("/metrics/latency_heatmap")
    public ResponseEntity<?> getLatencyHeatmap(@RequestParam(required = false) String start, @RequestParam(required = false) String end) {
        Instant s = start != null ? Instant.parse(start) : Instant.now().minusSeconds(3600);
        Instant e = end != null ? Instant.parse(end) : Instant.now();
        return ResponseEntity.ok(Map.of("avg_duration_us", traceRepo.avgDurationBetween(s, e).orElse(0.0)));
    }

    @GetMapping("/metrics/dashboard")
    public ResponseEntity<?> getDashboard(@RequestParam(required = false) String start, @RequestParam(required = false) String end) {
        Instant s = start != null ? Instant.parse(start) : Instant.now().minusSeconds(3600);
        Instant e = end != null ? Instant.parse(end) : Instant.now();

        long total = traceRepo.countByTimestampBetween(s, e);
        long errors = traceRepo.countErrorsBetween(s, e);
        double avgDuration = traceRepo.avgDurationBetween(s, e).orElse(0.0);
        long logCount = logRepo.countByTimestampBetween(s, e);

        return ResponseEntity.ok(Map.of(
            "total_requests", total,
            "error_count", errors,
            "error_rate", total > 0 ? (double) errors / total : 0,
            "avg_duration_us", avgDuration,
            "log_count", logCount,
            "services", traceRepo.findDistinctServiceNames()
        ));
    }

    @GetMapping("/metrics/service-map")
    public ResponseEntity<?> getServiceMap() {
        return ResponseEntity.ok(graphRAG.serviceMap(3));
    }

    @GetMapping("/metadata/services")
    public ResponseEntity<?> getServices() {
        Set<String> services = new TreeSet<>();
        services.addAll(traceRepo.findDistinctServiceNames());
        services.addAll(spanRepo.findDistinctServiceNames());
        services.addAll(logRepo.findDistinctServiceNames());
        return ResponseEntity.ok(services);
    }

    @GetMapping("/metadata/metrics")
    public ResponseEntity<?> getMetricNames() {
        return ResponseEntity.ok(metricRepo.findDistinctNames());
    }

    @GetMapping("/system/graph")
    public ResponseEntity<?> getSystemGraph() {
        return ResponseEntity.ok()
            .header("X-Cache", "miss")
            .body(graphRAG.serviceMap(3));
    }

    @GetMapping("/stats")
    public ResponseEntity<?> getStats() {
        return ResponseEntity.ok(Map.of(
            "traces", traceRepo.count(),
            "spans", spanRepo.count(),
            "logs", logRepo.count(),
            "metrics", metricRepo.count()
        ));
    }

    @GetMapping("/health")
    public ResponseEntity<?> health() {
        return ResponseEntity.ok(Map.of("status", "ok", "timestamp", Instant.now()));
    }

    @DeleteMapping("/admin/purge")
    public ResponseEntity<?> purge(@RequestParam(defaultValue = "7") int days) {
        Instant cutoff = Instant.now().minus(java.time.Duration.ofDays(days));
        int traces = traceRepo.deleteOlderThan(cutoff);
        int logs = logRepo.deleteOlderThan(cutoff);
        int metrics = metricRepo.deleteOlderThan(cutoff);
        return ResponseEntity.ok(Map.of("purged_traces", traces, "purged_logs", logs, "purged_metrics", metrics));
    }

    @PostMapping("/admin/vacuum")
    public ResponseEntity<?> vacuum() {
        return ResponseEntity.ok(Map.of("status", "vacuum not needed for H2"));
    }

    @GetMapping("/archive/search")
    public ResponseEntity<?> searchArchive(
            @RequestParam String type,
            @RequestParam String start,
            @RequestParam String end) {
        return ResponseEntity.ok(Map.of("source", "cold", "type", type, "start", start, "end", end,
            "message", "Cold archive search not yet implemented"));
    }
}
