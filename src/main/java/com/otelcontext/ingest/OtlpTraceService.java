package com.otelcontext.ingest;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.model.LogEntry;
import com.otelcontext.model.Span;
import com.otelcontext.model.Trace;
import com.otelcontext.realtime.WebSocketHub;
import com.otelcontext.repository.LogRepository;
import com.otelcontext.repository.SpanRepository;
import com.otelcontext.repository.TraceRepository;
import com.otelcontext.vectordb.TfIdfIndex;
import io.grpc.stub.StreamObserver;
import io.opentelemetry.proto.collector.trace.v1.*;
import io.opentelemetry.proto.trace.v1.Status;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

public class OtlpTraceService extends TraceServiceGrpc.TraceServiceImplBase {

    private static final Logger log = LoggerFactory.getLogger(OtlpTraceService.class);
    private static final ObjectMapper mapper = new ObjectMapper();

    private final TraceRepository traceRepo;
    private final SpanRepository spanRepo;
    private final LogRepository logRepo;
    private final GraphRAGService graphRAG;
    private final WebSocketHub wsHub;
    private final TfIdfIndex vectorIndex;
    private final Sampler sampler;

    public OtlpTraceService(TraceRepository traceRepo, SpanRepository spanRepo, LogRepository logRepo,
                            GraphRAGService graphRAG, WebSocketHub wsHub, TfIdfIndex vectorIndex, Sampler sampler) {
        this.traceRepo = traceRepo;
        this.spanRepo = spanRepo;
        this.logRepo = logRepo;
        this.graphRAG = graphRAG;
        this.wsHub = wsHub;
        this.vectorIndex = vectorIndex;
        this.sampler = sampler;
    }

    @Override
    public void export(ExportTraceServiceRequest request, StreamObserver<ExportTraceServiceResponse> responseObserver) {
        List<Span> spans = new ArrayList<>();
        List<Trace> traces = new ArrayList<>();
        List<LogEntry> logs = new ArrayList<>();

        for (var resourceSpans : request.getResourceSpansList()) {
            String serviceName = IngestUtils.getServiceName(resourceSpans.getResource().getAttributesList());

            for (var scopeSpans : resourceSpans.getScopeSpansList()) {
                for (var span : scopeSpans.getSpansList()) {
                    Instant startTime = Instant.ofEpochSecond(0, span.getStartTimeUnixNano());
                    Instant endTime = Instant.ofEpochSecond(0, span.getEndTimeUnixNano());
                    long duration = java.time.Duration.between(startTime, endTime).toNanos() / 1000;

                    String statusStr = "STATUS_CODE_UNSET";
                    if (span.hasStatus()) statusStr = span.getStatus().getCode().name();

                    boolean isError = "STATUS_CODE_ERROR".equals(statusStr);

                    if (sampler != null && !sampler.shouldSample(serviceName, isError, duration / 1000.0)) continue;

                    String attrs = "{}";
                    try { attrs = mapper.writeValueAsString(span.getAttributesList()); } catch (Exception ignored) {}

                    Span sModel = new Span();
                    sModel.setTraceId(IngestUtils.hexBytes(span.getTraceId()));
                    sModel.setSpanId(IngestUtils.hexBytes(span.getSpanId()));
                    sModel.setParentSpanId(IngestUtils.hexBytes(span.getParentSpanId()));
                    sModel.setOperationName(span.getName());
                    sModel.setStartTime(startTime);
                    sModel.setEndTime(endTime);
                    sModel.setDuration(duration);
                    sModel.setServiceName(serviceName);
                    sModel.setAttributesJson(attrs);
                    spans.add(sModel);

                    Trace tModel = new Trace();
                    tModel.setTraceId(IngestUtils.hexBytes(span.getTraceId()));
                    tModel.setServiceName(serviceName);
                    tModel.setTimestamp(startTime);
                    tModel.setDuration(duration);
                    tModel.setStatus(statusStr);
                    traces.add(tModel);

                    // Synthesize logs from span events
                    for (var event : span.getEventsList()) {
                        String severity = "exception".equals(event.getName()) ? "ERROR" : "INFO";
                        String body = event.getName();
                        for (var attr : event.getAttributesList()) {
                            if ("exception.message".equals(attr.getKey()) || "message".equals(attr.getKey())) {
                                body = attr.getValue().getStringValue();
                                break;
                            }
                        }
                        LogEntry l = new LogEntry();
                        l.setTraceId(sModel.getTraceId());
                        l.setSpanId(sModel.getSpanId());
                        l.setSeverity(severity);
                        l.setBody(body);
                        l.setServiceName(serviceName);
                        l.setAttributesJson("{}");
                        l.setTimestamp(Instant.ofEpochSecond(0, event.getTimeUnixNano()));
                        logs.add(l);
                    }

                    // Create error log if span has error status
                    if (isError) {
                        String msg = span.hasStatus() ? span.getStatus().getMessage() : "Span '" + span.getName() + "' failed";
                        if (msg.isEmpty()) msg = "Span '" + span.getName() + "' failed";
                        LogEntry errLog = new LogEntry();
                        errLog.setTraceId(sModel.getTraceId());
                        errLog.setSpanId(sModel.getSpanId());
                        errLog.setSeverity("ERROR");
                        errLog.setBody(msg);
                        errLog.setServiceName(serviceName);
                        errLog.setAttributesJson("{}");
                        errLog.setTimestamp(endTime);
                        logs.add(errLog);
                    }
                }
            }
        }

        // Persist
        if (!traces.isEmpty()) {
            try { traceRepo.saveAll(traces); } catch (Exception e) { log.error("Failed to save traces", e); }
        }
        if (!spans.isEmpty()) {
            try {
                spanRepo.saveAll(spans);
                for (var s : spans) graphRAG.onSpanIngested(s);
            } catch (Exception e) { log.error("Failed to save spans", e); }
        }
        if (!logs.isEmpty()) {
            try {
                logRepo.saveAll(logs);
                for (var l : logs) {
                    graphRAG.onLogIngested(l);
                    vectorIndex.add(l.getId() != null ? l.getId() : 0, l.getServiceName(), l.getSeverity(), l.getBody());
                }
            } catch (Exception e) { log.error("Failed to save logs", e); }
        }

        responseObserver.onNext(ExportTraceServiceResponse.getDefaultInstance());
        responseObserver.onCompleted();
    }
}
