package com.otelcontext.model;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "spans", indexes = {
    @Index(name = "idx_span_trace_id", columnList = "traceId"),
    @Index(name = "idx_span_operation", columnList = "operationName"),
    @Index(name = "idx_span_service", columnList = "serviceName")
})
public class Span {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, length = 32)
    private String traceId;

    @Column(nullable = false, length = 16)
    private String spanId;

    @Column(length = 16)
    private String parentSpanId;

    @Column(length = 255)
    private String operationName;

    private Instant startTime;
    private Instant endTime;
    private long duration; // microseconds

    @Column(length = 255)
    private String serviceName;

    @Column(columnDefinition = "CLOB")
    private String attributesJson;

    @Transient
    public double getDurationMs() { return duration / 1000.0; }

    @Transient
    public boolean isError() {
        // Check if attributes contain error status
        return attributesJson != null && attributesJson.contains("STATUS_CODE_ERROR");
    }

    // Getters and setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getTraceId() { return traceId; }
    public void setTraceId(String traceId) { this.traceId = traceId; }
    public String getSpanId() { return spanId; }
    public void setSpanId(String spanId) { this.spanId = spanId; }
    public String getParentSpanId() { return parentSpanId; }
    public void setParentSpanId(String parentSpanId) { this.parentSpanId = parentSpanId; }
    public String getOperationName() { return operationName; }
    public void setOperationName(String operationName) { this.operationName = operationName; }
    public Instant getStartTime() { return startTime; }
    public void setStartTime(Instant startTime) { this.startTime = startTime; }
    public Instant getEndTime() { return endTime; }
    public void setEndTime(Instant endTime) { this.endTime = endTime; }
    public long getDuration() { return duration; }
    public void setDuration(long duration) { this.duration = duration; }
    public String getServiceName() { return serviceName; }
    public void setServiceName(String serviceName) { this.serviceName = serviceName; }
    public String getAttributesJson() { return attributesJson; }
    public void setAttributesJson(String attributesJson) { this.attributesJson = attributesJson; }
}
