package com.otelcontext.config;

import com.otelcontext.graphrag.GraphRAGService;
import com.otelcontext.ingest.*;
import com.otelcontext.realtime.WebSocketHub;
import com.otelcontext.repository.LogRepository;
import com.otelcontext.repository.SpanRepository;
import com.otelcontext.repository.TraceRepository;
import com.otelcontext.tsdb.TsdbAggregator;
import com.otelcontext.vectordb.TfIdfIndex;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.protobuf.services.ProtoReflectionService;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.io.IOException;

@Configuration
public class GrpcServerConfig {

    private static final Logger log = LoggerFactory.getLogger(GrpcServerConfig.class);

    private final AppConfig appConfig;
    private Server grpcServer;

    public GrpcServerConfig(AppConfig appConfig) {
        this.appConfig = appConfig;
    }

    @Bean
    public Sampler sampler() {
        return new Sampler(appConfig.getSamplingRate(), appConfig.isSamplingAlwaysOnErrors(),
            appConfig.getSamplingLatencyThresholdMs());
    }

    @Bean
    public OtlpTraceService otlpTraceService(TraceRepository traceRepo, SpanRepository spanRepo,
                                              LogRepository logRepo, GraphRAGService graphRAG,
                                              WebSocketHub wsHub, TfIdfIndex vectorIndex, Sampler sampler) {
        return new OtlpTraceService(traceRepo, spanRepo, logRepo, graphRAG, wsHub, vectorIndex, sampler);
    }

    @Bean
    public OtlpLogService otlpLogService(LogRepository logRepo, GraphRAGService graphRAG,
                                          WebSocketHub wsHub, TfIdfIndex vectorIndex) {
        return new OtlpLogService(logRepo, graphRAG, wsHub, vectorIndex);
    }

    @Bean
    public OtlpMetricService otlpMetricService(TsdbAggregator tsdb, GraphRAGService graphRAG, WebSocketHub wsHub) {
        return new OtlpMetricService(tsdb, graphRAG, wsHub);
    }

    @PostConstruct
    public void startGrpcServer() throws IOException {
        // Defer to avoid circular dependency - actual start happens via ApplicationRunner
    }

    @Bean
    public org.springframework.boot.ApplicationRunner grpcStarter(OtlpTraceService traceService,
                                                                   OtlpLogService logService,
                                                                   OtlpMetricService metricService) {
        return args -> {
            grpcServer = ServerBuilder.forPort(appConfig.getGrpcPort())
                .addService(traceService)
                .addService(logService)
                .addService(metricService)
                .addService(ProtoReflectionService.newInstance())
                .build()
                .start();
            log.info("gRPC OTLP server started on port {}", appConfig.getGrpcPort());
        };
    }

    @PreDestroy
    public void stopGrpcServer() {
        if (grpcServer != null) {
            grpcServer.shutdown();
            log.info("gRPC server stopped");
        }
    }
}
