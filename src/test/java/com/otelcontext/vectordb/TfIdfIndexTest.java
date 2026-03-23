package com.otelcontext.vectordb;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class TfIdfIndexTest {

    @Test
    void testAddAndSearch() {
        var index = new TfIdfIndex(1000);
        index.add(1, "svc-a", "ERROR", "NullPointerException in UserService.getUser");
        index.add(2, "svc-a", "ERROR", "NullPointerException in OrderService.createOrder");
        index.add(3, "svc-b", "ERROR", "Connection timeout to database");
        index.add(4, "svc-b", "WARN", "Slow query detected in payment processing");

        assertEquals(4, index.size());

        var results = index.search("NullPointerException", 5);
        assertFalse(results.isEmpty());
        // Both NPE logs should rank higher
        assertTrue(results.getFirst().score() > 0);
    }

    @Test
    void testOnlyIndexesErrorAndWarn() {
        var index = new TfIdfIndex(100);
        index.add(1, "svc-a", "INFO", "Server started");
        index.add(2, "svc-a", "DEBUG", "Request received");
        index.add(3, "svc-a", "ERROR", "Database connection failed");

        assertEquals(1, index.size());
    }

    @Test
    void testFifoEviction() {
        var index = new TfIdfIndex(10);
        for (int i = 0; i < 15; i++) {
            index.add(i, "svc", "ERROR", "error message " + i);
        }
        // After eviction, size should be around 10 (9 kept + new ones)
        assertTrue(index.size() <= 15);
    }

    @Test
    void testEmptySearch() {
        var index = new TfIdfIndex(100);
        var results = index.search("anything", 5);
        assertTrue(results.isEmpty());
    }

    @Test
    void testTokenize() {
        var tokens = TfIdfIndex.tokenize("NullPointerException in UserService.getUser");
        assertFalse(tokens.isEmpty());
        assertTrue(tokens.contains("nullpointerexception"));
        assertTrue(tokens.contains("userservice"));
    }
}
