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
	tracer         trace.Tracer
	paymentCounter metric.Int64UpDownCounter
)

func initOTel() func(context.Context) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("payment-service"),
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

	meter := otel.Meter("payment-service")
	paymentCounter, _ = meter.Int64UpDownCounter("active_payments", metric.WithDescription("Current active payment requests"))

	return func(ctx context.Context) error {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil
	}
}

func main() {
	shutdown := initOTel()
	defer shutdown(context.Background())

	tracer = otel.Tracer("payment-service")

	mux := http.NewServeMux()
	mux.Handle("/pay", otelhttp.NewHandler(http.HandlerFunc(handlePay), "POST /pay"))

	log.Println("💳 Payment Service listening on :9002")
	log.Fatal(http.ListenAndServe(":9002", mux))
}

func handlePay(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "process_payment")
	defer span.End()

	if paymentCounter != nil {
		paymentCounter.Add(ctx, 1)
		defer paymentCounter.Add(ctx, -1)
	}

	span.SetAttributes(
		attribute.String("payment.method", "credit_card"),
		attribute.String("payment.provider", "stripe"),
	)
	span.AddEvent("payment_request_received", trace.WithAttributes(attribute.String("amount", "99.99")))

	// Chaos: 10% chance of Gateway Timeout (HTTP 500)
	if rand.Intn(100) < 10 {
		err := fmt.Errorf("Gateway Timeout: Upstream Payment Provider Unreachable")
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.type", "payment_gateway_timeout"))
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}

	// 1. Double check auth (Simulating multi-step validation)
	span.AddEvent("secondary_auth_check", trace.WithAttributes(attribute.String("upstream", "auth-service")))
	if err := callAuthService(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, "Auth Failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Call Inventory Service (Service C) to verify stock
	span.AddEvent("verifying_inventory")
	if err := callInventoryService(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "inventory check failed: "+err.Error())
		// Still process payment, but record the inventory error
		span.AddEvent("inventory_check_degraded", trace.WithAttributes(
			attribute.String("reason", err.Error()),
		))
	}

	time.Sleep(50 * time.Millisecond) // Simulating payment processing
	span.AddEvent("payment_approved")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Payment Processed"))
}

func callInventoryService(ctx context.Context) error {
	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:9003/check", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inventory service returned %d", resp.StatusCode)
	}
	return nil
}

func callAuthService(ctx context.Context) error {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:9004/validate", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth service returned %d", resp.StatusCode)
	}
	return nil
}
