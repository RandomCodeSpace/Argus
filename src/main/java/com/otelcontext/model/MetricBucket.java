package com.otelcontext.model;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "metric_buckets", indexes = {
    @Index(name = "idx_metric_name", columnList = "name"),
    @Index(name = "idx_metric_service", columnList = "serviceName"),
    @Index(name = "idx_metric_time", columnList = "timeBucket")
})
public class MetricBucket {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, length = 255)
    private String name;

    @Column(nullable = false, length = 255)
    private String serviceName;

    @Column(nullable = false)
    private Instant timeBucket;

    private double min;
    private double max;
    private double sum;
    private long count;

    @Column(columnDefinition = "CLOB")
    private String attributesJson;

    // Getters and setters
    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    public String getServiceName() { return serviceName; }
    public void setServiceName(String serviceName) { this.serviceName = serviceName; }
    public Instant getTimeBucket() { return timeBucket; }
    public void setTimeBucket(Instant timeBucket) { this.timeBucket = timeBucket; }
    public double getMin() { return min; }
    public void setMin(double min) { this.min = min; }
    public double getMax() { return max; }
    public void setMax(double max) { this.max = max; }
    public double getSum() { return sum; }
    public void setSum(double sum) { this.sum = sum; }
    public long getCount() { return count; }
    public void setCount(long count) { this.count = count; }
    public String getAttributesJson() { return attributesJson; }
    public void setAttributesJson(String attributesJson) { this.attributesJson = attributesJson; }
}
