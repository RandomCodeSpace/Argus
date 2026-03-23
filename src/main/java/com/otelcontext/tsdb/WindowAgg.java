package com.otelcontext.tsdb;

import java.time.Instant;

public class WindowAgg {
    private String metricName;
    private String serviceName;
    private Instant windowStart;
    private long count;
    private double sum;
    private double min = Double.MAX_VALUE;
    private double max = -Double.MAX_VALUE;
    private double p50, p95, p99;

    public WindowAgg(String metricName, String serviceName) {
        this.metricName = metricName;
        this.serviceName = serviceName;
    }

    // Getters and setters
    public String getMetricName() { return metricName; }
    public String getServiceName() { return serviceName; }
    public Instant getWindowStart() { return windowStart; }
    public void setWindowStart(Instant windowStart) { this.windowStart = windowStart; }
    public long getCount() { return count; }
    public void setCount(long count) { this.count = count; }
    public double getSum() { return sum; }
    public void setSum(double sum) { this.sum = sum; }
    public double getMin() { return min == Double.MAX_VALUE ? 0 : min; }
    public void setMin(double min) { this.min = min; }
    public double getMax() { return max == -Double.MAX_VALUE ? 0 : max; }
    public void setMax(double max) { this.max = max; }
    public double getP50() { return p50; }
    public void setP50(double p50) { this.p50 = p50; }
    public double getP95() { return p95; }
    public void setP95(double p95) { this.p95 = p95; }
    public double getP99() { return p99; }
    public void setP99(double p99) { this.p99 = p99; }
}
