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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func initTracer() func(context.Context) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("inventory-service"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	)

	exporter, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tracerProvider.Shutdown
}

func main() {
	shutdown := initTracer()
	defer shutdown(context.Background())

	tracer = otel.Tracer("inventory-service")

	mux := http.NewServeMux()
	mux.Handle("/check", otelhttp.NewHandler(http.HandlerFunc(handleCheckInventory), "POST /check"))

	log.Println("📦 Inventory Service listening on :9003")
	log.Fatal(http.ListenAndServe(":9003", mux))
}

func handleCheckInventory(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "check_inventory")
	defer span.End()

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
