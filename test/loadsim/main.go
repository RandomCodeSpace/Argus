//go:build loadtest

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// operations is the fixed pool picked round-robin per producer.
var operations = []string{
	"GET /api/items",
	"POST /api/orders",
	"GET /health",
	"GET /api/users",
	"POST /api/payments",
}

// -------------------------------------------------------------------------
// Pure helper functions (tested directly by main_test.go)
// -------------------------------------------------------------------------

// serviceName returns the zero-padded service name for index i.
func serviceName(i int) string {
	return fmt.Sprintf("loadsim-svc-%03d", i)
}

// pickOperation returns an operation name using round-robin on the global ops slice.
func pickOperation(seq int) string {
	return operations[seq%len(operations)]
}

// randomDuration returns a uniformly random duration in [5ms, 500ms].
// Uses the shared global RNG; the hot-path variant is (*producer).randomDuration.
func randomDuration() time.Duration {
	// 5ms + [0, 495ms)
	return time.Duration(5+rand.Intn(496)) * time.Millisecond
}

// randomDuration returns a uniformly random duration in [5ms, 500ms] using the
// producer's private RNG (no cross-goroutine mutex contention).
func (p *producer) randomDuration() time.Duration {
	return time.Duration(5+p.rng.Intn(496)) * time.Millisecond
}

// isError returns true for approximately 5% of call sites (seq % 20 == 0).
// This is deterministic for a given seq, giving exactly 5% over a complete cycle.
func isError(seq int) bool {
	return seq%20 == 0
}

// -------------------------------------------------------------------------
// Ticker-based rate limiter (no golang.org/x/time/rate dependency)
// -------------------------------------------------------------------------

type rateLimiter struct {
	ticker *time.Ticker
	ch     chan struct{}
	done   chan struct{}
}

func newRateLimiter(rps int) *rateLimiter {
	interval := time.Second / time.Duration(rps)
	rl := &rateLimiter{
		ticker: time.NewTicker(interval),
		ch:     make(chan struct{}, 1), // capacity 1 avoids head-of-line blocking
		done:   make(chan struct{}),
	}
	go func() {
		for {
			select {
			case <-rl.ticker.C:
				select {
				case rl.ch <- struct{}{}:
				default: // drop tick if consumer is behind — no burst accumulation
				}
			case <-rl.done:
				return
			}
		}
	}()
	return rl
}

// wait blocks until one token is available.
func (rl *rateLimiter) wait() {
	<-rl.ch
}

func (rl *rateLimiter) stop() {
	rl.ticker.Stop()
	close(rl.done)
}

// -------------------------------------------------------------------------
// Per-producer state
// -------------------------------------------------------------------------

type producer struct {
	idx      int
	endpoint string
	tenantID string
	insecure bool

	tp     *sdktrace.TracerProvider
	tracer trace.Tracer

	// rng is a per-producer RNG — avoids 200-goroutine contention on the global
	// math/rand mutex in the hot path (duration, child count).
	rng *rand.Rand

	sentTotal  atomic.Int64
	errorTotal atomic.Int64
}

func newProducer(ctx context.Context, idx int, endpoint, tenantID string, insecure bool) (*producer, error) {
	svc := serviceName(idx)

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if tenantID != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{"x-tenant-id": tenantID}))
	}

	client := otlptracegrpc.NewClient(opts...)
	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("producer %d exporter: %w", idx, err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(svc)),
	)
	if err != nil {
		return nil, fmt.Errorf("producer %d resource: %w", idx, err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)

	return &producer{
		idx:      idx,
		endpoint: endpoint,
		tenantID: tenantID,
		insecure: insecure,
		tp:       tp,
		tracer:   tp.Tracer(svc),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano() + int64(idx))),
	}, nil
}

// run emits spans at the given rate for the given duration, then returns.
func (p *producer) run(ctx context.Context, rps int, dur time.Duration) {
	rl := newRateLimiter(rps)
	defer rl.stop()

	deadline := time.Now().Add(dur)
	seq := 0

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		rl.wait()
		p.emitSpan(ctx, seq)
		seq++
	}
}

// emitSpan creates one span (with optional child spans every 10th call).
func (p *producer) emitSpan(ctx context.Context, seq int) {
	op := pickOperation(seq)
	dur := p.randomDuration()
	errored := isError(seq)

	// Every 10th span: create a parent with 1–3 children in the same trace.
	if seq%10 == 0 {
		parentCtx, parentSpan := p.tracer.Start(ctx, op)
		if errored {
			parentSpan.SetStatus(codes.Error, "simulated error")
			parentSpan.RecordError(errors.New("fake failure"))
			p.errorTotal.Add(1)
		}

		numChildren := 1 + p.rng.Intn(3) // [1,3]
		for c := 0; c < numChildren; c++ {
			childOp := pickOperation(seq + c + 1)
			_, childSpan := p.tracer.Start(parentCtx, childOp)
			time.Sleep(dur / time.Duration(numChildren+1))
			childSpan.End()
			p.sentTotal.Add(1)
		}

		time.Sleep(dur / time.Duration(numChildren+1))
		parentSpan.End()
		p.sentTotal.Add(1)
	} else {
		_, span := p.tracer.Start(ctx, op)
		if errored {
			span.SetStatus(codes.Error, "simulated error")
			span.RecordError(errors.New("fake failure"))
			p.errorTotal.Add(1)
		}
		time.Sleep(dur)
		span.End()
		p.sentTotal.Add(1)
	}
}

