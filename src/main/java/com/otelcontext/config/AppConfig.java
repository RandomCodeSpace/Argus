package com.otelcontext.config;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.context.annotation.Configuration;

@Configuration
@ConfigurationProperties(prefix = "otelcontext")
public class AppConfig {

    private int grpcPort = 4317;
    private String env = "development";
    private int hotRetentionDays = 7;
    private String coldStoragePath = "./data/cold";
    private int coldStorageMaxGb = 50;
    private int archiveScheduleHour = 2;
    private int archiveBatchSize = 10000;
    private double samplingRate = 1.0;
    private boolean samplingAlwaysOnErrors = true;
    private int samplingLatencyThresholdMs = 500;
    private int metricMaxCardinality = 10000;
    private int apiRateLimitRps = 100;
    private boolean mcpEnabled = true;
    private String mcpPath = "/mcp";
    private int vectorIndexMaxEntries = 100000;
    private String dlqPath = "./data/dlq";
    private int dlqMaxFiles = 1000;
    private int dlqMaxDiskMb = 500;
    private int dlqMaxRetries = 10;
    private String dlqReplayInterval = "5m";
    private int tsdbWindowSeconds = 30;
    private int tsdbRingSlots = 120;

    public boolean isDevMode() { return "development".equals(env); }

    // Getters and setters
    public int getGrpcPort() { return grpcPort; }
    public void setGrpcPort(int grpcPort) { this.grpcPort = grpcPort; }
    public String getEnv() { return env; }
    public void setEnv(String env) { this.env = env; }
    public int getHotRetentionDays() { return hotRetentionDays; }
    public void setHotRetentionDays(int hotRetentionDays) { this.hotRetentionDays = hotRetentionDays; }
    public String getColdStoragePath() { return coldStoragePath; }
    public void setColdStoragePath(String coldStoragePath) { this.coldStoragePath = coldStoragePath; }
    public int getColdStorageMaxGb() { return coldStorageMaxGb; }
    public void setColdStorageMaxGb(int coldStorageMaxGb) { this.coldStorageMaxGb = coldStorageMaxGb; }
    public int getArchiveScheduleHour() { return archiveScheduleHour; }
    public void setArchiveScheduleHour(int archiveScheduleHour) { this.archiveScheduleHour = archiveScheduleHour; }
    public int getArchiveBatchSize() { return archiveBatchSize; }
    public void setArchiveBatchSize(int archiveBatchSize) { this.archiveBatchSize = archiveBatchSize; }
    public double getSamplingRate() { return samplingRate; }
    public void setSamplingRate(double samplingRate) { this.samplingRate = samplingRate; }
    public boolean isSamplingAlwaysOnErrors() { return samplingAlwaysOnErrors; }
    public void setSamplingAlwaysOnErrors(boolean samplingAlwaysOnErrors) { this.samplingAlwaysOnErrors = samplingAlwaysOnErrors; }
    public int getSamplingLatencyThresholdMs() { return samplingLatencyThresholdMs; }
    public void setSamplingLatencyThresholdMs(int samplingLatencyThresholdMs) { this.samplingLatencyThresholdMs = samplingLatencyThresholdMs; }
    public int getMetricMaxCardinality() { return metricMaxCardinality; }
    public void setMetricMaxCardinality(int metricMaxCardinality) { this.metricMaxCardinality = metricMaxCardinality; }
    public int getApiRateLimitRps() { return apiRateLimitRps; }
    public void setApiRateLimitRps(int apiRateLimitRps) { this.apiRateLimitRps = apiRateLimitRps; }
    public boolean isMcpEnabled() { return mcpEnabled; }
    public void setMcpEnabled(boolean mcpEnabled) { this.mcpEnabled = mcpEnabled; }
    public String getMcpPath() { return mcpPath; }
    public void setMcpPath(String mcpPath) { this.mcpPath = mcpPath; }
    public int getVectorIndexMaxEntries() { return vectorIndexMaxEntries; }
    public void setVectorIndexMaxEntries(int vectorIndexMaxEntries) { this.vectorIndexMaxEntries = vectorIndexMaxEntries; }
    public String getDlqPath() { return dlqPath; }
    public void setDlqPath(String dlqPath) { this.dlqPath = dlqPath; }
    public int getDlqMaxFiles() { return dlqMaxFiles; }
    public void setDlqMaxFiles(int dlqMaxFiles) { this.dlqMaxFiles = dlqMaxFiles; }
    public int getDlqMaxDiskMb() { return dlqMaxDiskMb; }
    public void setDlqMaxDiskMb(int dlqMaxDiskMb) { this.dlqMaxDiskMb = dlqMaxDiskMb; }
    public int getDlqMaxRetries() { return dlqMaxRetries; }
    public void setDlqMaxRetries(int dlqMaxRetries) { this.dlqMaxRetries = dlqMaxRetries; }
    public String getDlqReplayInterval() { return dlqReplayInterval; }
    public void setDlqReplayInterval(String dlqReplayInterval) { this.dlqReplayInterval = dlqReplayInterval; }
    public int getTsdbWindowSeconds() { return tsdbWindowSeconds; }
    public void setTsdbWindowSeconds(int tsdbWindowSeconds) { this.tsdbWindowSeconds = tsdbWindowSeconds; }
    public int getTsdbRingSlots() { return tsdbRingSlots; }
    public void setTsdbRingSlots(int tsdbRingSlots) { this.tsdbRingSlots = tsdbRingSlots; }
}
