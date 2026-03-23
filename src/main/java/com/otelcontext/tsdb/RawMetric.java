package com.otelcontext.tsdb;

import java.time.Instant;
import java.util.Map;

public record RawMetric(String name, String serviceName, double value, Instant timestamp, Map<String, Object> attributes) {}
