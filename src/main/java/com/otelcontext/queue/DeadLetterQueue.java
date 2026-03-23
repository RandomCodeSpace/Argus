package com.otelcontext.queue;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.nio.file.*;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.locks.ReentrantLock;
import java.util.function.Function;
import java.util.stream.Stream;

public class DeadLetterQueue {

    private static final Logger log = LoggerFactory.getLogger(DeadLetterQueue.class);
    private static final ObjectMapper mapper = new ObjectMapper();

    private final Path dir;
    private final Function<byte[], Boolean> replayFn;
    private final int maxFiles;
    private final long maxDiskBytes;
    private final int maxRetries;
    private final Map<String, Integer> retries = new ConcurrentHashMap<>();
    private final ReentrantLock lock = new ReentrantLock();
    private final ScheduledExecutorService scheduler;

    public DeadLetterQueue(String dirPath, java.time.Duration replayInterval, Function<byte[], Boolean> replayFn,
                           int maxFiles, int maxDiskMb, int maxRetries) throws IOException {
        this.dir = Path.of(dirPath);
        this.replayFn = replayFn;
        this.maxFiles = maxFiles;
        this.maxDiskBytes = (long) maxDiskMb * 1024 * 1024;
        this.maxRetries = maxRetries;
        Files.createDirectories(dir);

        this.scheduler = Executors.newSingleThreadScheduledExecutor(Thread.ofVirtual().factory());
        scheduler.scheduleAtFixedRate(this::processFiles, replayInterval.toMillis(), replayInterval.toMillis(), TimeUnit.MILLISECONDS);
    }

    public void enqueue(Object batch) {
        lock.lock();
        try {
            byte[] data = mapper.writeValueAsBytes(batch);
            enforceLimits(data.length);
            String filename = "batch_" + System.nanoTime() + ".json";
            Files.write(dir.resolve(filename), data);
            log.warn("DLQ: batch written: {} ({} bytes)", filename, data.length);
        } catch (Exception e) {
            log.error("DLQ enqueue failed", e);
        } finally {
            lock.unlock();
        }
    }

    public int size() {
        try (Stream<Path> stream = Files.list(dir)) {
            return (int) stream.filter(p -> p.toString().endsWith(".json")).count();
        } catch (IOException e) { return 0; }
    }

    public long diskBytes() {
        try (Stream<Path> stream = Files.list(dir)) {
            return stream.filter(p -> p.toString().endsWith(".json"))
                .mapToLong(p -> { try { return Files.size(p); } catch (IOException e) { return 0; }})
                .sum();
        } catch (IOException e) { return 0; }
    }

    private void processFiles() {
        try (Stream<Path> stream = Files.list(dir)) {
            var files = stream.filter(p -> p.toString().endsWith(".json")).sorted().toList();
            for (var file : files) {
                String name = file.getFileName().toString();
                int tries = retries.getOrDefault(name, 0);
                if (maxRetries > 0 && tries >= maxRetries) {
                    Files.deleteIfExists(file);
                    retries.remove(name);
                    log.error("DLQ: max retries for {}, dropping", name);
                    continue;
                }
                byte[] data = Files.readAllBytes(file);
                try {
                    if (replayFn.apply(data)) {
                        Files.deleteIfExists(file);
                        retries.remove(name);
                        log.info("DLQ: replayed and removed {}", name);
                    } else {
                        retries.merge(name, 1, Integer::sum);
                    }
                } catch (Exception e) {
                    retries.merge(name, 1, Integer::sum);
                    log.warn("DLQ replay failed for {}: {}", name, e.getMessage());
                }
            }
        } catch (IOException e) { log.error("DLQ processFiles error", e); }
    }

    private void enforceLimits(long incomingBytes) {
        try (Stream<Path> stream = Files.list(dir)) {
            var files = stream.filter(p -> p.toString().endsWith(".json")).sorted().toList();
            long totalBytes = files.stream().mapToLong(p -> { try { return Files.size(p); } catch (IOException e) { return 0; }}).sum();
            int i = 0;
            while (i < files.size()) {
                boolean overFiles = maxFiles > 0 && (files.size() - i) >= maxFiles;
                boolean overDisk = maxDiskBytes > 0 && (totalBytes + incomingBytes) > maxDiskBytes;
                if (!overFiles && !overDisk) break;
                try {
                    totalBytes -= Files.size(files.get(i));
                    Files.deleteIfExists(files.get(i));
                } catch (IOException ignored) {}
                i++;
            }
        } catch (IOException ignored) {}
    }

    public void stop() {
        scheduler.shutdownNow();
    }
}
