package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer           trace.Tracer
	inventoryCounter metric.Int64Counter
)

func initOTel() func(context.Context) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("inventory-service"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	// 1. Tracing Setup
	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	)
	traceExporter, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExporter)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// 2. Metrics Setup
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		log.Fatalf("failed to create metric exporter: %v", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(5*time.Second))),
	)
	otel.SetMeterProvider(mp)

	meter := otel.Meter("inventory-service")
	inventoryCounter, _ = meter.Int64Counter("inventory_queries_total", metric.WithDescription("Total number of inventory checks"))

	return func(ctx context.Context) error {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil
	}
}

func main() {
	shutdown := initOTel()
	defer shutdown(context.Background())

	tracer = otel.Tracer("inventory-service")

	mux := http.NewServeMux()
	mux.Handle("/check", otelhttp.NewHandler(http.HandlerFunc(handleCheckInventory), "POST /check"))

	log.Println("📦 Inventory Service listening on :9003")
	log.Fatal(http.ListenAndServe(":9003", mux))
}

func handleCheckInventory(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "check_inventory")
	defer span.End()

	if inventoryCounter != nil {
		inventoryCounter.Add(ctx, 1)
	}

	sku := fmt.Sprintf("SKU-%d", rand.Intn(50000))
	span.SetAttributes(
		attribute.String("inventory.sku", sku),
		attribute.String("inventory.warehouse", "us-west-2"),
	)

	// Chaos: 5% chance of "Database Lock" — High Latency + Error
	if rand.Intn(100) < 5 {
		lockDuration := time.Duration(2000+rand.Intn(3000)) * time.Millisecond
		span.AddEvent("database_lock_contention", trace.WithAttributes(
			attribute.String("lock_duration", lockDuration.String()),
			attribute.String("table", "inventory_items"),
		))

		time.Sleep(lockDuration)

		err := fmt.Errorf("Database Lock Timeout: inventory_items table locked for %s", lockDuration)
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.type", "database_lock"))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Normal processing
	time.Sleep(20 * time.Millisecond) // Simulating DB query
	span.SetAttributes(attribute.Int("inventory.quantity_available", 10+rand.Intn(500)))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Inventory Available"))
}
