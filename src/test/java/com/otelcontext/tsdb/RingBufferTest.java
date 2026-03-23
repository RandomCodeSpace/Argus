package com.otelcontext.tsdb;

import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.time.Instant;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class RingBufferTest {

    @Test
    void testRecordAndQuery() {
        var rb = new RingBuffer(120, Duration.ofSeconds(30));
        Instant now = Instant.now();

        for (int i = 0; i < 100; i++) {
            rb.record("latency", "svc-a", 10.0 + i, now);
        }

        var windows = rb.queryRecent("latency", "svc-a", 5);
        assertFalse(windows.isEmpty());
        assertEquals(100, windows.getFirst().getCount());
    }

    @Test
    void testPercentiles() {
        List<Double> data = List.of(1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0);
        assertEquals(5.0, RingBuffer.percentile(data, 50));
        assertEquals(10.0, RingBuffer.percentile(data, 95));
        assertEquals(10.0, RingBuffer.percentile(data, 99));
    }

    @Test
    void testEmptyQuery() {
        var rb = new RingBuffer(10, Duration.ofSeconds(30));
        var windows = rb.queryRecent("nonexistent", "svc", 5);
        assertTrue(windows.isEmpty());
    }

    @Test
    void testMultipleMetrics() {
        var rb = new RingBuffer(10, Duration.ofSeconds(30));
        Instant now = Instant.now();

        rb.record("latency", "svc-a", 100, now);
        rb.record("errors", "svc-a", 5, now);
        rb.record("latency", "svc-b", 200, now);

        assertEquals(3, rb.metricCount());
        assertEquals(3, rb.allKeys().size());
    }
}
