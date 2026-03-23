package com.otelcontext.model;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;
import java.time.Instant;
import java.util.List;

@Entity
@Table(name = "traces", indexes = {
    @Index(name = "idx_trace_trace_id", columnList = "traceId", unique = true),
    @Index(name = "idx_trace_service", columnList = "serviceName"),
    @Index(name = "idx_trace_timestamp", columnList = "timestamp"),
    @Index(name = "idx_trace_duration", columnList = "duration")
})
public class Trace {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, length = 32)
    private String traceId;

    @Column(length = 255)
    private String serviceName;

    private long duration; // microseconds

    @Column(length = 50)
    private String status;

    private Instant timestamp;

    @OneToMany(mappedBy = "traceId", fetch = FetchType.LAZY)
    @JsonIgnore
    private List<Span> spans;

    @Transient
    @JsonProperty("duration_ms")
    public double getDurationMs() { return duration / 1000.0; }

    @Transient
    @JsonProperty("span_count")
    public int getSpanCount() { return spans != null ? spans.size() : 0; }

    @Transient
    private String operation;

    // Getters and setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getTraceId() { return traceId; }
    public void setTraceId(String traceId) { this.traceId = traceId; }
    public String getServiceName() { return serviceName; }
    public void setServiceName(String serviceName) { this.serviceName = serviceName; }
    public long getDuration() { return duration; }
    public void setDuration(long duration) { this.duration = duration; }
    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }
    public Instant getTimestamp() { return timestamp; }
    public void setTimestamp(Instant timestamp) { this.timestamp = timestamp; }
    public List<Span> getSpans() { return spans; }
    public void setSpans(List<Span> spans) { this.spans = spans; }
    public String getOperation() { return operation; }
    public void setOperation(String operation) { this.operation = operation; }
}
