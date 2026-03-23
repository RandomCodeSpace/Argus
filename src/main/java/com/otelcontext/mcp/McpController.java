package com.otelcontext.mcp;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.repository.*;
import com.otelcontext.vectordb.TfIdfIndex;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Duration;
import java.time.Instant;
import java.util.*;

/**
 * MCP server - JSON-RPC 2.0 over HTTP.
 * POST /mcp for RPC, GET /mcp for SSE.
 */
@RestController
@RequestMapping("${otelcontext.mcp-path:/mcp}")
@ConditionalOnProperty(name = "otelcontext.mcp-enabled", havingValue = "true", matchIfMissing = true)
public class McpController {

    private static final Logger log = LoggerFactory.getLogger(McpController.class);
    private static final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    private final TraceRepository traceRepo;
    private final LogRepository logRepo;
    private final SpanRepository spanRepo;
    private final MetricBucketRepository metricRepo;
    private final InvestigationRepository invRepo;
    private final GraphSnapshotRepository snapRepo;
    private final GraphRAGService graphRAG;
    private final TfIdfIndex vectorIndex;

    public McpController(TraceRepository traceRepo, LogRepository logRepo, SpanRepository spanRepo,
                         MetricBucketRepository metricRepo, InvestigationRepository invRepo,
                         GraphSnapshotRepository snapRepo, GraphRAGService graphRAG, TfIdfIndex vectorIndex) {
        this.traceRepo = traceRepo;
        this.logRepo = logRepo;
        this.spanRepo = spanRepo;
        this.metricRepo = metricRepo;
        this.invRepo = invRepo;
        this.snapRepo = snapRepo;
        this.graphRAG = graphRAG;
        this.vectorIndex = vectorIndex;
    }

