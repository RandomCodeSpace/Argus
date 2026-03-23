package com.otelcontext.queue;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Path;
import java.time.Duration;

import static org.junit.jupiter.api.Assertions.*;

class DeadLetterQueueTest {

    @TempDir
    Path tempDir;

    @Test
    void testEnqueueAndSize() throws Exception {
        var dlq = new DeadLetterQueue(tempDir.toString(), Duration.ofHours(1),
            data -> true, 100, 100, 10);
        try {
            dlq.enqueue(java.util.Map.of("type", "logs", "data", java.util.List.of()));
            assertEquals(1, dlq.size());
            assertTrue(dlq.diskBytes() > 0);
        } finally {
            dlq.stop();
        }
    }

    @Test
    void testMaxFilesEnforcement() throws Exception {
        var dlq = new DeadLetterQueue(tempDir.toString(), Duration.ofHours(1),
            data -> true, 3, 100, 10);
        try {
            for (int i = 0; i < 5; i++) {
                dlq.enqueue(java.util.Map.of("batch", i));
            }
            assertTrue(dlq.size() <= 3);
        } finally {
            dlq.stop();
        }
    }
}
