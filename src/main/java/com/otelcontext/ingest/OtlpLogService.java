package com.otelcontext.ingest;

import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.model.LogEntry;
import com.otelcontext.realtime.WebSocketHub;
import com.otelcontext.repository.LogRepository;
import com.otelcontext.vectordb.TfIdfIndex;
import io.grpc.stub.StreamObserver;
import io.opentelemetry.proto.collector.logs.v1.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

public class OtlpLogService extends LogsServiceGrpc.LogsServiceImplBase {

    private static final Logger log = LoggerFactory.getLogger(OtlpLogService.class);

    private final LogRepository logRepo;
    private final GraphRAGService graphRAG;
    private final WebSocketHub wsHub;
    private final TfIdfIndex vectorIndex;

    public OtlpLogService(LogRepository logRepo, GraphRAGService graphRAG, WebSocketHub wsHub, TfIdfIndex vectorIndex) {
        this.logRepo = logRepo;
        this.graphRAG = graphRAG;
        this.wsHub = wsHub;
        this.vectorIndex = vectorIndex;
    }

    @Override
    public void export(ExportLogsServiceRequest request, StreamObserver<ExportLogsServiceResponse> responseObserver) {
        List<LogEntry> logsToInsert = new ArrayList<>();

        for (var resourceLogs : request.getResourceLogsList()) {
            String serviceName = IngestUtils.getServiceName(resourceLogs.getResource().getAttributesList());

            for (var scopeLogs : resourceLogs.getScopeLogsList()) {
                for (var lr : scopeLogs.getLogRecordsList()) {
                    String severity = lr.getSeverityText();
                    if (severity.isEmpty()) severity = lr.getSeverityNumber().name();

                    Instant timestamp = Instant.ofEpochSecond(0, lr.getTimeUnixNano());
                    if (timestamp.getEpochSecond() == 0) timestamp = Instant.now();

                    LogEntry entry = new LogEntry();
                    entry.setTraceId(IngestUtils.hexBytes(lr.getTraceId()));
                    entry.setSpanId(IngestUtils.hexBytes(lr.getSpanId()));
                    entry.setSeverity(severity);
                    entry.setBody(lr.getBody().getStringValue());
                    entry.setServiceName(serviceName);
                    entry.setAttributesJson("{}");
                    entry.setTimestamp(timestamp);
                    logsToInsert.add(entry);
                }
            }
        }

        if (!logsToInsert.isEmpty()) {
            try {
                logRepo.saveAll(logsToInsert);
                for (var l : logsToInsert) {
                    graphRAG.onLogIngested(l);
                    vectorIndex.add(l.getId() != null ? l.getId() : 0, l.getServiceName(), l.getSeverity(), l.getBody());
                    wsHub.broadcastLog(l);
                }
            } catch (Exception e) { log.error("Failed to save logs", e); }
        }

        responseObserver.onNext(ExportLogsServiceResponse.getDefaultInstance());
        responseObserver.onCompleted();
    }
}