// shutdown flushes the exporter and waits up to the given timeout.
func (p *producer) shutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := p.tp.Shutdown(ctx); err != nil {
		log.Printf("producer %d shutdown error: %v", p.idx, err)
	}
}

// -------------------------------------------------------------------------
// Coordinator
// -------------------------------------------------------------------------

type coordinator struct {
	startTime time.Time

	totalSent   atomic.Int64
	totalErrors atomic.Int64
}

func (c *coordinator) progressLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var prevSent int64
	prevTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			elapsed := now.Sub(c.startTime).Seconds()
			sent := c.totalSent.Load()
			errs := c.totalErrors.Load()
			delta := sent - prevSent
			dt := now.Sub(prevTime).Seconds()
			rate := float64(delta) / dt
			prevSent = sent
			prevTime = now
			fmt.Printf("[T+%3.0fs] sent=%d errors=%d rate=%.0f/s\n", elapsed, sent, errs, rate)
		}
	}
}

// -------------------------------------------------------------------------
// Main
// -------------------------------------------------------------------------

func main() {
	endpoint := flag.String("endpoint", "localhost:4317", "OTLP gRPC endpoint")
	numServices := flag.Int("services", 200, "Number of simulated services")
	rps := flag.Int("rate", 50, "Spans per second per service")
	duration := flag.Duration("duration", 60*time.Second, "Test duration")
	insecure := flag.Bool("insecure", true, "Use insecure gRPC connection")
	tenantID := flag.String("tenant-id", "", "x-tenant-id gRPC metadata value (empty = omit)")
	warmup := flag.Duration("warmup", 5*time.Second, "Stagger window for producer startup")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Suppress default OTel global TracerProvider noise.
	otel.SetTracerProvider(sdktrace.NewTracerProvider())

	fmt.Printf("Starting %d-service load simulator → %s\n", *numServices, *endpoint)
	fmt.Printf("Rate: %d span/s per service | Duration: %s | Warmup: %s\n", *rps, *duration, *warmup)
	fmt.Println("Press Ctrl+C to stop early.")

	coord := &coordinator{startTime: time.Now()}

	// Create all producers up front (no connections yet — lazy dial).
	producers := make([]*producer, *numServices)
	for i := 0; i < *numServices; i++ {
		p, err := newProducer(ctx, i, *endpoint, *tenantID, *insecure)
		if err != nil {
			log.Fatalf("Failed to create producer %d: %v", i, err)
		}
		producers[i] = p
	}

	// Stagger goroutine to roll out producers linearly over warmup window.
	staggerDelay := time.Duration(0)
	if *numServices > 1 {
		staggerDelay = *warmup / time.Duration(*numServices-1)
	}

	var wg sync.WaitGroup

	// Progress reporter (runs until ctx cancelled or all producers done).
	progressCtx, stopProgress := context.WithCancel(ctx)
	wg.Add(1)
	go func() {
		defer wg.Done()
		coord.progressLoop(progressCtx, 5*time.Second)
	}()

	// Aggregator: fold per-producer counters into coordinator totals.
	// We refresh once per second in a background goroutine.
	aggDone := make(chan struct{})
	go func() {
		defer close(aggDone)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var s, e int64
				for _, p := range producers {
					s += p.sentTotal.Load()
					e += p.errorTotal.Load()
				}
				coord.totalSent.Store(s)
				coord.totalErrors.Store(e)
			}
		}
	}()

	// Launch producers with stagger.
	producersDone := make(chan struct{})
	go func() {
		defer close(producersDone)
		var pwg sync.WaitGroup
	warmupLoop:
		for i, p := range producers {
			if i > 0 && staggerDelay > 0 {
				select {
				case <-ctx.Done():
					break warmupLoop
				case <-time.After(staggerDelay):
				}
			}
			pwg.Add(1)
			pp := p
			go func() {
				defer pwg.Done()
				pp.run(ctx, *rps, *duration)
			}()
		}
		pwg.Wait()
	}()

	// Wait for producers to finish or signal.
	select {
	case <-producersDone:
	case <-ctx.Done():
		fmt.Println("\nShutting down early (signal received)…")
	}

	// Stop aggregator and progress reporter.
	stop() // cancel signal context so agg loop exits
	stopProgress()
	wg.Wait()
	<-aggDone

	// Final aggregate.
	var totalSent, totalErrors int64
	for _, p := range producers {
		totalSent += p.sentTotal.Load()
		totalErrors += p.errorTotal.Load()
	}

	// Flush all exporters (up to 5s total).
	fmt.Printf("Flushing %d exporters…\n", len(producers))
	flushTimeout := 5 * time.Second / time.Duration(len(producers)+1)
	if flushTimeout < 100*time.Millisecond {
		flushTimeout = 100 * time.Millisecond
	}
	var shutWg sync.WaitGroup
	for _, p := range producers {
		shutWg.Add(1)
		pp := p
		go func() {
			defer shutWg.Done()
			pp.shutdown(flushTimeout)
		}()
	}
	shutWg.Wait()

	elapsed := time.Since(coord.startTime)
	successCount := totalSent - totalErrors

	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("Duration:        %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total spans:     %d\n", totalSent)
	fmt.Printf("Errors:          %d (%.1f%%)\n", totalErrors, 100*float64(totalErrors)/float64(totalSent+1))
	fmt.Printf("Success:         %d\n", successCount)
	fmt.Printf("Effective rate:  %.0f span/s\n", float64(totalSent)/elapsed.Seconds())
	fmt.Println("─────────────────────────────────────────")
}
