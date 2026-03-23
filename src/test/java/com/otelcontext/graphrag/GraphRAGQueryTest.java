package com.otelcontext.graphrag;

import com.otelcontext.graphrag.Schema.*;
import com.otelcontext.graphrag.Stores.*;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.time.Instant;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class GraphRAGQueryTest {

    @Test
    void testServiceStoreUpsert() {
        var store = new ServiceStore();
        Instant now = Instant.now();

        store.upsertService("svc-a", 100.0, false, now);
        store.upsertService("svc-a", 200.0, true, now.plusSeconds(1));

        var svc = store.getService("svc-a");
        assertNotNull(svc);
        assertEquals("svc-a", svc.name());
        assertEquals(2, svc.callCount());
        assertEquals(1, svc.errorCount());
        assertEquals(0.5, svc.errorRate());
        assertTrue(svc.healthScore() >= 0 && svc.healthScore() <= 1);
    }

    @Test
    void testCallEdges() {
        var store = new ServiceStore();
        Instant now = Instant.now();

        store.upsertService("api-gateway", 50, false, now);
        store.upsertService("user-service", 100, false, now);
        store.upsertCallEdge("api-gateway", "user-service", 100, false, now);

        var edges = store.callEdgesFrom("api-gateway");
        assertEquals(1, edges.size());
        assertEquals("user-service", edges.getFirst().toId());

        var incoming = store.callEdgesTo("user-service");
        assertEquals(1, incoming.size());
    }

    @Test
    void testTraceStorePrune() {
        var store = new TraceStore(Duration.ofMillis(100));
        Instant old = Instant.now().minus(Duration.ofSeconds(10));
        Instant recent = Instant.now();

        store.upsertSpan(new SpanNode("s1", "t1", "", "svc", "op", 10, "OK", false, old));
        store.upsertSpan(new SpanNode("s2", "t1", "", "svc", "op", 20, "OK", false, recent));

        int pruned = store.prune();
        assertTrue(pruned >= 1);
    }

    @Test
    void testAnomalyStore() {
        var store = new AnomalyStore();
        Instant now = Instant.now();

        store.addAnomaly(new AnomalyNode("a1", AnomalyType.error_spike, AnomalySeverity.critical, "svc-a", "high errors", now));
        store.addAnomaly(new AnomalyNode("a2", AnomalyType.latency_spike, AnomalySeverity.warning, "svc-b", "slow", now));

        var all = store.anomaliesSince(now.minus(Duration.ofMinutes(1)));
        assertEquals(2, all.size());

        var forSvc = store.anomaliesForService("svc-a", now.minus(Duration.ofMinutes(1)));
        assertEquals(1, forSvc.size());
    }

    @Test
    void testHealthScoreComputation() {
        assertEquals(1.0, Schema.computeHealth(0.0, 50.0));
        assertTrue(Schema.computeHealth(0.5, 1000.0) < 0.5);
        assertEquals(0.0, Schema.computeHealth(1.0, 2000.0));
    }

    @Test
    void testSignalStore() {
        var store = new SignalStore();
        Instant now = Instant.now();

        store.upsertLogCluster("lc1", "error template", "ERROR", "svc-a", now);
        store.upsertLogCluster("lc1", "error template", "ERROR", "svc-a", now);
        store.upsertMetric("cpu_usage", "svc-a", 75.0, now);

        var logs = store.logClustersForService("svc-a");
        assertEquals(1, logs.size());
        assertEquals(2, logs.getFirst().count());

        var metrics = store.metricsForService("svc-a");
        assertEquals(1, metrics.size());
    }
}
