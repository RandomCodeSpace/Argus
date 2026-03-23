package com.otelcontext.ingest;

import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.realtime.WebSocketHub;
import com.otelcontext.tsdb.RawMetric;
import com.otelcontext.tsdb.TsdbAggregator;
import io.grpc.stub.StreamObserver;
import io.opentelemetry.proto.collector.metrics.v1.*;
import io.opentelemetry.proto.metrics.v1.NumberDataPoint;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

public class OtlpMetricService extends MetricsServiceGrpc.MetricsServiceImplBase {

    private static final Logger log = LoggerFactory.getLogger(OtlpMetricService.class);

    private final TsdbAggregator tsdb;
    private final GraphRAGService graphRAG;
    private final WebSocketHub wsHub;

    public OtlpMetricService(TsdbAggregator tsdb, GraphRAGService graphRAG, WebSocketHub wsHub) {
        this.tsdb = tsdb;
        this.graphRAG = graphRAG;
        this.wsHub = wsHub;
    }

    @Override
    public void export(ExportMetricsServiceRequest request, StreamObserver<ExportMetricsServiceResponse> responseObserver) {
        for (var resourceMetrics : request.getResourceMetricsList()) {
            String serviceName = IngestUtils.getServiceName(resourceMetrics.getResource().getAttributesList());

            for (var scopeMetrics : resourceMetrics.getScopeMetricsList()) {
                for (var metric : scopeMetrics.getMetricsList()) {
                    java.util.List<NumberDataPoint> points = new java.util.ArrayList<>();

                    if (metric.hasGauge()) points.addAll(metric.getGauge().getDataPointsList());
                    else if (metric.hasSum()) points.addAll(metric.getSum().getDataPointsList());

                    for (var p : points) {
                        double val = p.hasAsDouble() ? p.getAsDouble() : p.getAsInt();
                        Map<String, Object> attrs = new HashMap<>();
                        for (var kv : p.getAttributesList()) {
                            attrs.put(kv.getKey(), kv.getValue().getStringValue());
                        }

                        var raw = new RawMetric(metric.getName(), serviceName, val,
                            Instant.ofEpochSecond(0, p.getTimeUnixNano()), attrs);

                        if (tsdb != null) tsdb.ingest(raw);
                        graphRAG.onMetricIngested(raw);
                        wsHub.broadcastMetric(raw);
                    }
                }
            }
        }

        responseObserver.onNext(ExportMetricsServiceResponse.getDefaultInstance());
        responseObserver.onCompleted();
    }
}
