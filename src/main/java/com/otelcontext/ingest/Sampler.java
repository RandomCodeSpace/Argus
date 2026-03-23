package com.otelcontext.ingest;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Adaptive trace sampler with per-service token buckets.
 */
public class Sampler {

    private final double rate;
    private final boolean alwaysOnErrors;
    private final double latencyThresholdMs;
    private final Map<String, TokenBucket> buckets = new ConcurrentHashMap<>();
    private final AtomicLong totalSeen = new AtomicLong();
    private final AtomicLong totalDropped = new AtomicLong();

    public Sampler(double rate, boolean alwaysOnErrors, double latencyThresholdMs) {
        this.rate = Math.max(0, Math.min(1, rate));
        this.alwaysOnErrors = alwaysOnErrors;
        this.latencyThresholdMs = latencyThresholdMs;
    }

    public boolean shouldSample(String serviceName, boolean isError, double durationMs) {
        totalSeen.incrementAndGet();
        if (alwaysOnErrors && isError) return true;
        if (durationMs >= latencyThresholdMs) return true;
        if (rate >= 1.0) return true;
        if (rate <= 0) { totalDropped.incrementAndGet(); return false; }

        var bucket = buckets.computeIfAbsent(serviceName, k -> {
            // Always let first trace through
            return new TokenBucket(rate);
        });

        boolean allow = bucket.allow();
        if (!allow) totalDropped.incrementAndGet();
        return allow;
    }

    private static class TokenBucket {
        private final double rate;
        private double tokens;
        private long lastTickNanos;

        TokenBucket(double rate) {
            this.rate = rate;
            this.tokens = rate;
            this.lastTickNanos = System.nanoTime();
        }

        synchronized boolean allow() {
            long now = System.nanoTime();
            double elapsed = (now - lastTickNanos) / 1_000_000_000.0;
            lastTickNanos = now;
            tokens = Math.min(1.0, tokens + elapsed * rate);
            if (tokens >= 1.0 / rate) {
                tokens -= 1.0 / rate;
                return true;
            }
            return false;
        }
    }
}
