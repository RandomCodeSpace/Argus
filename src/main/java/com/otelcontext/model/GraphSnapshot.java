package com.otelcontext.model;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "graph_snapshots")
public class GraphSnapshot {

    @Id
    @Column(length = 64)
    private String id;

    private Instant createdAt;

    @Column(columnDefinition = "CLOB")
    private String nodes; // JSON

    @Column(columnDefinition = "CLOB")
    private String edges; // JSON

    private int serviceCount;
    private long totalCalls;
    private double avgHealthScore;

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public String getNodes() { return nodes; }
    public void setNodes(String nodes) { this.nodes = nodes; }
    public String getEdges() { return edges; }
    public void setEdges(String edges) { this.edges = edges; }
    public int getServiceCount() { return serviceCount; }
    public void setServiceCount(int serviceCount) { this.serviceCount = serviceCount; }
    public long getTotalCalls() { return totalCalls; }
    public void setTotalCalls(long totalCalls) { this.totalCalls = totalCalls; }
    public double getAvgHealthScore() { return avgHealthScore; }
    public void setAvgHealthScore(double avgHealthScore) { this.avgHealthScore = avgHealthScore; }
}
