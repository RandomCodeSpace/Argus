package com.otelcontext.model;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "investigations", indexes = {
    @Index(name = "idx_inv_trigger_svc", columnList = "triggerService")
})
public class Investigation {

    @Id
    @Column(length = 64)
    private String id;

    private Instant createdAt;

    @Column(length = 20)
    private String status; // detected, triaged, resolved

    @Column(length = 20)
    private String severity; // critical, warning, info

    @Column(length = 255)
    private String triggerService;

    @Column(length = 255)
    private String triggerOperation;

    @Column(columnDefinition = "CLOB")
    private String errorMessage;

    @Column(length = 255)
    private String rootService;

    @Column(length = 255)
    private String rootOperation;

    @Column(columnDefinition = "CLOB")
    private String causalChain; // JSON

    @Column(columnDefinition = "CLOB")
    private String traceIds; // JSON

    @Column(columnDefinition = "CLOB")
    private String errorLogs; // JSON

    @Column(columnDefinition = "CLOB")
    private String anomalousMetrics; // JSON

    @Column(columnDefinition = "CLOB")
    private String affectedServices; // JSON

    @Column(columnDefinition = "CLOB")
    private String spanChain; // JSON

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }
    public String getSeverity() { return severity; }
    public void setSeverity(String severity) { this.severity = severity; }
    public String getTriggerService() { return triggerService; }
    public void setTriggerService(String triggerService) { this.triggerService = triggerService; }
    public String getTriggerOperation() { return triggerOperation; }
    public void setTriggerOperation(String triggerOperation) { this.triggerOperation = triggerOperation; }
    public String getErrorMessage() { return errorMessage; }
    public void setErrorMessage(String errorMessage) { this.errorMessage = errorMessage; }
    public String getRootService() { return rootService; }
    public void setRootService(String rootService) { this.rootService = rootService; }
    public String getRootOperation() { return rootOperation; }
    public void setRootOperation(String rootOperation) { this.rootOperation = rootOperation; }
    public String getCausalChain() { return causalChain; }
    public void setCausalChain(String causalChain) { this.causalChain = causalChain; }
    public String getTraceIds() { return traceIds; }
    public void setTraceIds(String traceIds) { this.traceIds = traceIds; }
    public String getErrorLogs() { return errorLogs; }
    public void setErrorLogs(String errorLogs) { this.errorLogs = errorLogs; }
    public String getAnomalousMetrics() { return anomalousMetrics; }
    public void setAnomalousMetrics(String anomalousMetrics) { this.anomalousMetrics = anomalousMetrics; }
    public String getAffectedServices() { return affectedServices; }
    public void setAffectedServices(String affectedServices) { this.affectedServices = affectedServices; }
    public String getSpanChain() { return spanChain; }
    public void setSpanChain(String spanChain) { this.spanChain = spanChain; }
}
