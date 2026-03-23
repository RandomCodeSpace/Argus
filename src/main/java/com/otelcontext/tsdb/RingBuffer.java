package com.otelcontext.tsdb;

import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Per-metric ring buffer with pre-computed percentiles.
 */
public class RingBuffer {

    private static final int MAX_SAMPLES = 256;

    private final ConcurrentHashMap<String, MetricRing> rings = new ConcurrentHashMap<>();
    private final int slots;
    private final Duration windowDur;

    public RingBuffer(int slots, Duration windowDur) {
        this.slots = slots;
        this.windowDur = windowDur;
    }

    public void record(String metricName, String serviceName, double value, Instant at) {
        String key = serviceName + "|" + metricName;
        rings.computeIfAbsent(key, k -> new MetricRing(metricName, serviceName, slots, windowDur))
             .record(value, at);
    }

    public List<WindowAgg> queryRecent(String metricName, String serviceName, int windowCount) {
        String key = serviceName + "|" + metricName;
        MetricRing ring = rings.get(key);
        return ring != null ? ring.windows(windowCount) : List.of();
    }

    public Set<String> allKeys() { return rings.keySet(); }
    public int metricCount() { return rings.size(); }

    private static class MetricRing {
        private final ReentrantLock lock = new ReentrantLock();
        private final RingSlot[] ringSlots;
        private final int size;
        private final Duration windowDur;
        private final String metricName;
        private final String serviceName;
        private int currentIdx;
        private Instant currentStart;

        MetricRing(String metricName, String serviceName, int slots, Duration windowDur) {
            this.size = slots;
            this.windowDur = windowDur;
            this.metricName = metricName;
            this.serviceName = serviceName;
            this.ringSlots = new RingSlot[slots];
            for (int i = 0; i < slots; i++) ringSlots[i] = new RingSlot();
            this.currentStart = Instant.now().truncatedTo(java.time.temporal.ChronoUnit.SECONDS);
            long secs = windowDur.toSeconds();
            if (secs > 0) {
                long epoch = currentStart.getEpochSecond();
                currentStart = Instant.ofEpochSecond(epoch - (epoch % secs));
            }
            ringSlots[0].windowStart = currentStart;
        }

        void record(double value, Instant at) {
            lock.lock();
            try {
                long secs = windowDur.toSeconds();
                long epoch = at.getEpochSecond();
                Instant windowStart = Instant.ofEpochSecond(epoch - (epoch % secs));

                if (windowStart.isAfter(currentStart)) {
                    long steps = Duration.between(currentStart, windowStart).toSeconds() / secs;
                    for (long i = 0; i < steps && i < size; i++) {
                        currentIdx = (currentIdx + 1) % size;
                        currentStart = currentStart.plus(windowDur);
                        ringSlots[currentIdx] = new RingSlot();
                        ringSlots[currentIdx].windowStart = currentStart;
                    }
                }

                RingSlot slot = ringSlots[currentIdx];
                slot.count++;
                slot.sum += value;
                if (value < slot.min) slot.min = value;
                if (value > slot.max) slot.max = value;
                if (slot.samples.size() < MAX_SAMPLES) slot.samples.add(value);
            } finally {
                lock.unlock();
            }
        }

        List<WindowAgg> windows(int n) {
            lock.lock();
            List<RingSlot> snaps = new ArrayList<>();
            try {
                if (n > size) n = size;
                for (int i = 0; i < n; i++) {
                    int idx = (currentIdx - i + size) % size;
                    RingSlot s = ringSlots[idx];
                    if (s.count > 0) {
                        snaps.add(s.copy());
                    }
                }
            } finally {
                lock.unlock();
            }

            List<WindowAgg> out = new ArrayList<>();
            for (RingSlot snap : snaps) {
                WindowAgg agg = new WindowAgg(metricName, serviceName);
                agg.setWindowStart(snap.windowStart);
                agg.setCount(snap.count);
                agg.setSum(snap.sum);
                agg.setMin(snap.min);
                agg.setMax(snap.max);
                agg.setP50(percentile(snap.samples, 50));
                agg.setP95(percentile(snap.samples, 95));
                agg.setP99(percentile(snap.samples, 99));
                out.add(agg);
            }
            return out;
        }
    }

    private static class RingSlot {
        Instant windowStart = Instant.EPOCH;
        long count;
        double sum;
        double min = Double.MAX_VALUE;
        double max = -Double.MAX_VALUE;
        List<Double> samples = new ArrayList<>();

        RingSlot copy() {
            RingSlot c = new RingSlot();
            c.windowStart = windowStart;
            c.count = count;
            c.sum = sum;
            c.min = min;
            c.max = max;
            c.samples = new ArrayList<>(samples);
            return c;
        }
    }

    static double percentile(List<Double> data, double p) {
        if (data.isEmpty()) return 0;
        double[] sorted = data.stream().mapToDouble(Double::doubleValue).sorted().toArray();
        int idx = (int) Math.ceil(p / 100.0 * sorted.length) - 1;
        idx = Math.max(0, Math.min(idx, sorted.length - 1));
        return sorted[idx];
    }
}
