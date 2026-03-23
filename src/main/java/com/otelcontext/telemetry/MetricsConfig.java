package com.otelcontext.telemetry;

import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.Gauge;
import com.otelcontext.realtime.WebSocketHub;
import com.otelcontext.tsdb.TsdbAggregator;
import org.springframework.context.annotation.Configuration;

/**
 * Prometheus metrics registration via Micrometer.
 */
@Configuration
public class MetricsConfig {

    public MetricsConfig(MeterRegistry registry, WebSocketHub wsHub, TsdbAggregator tsdb) {
        // Ingestion metrics
        Counter.builder("otelcontext.ingestion.total").description("Total ingested items").register(registry);
        Counter.builder("otelcontext.grpc.requests.total").description("gRPC requests").tag("method", "export").tag("status", "ok").register(registry);

        // WebSocket
        Gauge.builder("otelcontext.ws.connections", wsHub, WebSocketHub::getConnectionCount)
            .description("Active WebSocket connections").register(registry);

        // TSDB
        Gauge.builder("otelcontext.tsdb.buckets", tsdb, TsdbAggregator::bucketCount)
            .description("TSDB in-memory buckets").register(registry);
    }
}
