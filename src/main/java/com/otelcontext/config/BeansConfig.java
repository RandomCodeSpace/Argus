package com.otelcontext.config;

import com.otelcontext.queue.DeadLetterQueue;
import com.otelcontext.repository.MetricBucketRepository;
import com.otelcontext.tsdb.RingBuffer;
import com.otelcontext.tsdb.TsdbAggregator;
import com.otelcontext.vectordb.TfIdfIndex;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.scheduling.annotation.Scheduled;

import java.io.IOException;
import java.time.Duration;

@Configuration
public class BeansConfig {

    private static final Logger log = LoggerFactory.getLogger(BeansConfig.class);

    @Bean
    public TfIdfIndex tfIdfIndex(AppConfig config) {
        return new TfIdfIndex(config.getVectorIndexMaxEntries());
    }

    @Bean
    public RingBuffer ringBuffer(AppConfig config) {
        return new RingBuffer(config.getTsdbRingSlots(), Duration.ofSeconds(config.getTsdbWindowSeconds()));
    }

    @Bean
    public TsdbAggregator tsdbAggregator(MetricBucketRepository repo, AppConfig config, RingBuffer ringBuffer) {
        var agg = new TsdbAggregator(repo, Duration.ofSeconds(config.getTsdbWindowSeconds()),
            ringBuffer, config.getMetricMaxCardinality());
        log.info("TSDB Aggregator initialized ({}s window, {} max cardinality)",
            config.getTsdbWindowSeconds(), config.getMetricMaxCardinality());
        return agg;
    }

    @Bean
    public DeadLetterQueue deadLetterQueue(AppConfig config) throws IOException {
        Duration interval = parseDuration(config.getDlqReplayInterval());
        return new DeadLetterQueue(config.getDlqPath(), interval, data -> {
            // Simple replay - log for now
            log.info("DLQ replay: {} bytes", data.length);
            return true;
        }, config.getDlqMaxFiles(), config.getDlqMaxDiskMb(), config.getDlqMaxRetries());
    }

    @Bean
    public TsdbFlusher tsdbFlusher(TsdbAggregator tsdb) {
        return new TsdbFlusher(tsdb);
    }

    private Duration parseDuration(String s) {
        if (s == null || s.isEmpty()) return Duration.ofMinutes(5);
        try {
            if (s.endsWith("m")) return Duration.ofMinutes(Long.parseLong(s.replace("m", "")));
            if (s.endsWith("s")) return Duration.ofSeconds(Long.parseLong(s.replace("s", "")));
            if (s.endsWith("h")) return Duration.ofHours(Long.parseLong(s.replace("h", "")));
            return Duration.parse("PT" + s);
        } catch (Exception e) { return Duration.ofMinutes(5); }
    }

    /** Scheduled TSDB flusher */
    public static class TsdbFlusher {
        private final TsdbAggregator tsdb;
        TsdbFlusher(TsdbAggregator tsdb) { this.tsdb = tsdb; }

        @Scheduled(fixedRateString = "${otelcontext.tsdb-window-seconds:30}000")
        public void flush() { tsdb.flush(); }
    }
}
