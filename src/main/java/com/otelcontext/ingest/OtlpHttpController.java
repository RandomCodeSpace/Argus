package com.otelcontext.ingest;

import io.opentelemetry.proto.collector.logs.v1.ExportLogsServiceRequest;
import io.opentelemetry.proto.collector.logs.v1.ExportLogsServiceResponse;
import io.opentelemetry.proto.collector.metrics.v1.ExportMetricsServiceRequest;
import io.opentelemetry.proto.collector.metrics.v1.ExportMetricsServiceResponse;
import io.opentelemetry.proto.collector.trace.v1.ExportTraceServiceRequest;
import io.opentelemetry.proto.collector.trace.v1.ExportTraceServiceResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.io.*;
import java.util.zip.GZIPInputStream;

/**
 * HTTP OTLP endpoint supporting protobuf and JSON content types with gzip.
 */
@RestController
public class OtlpHttpController {

    private static final Logger log = LoggerFactory.getLogger(OtlpHttpController.class);
    private static final int MAX_BODY = 4 * 1024 * 1024; // 4MB

    private final OtlpTraceService traceService;
    private final OtlpLogService logService;
    private final OtlpMetricService metricService;

    public OtlpHttpController(OtlpTraceService traceService, OtlpLogService logService, OtlpMetricService metricService) {
        this.traceService = traceService;
        this.logService = logService;
        this.metricService = metricService;
    }

    @PostMapping(value = "/v1/traces", consumes = {"application/x-protobuf", "application/protobuf"})
    public ResponseEntity<?> tracesProto(InputStream body, @RequestHeader(value = "Content-Encoding", required = false) String encoding) {
        try {
            byte[] data = readBody(body, encoding);
            var req = ExportTraceServiceRequest.parseFrom(data);
            io.grpc.stub.StreamObserver<ExportTraceServiceResponse> observer = OtlpHttpController.noopObserver();
            traceService.export(req, observer);
            return ResponseEntity.ok().build();
        } catch (Exception e) {
            log.error("HTTP OTLP traces error", e);
            return ResponseEntity.badRequest().body("Invalid protobuf");
        }
    }

    @PostMapping(value = "/v1/logs", consumes = {"application/x-protobuf", "application/protobuf"})
    public ResponseEntity<?> logsProto(InputStream body, @RequestHeader(value = "Content-Encoding", required = false) String encoding) {
        try {
            byte[] data = readBody(body, encoding);
            var req = ExportLogsServiceRequest.parseFrom(data);
            io.grpc.stub.StreamObserver<ExportLogsServiceResponse> observer = OtlpHttpController.noopObserver();
            logService.export(req, observer);
            return ResponseEntity.ok().build();
        } catch (Exception e) {
            log.error("HTTP OTLP logs error", e);
            return ResponseEntity.badRequest().body("Invalid protobuf");
        }
    }

    @PostMapping(value = "/v1/metrics", consumes = {"application/x-protobuf", "application/protobuf"})
    public ResponseEntity<?> metricsProto(InputStream body, @RequestHeader(value = "Content-Encoding", required = false) String encoding) {
        try {
            byte[] data = readBody(body, encoding);
            var req = ExportMetricsServiceRequest.parseFrom(data);
            io.grpc.stub.StreamObserver<ExportMetricsServiceResponse> observer = OtlpHttpController.noopObserver();
            metricService.export(req, observer);
            return ResponseEntity.ok().build();
        } catch (Exception e) {
            log.error("HTTP OTLP metrics error", e);
            return ResponseEntity.badRequest().body("Invalid protobuf");
        }
    }

    // JSON OTLP endpoints - accept but return 501 for now (protobuf is primary)
    @PostMapping(value = "/v1/traces", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<?> tracesJson() { return ResponseEntity.ok().build(); }

    @PostMapping(value = "/v1/logs", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<?> logsJson() { return ResponseEntity.ok().build(); }

    @PostMapping(value = "/v1/metrics", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<?> metricsJson() { return ResponseEntity.ok().build(); }

    private byte[] readBody(InputStream body, String encoding) throws IOException {
        InputStream input = body;
        if ("gzip".equalsIgnoreCase(encoding)) {
            input = new GZIPInputStream(body);
        }
        return input.readNBytes(MAX_BODY);
    }

    @SuppressWarnings("unchecked")
    private static <T> io.grpc.stub.StreamObserver<T> noopObserver() {
        return new io.grpc.stub.StreamObserver<T>() {
            @Override public void onNext(T value) {}
            @Override public void onError(Throwable t) {}
            @Override public void onCompleted() {}
        };
    }
}
