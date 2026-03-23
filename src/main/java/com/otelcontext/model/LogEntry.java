package com.otelcontext.model;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "logs", indexes = {
    @Index(name = "idx_log_trace_id", columnList = "traceId"),
    @Index(name = "idx_log_severity", columnList = "severity"),
    @Index(name = "idx_log_service", columnList = "serviceName"),
    @Index(name = "idx_log_timestamp", columnList = "timestamp")
})
public class LogEntry {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(length = 32)
    private String traceId;

    @Column(length = 16)
    private String spanId;

    @Column(length = 50)
    private String severity;

    @Column(columnDefinition = "CLOB")
    private String body;

    @Column(length = 255)
    private String serviceName;

    @Column(columnDefinition = "CLOB")
    private String attributesJson;

    @Column(columnDefinition = "CLOB")
    private String aiInsight;

    private Instant timestamp;

    // Getters and setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getTraceId() { return traceId; }
    public void setTraceId(String traceId) { this.traceId = traceId; }
    public String getSpanId() { return spanId; }
    public void setSpanId(String spanId) { this.spanId = spanId; }
    public String getSeverity() { return severity; }
    public void setSeverity(String severity) { this.severity = severity; }
    public String getBody() { return body; }
    public void setBody(String body) { this.body = body; }
    public String getServiceName() { return serviceName; }
    public void setServiceName(String serviceName) { this.serviceName = serviceName; }
    public String getAttributesJson() { return attributesJson; }
    public void setAttributesJson(String attributesJson) { this.attributesJson = attributesJson; }
    public String getAiInsight() { return aiInsight; }
    public void setAiInsight(String aiInsight) { this.aiInsight = aiInsight; }
    public Instant getTimestamp() { return timestamp; }
    public void setTimestamp(Instant timestamp) { this.timestamp = timestamp; }
}
