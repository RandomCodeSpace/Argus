package com.otelcontext.tsdb;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.otelcontext.model.MetricBucket;
import com.otelcontext.repository.MetricBucketRepository;
import jakarta.annotation.PreDestroy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;

import java.time.Duration;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Tumbling-window metric aggregator with cardinality limits.
 */
public class TsdbAggregator {

    private static final Logger log = LoggerFactory.getLogger(TsdbAggregator.class);
    private static final ObjectMapper mapper = new ObjectMapper();

    private final MetricBucketRepository repo;
    private final Duration windowSize;
    private final ConcurrentHashMap<String, MetricBucket> buckets = new ConcurrentHashMap<>();
    private final RingBuffer ringBuffer;
    private final int maxCardinality;
    private final AtomicLong droppedBatches = new AtomicLong();
    private final BlockingQueue<List<MetricBucket>> flushQueue = new LinkedBlockingQueue<>(500);
    private final ExecutorService workers;

    public TsdbAggregator(MetricBucketRepository repo, Duration windowSize, RingBuffer ringBuffer, int maxCardinality) {
        this.repo = repo;
        this.windowSize = windowSize;
        this.ringBuffer = ringBuffer;
        this.maxCardinality = maxCardinality;

        // Start 3 persistence workers using virtual threads
        this.workers = Executors.newVirtualThreadPerTaskExecutor();
        for (int i = 0; i < 3; i++) {
            workers.submit(this::persistenceWorker);
        }
    }

    public void ingest(RawMetric m) {
        final String attrJson;
        String tmp = "{}";
        try { tmp = mapper.writeValueAsString(m.attributes()); } catch (Exception ignored) {}
        attrJson = tmp;
        String key = m.serviceName() + "|" + m.name() + "|" + attrJson;

        if (ringBuffer != null) {
            ringBuffer.record(m.name(), m.serviceName(), m.value(), m.timestamp());
        }

        buckets.compute(key, (k, existing) -> {
            if (existing == null) {
                if (maxCardinality > 0 && buckets.size() >= maxCardinality) {
                    return existing; // drop
                }
                MetricBucket b = new MetricBucket();
                b.setName(m.name());
                b.setServiceName(m.serviceName());
                long secs = windowSize.toSeconds();
                long epoch = m.timestamp().getEpochSecond();
                b.setTimeBucket(Instant.ofEpochSecond(epoch - (epoch % secs)));
                b.setMin(m.value());
                b.setMax(m.value());
                b.setSum(m.value());
                b.setCount(1);
                b.setAttributesJson(attrJson);
                return b;
            }
            if (m.value() < existing.getMin()) existing.setMin(m.value());
            if (m.value() > existing.getMax()) existing.setMax(m.value());
            existing.setSum(existing.getSum() + m.value());
            existing.setCount(existing.getCount() + 1);
            return existing;
        });
    }

    public void flush() {
        if (buckets.isEmpty()) return;
        List<MetricBucket> batch = new ArrayList<>();
        var iter = buckets.entrySet().iterator();
        while (iter.hasNext()) {
            batch.add(iter.next().getValue());
            iter.remove();
        }
        if (!flushQueue.offer(batch)) {
            droppedBatches.incrementAndGet();
            log.warn("TSDB flush queue full, dropped batch of {}", batch.size());
        }
    }

    private void persistenceWorker() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                List<MetricBucket> batch = flushQueue.poll(5, TimeUnit.SECONDS);
                if (batch != null && !batch.isEmpty()) {
                    repo.saveAll(batch);
                    log.debug("TSDB persisted {} metric buckets", batch.size());
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return;
            } catch (Exception e) {
                log.error("TSDB persistence error", e);
            }
        }
    }

    public int bucketCount() { return buckets.size(); }
    public long droppedBatches() { return droppedBatches.get(); }
    public RingBuffer getRingBuffer() { return ringBuffer; }

    @PreDestroy
    public void stop() {
        flush();
        workers.shutdownNow();
    }
}
