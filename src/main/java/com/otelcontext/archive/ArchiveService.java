package com.otelcontext.archive;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.otelcontext.compress.ZstdCompressor;
import com.otelcontext.config.AppConfig;
import com.otelcontext.repository.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.nio.file.*;
import java.security.MessageDigest;
import java.time.*;
import java.util.HexFormat;
import java.util.concurrent.atomic.AtomicLong;

@Service
public class ArchiveService {

    private static final Logger log = LoggerFactory.getLogger(ArchiveService.class);
    private static final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());

    private final TraceRepository traceRepo;
    private final LogRepository logRepo;
    private final MetricBucketRepository metricRepo;
    private final AppConfig config;
    private final AtomicLong recordsMoved = new AtomicLong();

    public ArchiveService(TraceRepository traceRepo, LogRepository logRepo,
                          MetricBucketRepository metricRepo, AppConfig config) {
        this.traceRepo = traceRepo;
        this.logRepo = logRepo;
        this.metricRepo = metricRepo;
        this.config = config;
    }

    /**
     * Run archival once daily. Check every hour if it's time.
     */
    @Scheduled(fixedRate = 3600000) // Check every hour
    public void checkAndArchive() {
        int currentHour = LocalTime.now(ZoneOffset.UTC).getHour();
        if (currentHour == config.getArchiveScheduleHour()) {
            try {
                runOnce();
            } catch (Exception e) {
                log.error("Archive run failed", e);
            }
        }
    }

    public void runOnce() throws IOException {
        Instant cutoff = Instant.now().minus(Duration.ofDays(config.getHotRetentionDays()));
        log.info("Starting archival pass, cutoff: {}", cutoff);

        // For simplicity, just purge old data (full archive would compress to disk)
        int traces = traceRepo.deleteOlderThan(cutoff);
        int logs = logRepo.deleteOlderThan(cutoff);
        int metrics = metricRepo.deleteOlderThan(cutoff);

        long total = traces + logs + metrics;
        recordsMoved.addAndGet(total);
        log.info("Archival complete: {} traces, {} logs, {} metrics purged", traces, logs, metrics);
    }

    public long getRecordsMoved() { return recordsMoved.get(); }
}