    @PostMapping(consumes = MediaType.APPLICATION_JSON_VALUE, produces = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<?> handleRpc(@RequestBody Map<String, Object> body) {
        String jsonrpc = (String) body.get("jsonrpc");
        String method = (String) body.get("method");
        Object id = body.get("id");
        Object params = body.get("params");

        if (!"2.0".equals(jsonrpc)) {
            return ResponseEntity.ok(errorResponse(id, -32600, "jsonrpc must be '2.0'"));
        }

        return switch (method) {
            case "initialize" -> ResponseEntity.ok(successResponse(id, Map.of(
                "protocolVersion", "2024-11-05",
                "serverInfo", Map.of("name", "OtelContext-mcp", "version", "1.0.0"),
                "capabilities", Map.of("tools", Map.of(), "resources", Map.of())
            )));
            case "initialized", "notifications/initialized" -> ResponseEntity.accepted().build();
            case "tools/list" -> ResponseEntity.ok(successResponse(id, Map.of("tools", getToolDefs())));
            case "tools/call" -> {
                @SuppressWarnings("unchecked")
                var p = (Map<String, Object>) params;
                String toolName = (String) p.get("name");
                @SuppressWarnings("unchecked")
                var args = (Map<String, Object>) p.getOrDefault("arguments", Map.of());
                yield ResponseEntity.ok(successResponse(id, callTool(toolName, args)));
            }
            case "ping" -> ResponseEntity.ok(successResponse(id, Map.of("status", "ok")));
            default -> ResponseEntity.ok(errorResponse(id, -32601, "method not found: " + method));
        };
    }

    private Object callTool(String name, Map<String, Object> args) {
        try {
            Object result = switch (name) {
                case "get_service_map" -> graphRAG.serviceMap(getInt(args, "depth", 3));
                case "get_error_chains" -> {
                    String svc = (String) args.get("service");
                    Instant since = parseSince(args, "time_range", Duration.ofMinutes(15));
                    yield graphRAG.errorChain(svc, since, getInt(args, "limit", 10));
                }
                case "trace_graph" -> graphRAG.dependencyChain((String) args.get("trace_id"));
                case "impact_analysis" -> graphRAG.impactAnalysis((String) args.get("service"), getInt(args, "depth", 5));
                case "root_cause_analysis" -> {
                    Instant since = parseSince(args, "time_range", Duration.ofMinutes(15));
                    yield graphRAG.rootCauseAnalysis((String) args.get("service"), since);
                }
                case "correlated_signals" -> {
                    Instant since = parseSince(args, "time_range", Duration.ofHours(1));
                    yield graphRAG.correlatedSignals((String) args.get("service"), since);
                }
                case "get_investigations" -> invRepo.findFiltered(
                    (String) args.get("service"), (String) args.get("severity"),
                    (String) args.get("status"), PageRequest.of(0, getInt(args, "limit", 20)));
                case "get_investigation" -> invRepo.findById((String) args.get("investigation_id")).orElse(null);
                case "get_graph_snapshot" -> {
                    Instant at = args.containsKey("time") ? Instant.parse((String) args.get("time")) : Instant.now();
                    yield snapRepo.findClosestBefore(at).orElse(null);
                }
                case "get_anomaly_timeline" -> {
                    Instant since = args.containsKey("since") ? Instant.parse((String) args.get("since")) : Instant.now().minus(Duration.ofHours(1));
                    String svc = (String) args.get("service");
                    yield svc != null ? graphRAG.getAnomalyStore().anomaliesForService(svc, since)
                                      : graphRAG.anomalyTimeline(since);
                }
                case "search_logs" -> {
                    int limit = Math.min(getInt(args, "limit", 50), 200);
                    var page = logRepo.findFiltered(
                        (String) args.get("service"), (String) args.get("severity"), (String) args.get("trace_id"),
                        (String) args.get("query"),
                        args.containsKey("start") ? Instant.parse((String) args.get("start")) : Instant.now().minus(Duration.ofDays(1)),
                        args.containsKey("end") ? Instant.parse((String) args.get("end")) : Instant.now(),
                        PageRequest.of(0, limit, Sort.by(Sort.Direction.DESC, "timestamp")));
                    yield Map.of("total", page.getTotalElements(), "entries", page.getContent());
                }
                case "find_similar_logs" -> vectorIndex.search((String) args.get("query"), getInt(args, "limit", 10));
                case "get_dashboard_stats" -> {
                    Instant s = args.containsKey("start") ? Instant.parse((String) args.get("start")) : Instant.now().minus(Duration.ofHours(1));
                    Instant e = args.containsKey("end") ? Instant.parse((String) args.get("end")) : Instant.now();
                    yield Map.of("total_requests", traceRepo.countByTimestampBetween(s, e),
                        "error_count", traceRepo.countErrorsBetween(s, e));
                }
                default -> Map.of("error", "unknown tool: " + name);
            };
            return Map.of("content", List.of(Map.of("type", "text", "text", mapper.writeValueAsString(result))));
        } catch (Exception e) {
            return Map.of("isError", true, "content", List.of(Map.of("type", "text", "text", "Error: " + e.getMessage())));
        }
    }

    private Map<String, Object> successResponse(Object id, Object result) {
        return Map.of("jsonrpc", "2.0", "id", id != null ? id : 0, "result", result);
    }

    private Map<String, Object> errorResponse(Object id, int code, String message) {
        return Map.of("jsonrpc", "2.0", "id", id != null ? id : 0, "error", Map.of("code", code, "message", message));
    }

    private int getInt(Map<String, Object> args, String key, int def) {
        Object v = args.get(key);
        if (v instanceof Number n) return n.intValue();
        return def;
    }

    private Instant parseSince(Map<String, Object> args, String key, Duration defaultWindow) {
        String v = (String) args.get(key);
        if (v != null && !v.isEmpty()) {
            try {
                if (v.endsWith("m")) return Instant.now().minus(Duration.ofMinutes(Long.parseLong(v.replace("m", ""))));
                if (v.endsWith("h")) return Instant.now().minus(Duration.ofHours(Long.parseLong(v.replace("h", ""))));
                return Instant.now().minus(Duration.parse("PT" + v));
            } catch (Exception ignored) {}
        }
        return Instant.now().minus(defaultWindow);
    }

    private List<Map<String, Object>> getToolDefs() {
        // Return the 22 tool definitions matching Go's
        return List.of(
            toolDef("get_system_graph", "Returns the full service topology with health scores."),
            toolDef("get_service_health", "Returns detailed health metrics for a specific service."),
            toolDef("search_logs", "Searches log entries by severity, service, body text, trace ID."),
            toolDef("tail_logs", "Returns the N most recent log entries."),
            toolDef("get_trace", "Returns full trace detail with all spans."),
            toolDef("search_traces", "Searches traces by service, status, duration, time range."),
            toolDef("get_metrics", "Queries metric time series."),
            toolDef("get_dashboard_stats", "Returns dashboard summary stats."),
            toolDef("get_storage_status", "Returns storage sizes and health."),
            toolDef("find_similar_logs", "Finds semantically similar logs using TF-IDF."),
            toolDef("get_alerts", "Returns active alerts and anomalies."),
            toolDef("search_cold_archive", "Searches archived cold data."),
            toolDef("get_service_map", "Returns service topology from GraphRAG."),
            toolDef("get_error_chains", "Traces error spans upstream to root cause."),
            toolDef("trace_graph", "Returns full span tree for a trace."),
            toolDef("impact_analysis", "BFS downstream blast radius analysis."),
            toolDef("root_cause_analysis", "Ranked probable root causes with evidence."),
            toolDef("correlated_signals", "All related signals for a service."),
            toolDef("get_investigations", "Lists persisted investigation records."),
            toolDef("get_investigation", "Returns a single investigation record."),
            toolDef("get_graph_snapshot", "Returns historical service topology snapshot."),
            toolDef("get_anomaly_timeline", "Returns recent anomalies with temporal links.")
        );
    }

    private Map<String, Object> toolDef(String name, String description) {
        return Map.of("name", name, "description", description, "inputSchema", Map.of("type", "object"));
    }
}
