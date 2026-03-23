package com.otelcontext.realtime;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.*;
import org.springframework.web.socket.handler.TextWebSocketHandler;

import java.io.IOException;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

@Component
public class WebSocketHub extends TextWebSocketHandler {

    private static final Logger log = LoggerFactory.getLogger(WebSocketHub.class);
    private static final ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());
    private static final int MAX_BUFFER = 100;
    private static final long FLUSH_INTERVAL_MS = 500;

    private final Set<WebSocketSession> sessions = ConcurrentHashMap.newKeySet();
    private final List<Object> logBuffer = new ArrayList<>();
    private final List<Object> metricBuffer = new ArrayList<>();
    private final AtomicInteger connectionCount = new AtomicInteger();

    public WebSocketHub() {
        // Start flush timer
        var scheduler = Executors.newSingleThreadScheduledExecutor(Thread.ofVirtual().factory());
        scheduler.scheduleAtFixedRate(this::flush, FLUSH_INTERVAL_MS, FLUSH_INTERVAL_MS, TimeUnit.MILLISECONDS);
    }

    @Override
    public void afterConnectionEstablished(WebSocketSession session) {
        sessions.add(session);
        connectionCount.incrementAndGet();
        log.info("WebSocket client connected, total: {}", sessions.size());
    }

    @Override
    public void afterConnectionClosed(WebSocketSession session, CloseStatus status) {
        sessions.remove(session);
        connectionCount.decrementAndGet();
        log.info("WebSocket client disconnected, total: {}", sessions.size());
    }

    public void broadcastLog(Object logEntry) {
        synchronized (logBuffer) {
            logBuffer.add(logEntry);
            if (logBuffer.size() >= MAX_BUFFER) flush();
        }
    }

    public void broadcastMetric(Object metricEntry) {
        synchronized (metricBuffer) {
            metricBuffer.add(metricEntry);
            if (metricBuffer.size() >= MAX_BUFFER) flush();
        }
    }

    private void flush() {
        List<Object> logs, metrics;
        synchronized (logBuffer) {
            if (logBuffer.isEmpty() && metricBuffer.isEmpty()) return;
            logs = new ArrayList<>(logBuffer);
            logBuffer.clear();
            metrics = new ArrayList<>(metricBuffer);
            metricBuffer.clear();
        }

        if (!logs.isEmpty()) sendBatch("logs", logs);
        if (!metrics.isEmpty()) sendBatch("metrics", metrics);
    }

    private void sendBatch(String type, List<Object> data) {
        try {
            String json = mapper.writeValueAsString(Map.of("type", type, "data", data));
            TextMessage msg = new TextMessage(json);
            List<WebSocketSession> slow = new ArrayList<>();
            for (var session : sessions) {
                try {
                    if (session.isOpen()) session.sendMessage(msg);
                    else slow.add(session);
                } catch (IOException e) { slow.add(session); }
            }
            sessions.removeAll(slow);
        } catch (Exception e) { log.error("WebSocket broadcast error", e); }
    }

    public int getConnectionCount() { return connectionCount.get(); }
}
