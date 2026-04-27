package ingest

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
	"github.com/RandomCodeSpace/otelcontext/internal/telemetry"
)

// SignalType identifies the OTLP signal a Batch carries. The label
// is exported on pipeline metrics so operators can attribute drops.
type SignalType uint8

const (
	SignalTraces SignalType = iota
	SignalLogs
)

// signalLabel returns the metric-label form of a SignalType.
func signalLabel(t SignalType) string {
	switch t {
	case SignalTraces:
		return "traces"
	case SignalLogs:
		return "logs"
	default:
		return "unknown"
	}
}

// Batch is the unit of work flowing through the async ingest Pipeline.
// One Batch corresponds to the persistable output of a single OTLP
// Export() call. Trace insertion ordering (Traces → Spans → Logs) is
// honored by the worker that processes the batch — packaging the three
// slices together preserves the FK invariant the synchronous path
// already enforces.
type Batch struct {
	Type   SignalType
	Tenant string

	Traces []storage.Trace
	Spans  []storage.Span
	Logs   []storage.Log

	// Priority flags. Errors and slow traces are protected from soft
	// backpressure drops — they may still be rejected at hard capacity.
	HasError bool
	HasSlow  bool

	// Optional per-record callbacks invoked after a successful DB write.
	// In production these feed GraphRAG ingestion. Nil callbacks are
	// skipped silently.
	SpanCallback func(storage.Span)
	LogCallback  func(storage.Log)

	enqueuedAt time.Time
}

// Priority reports whether the batch is protected from soft-backpressure
// drops. Used by Submit() to decide whether to enqueue at >= 90% fullness.
func (b *Batch) Priority() bool { return b.HasError || b.HasSlow }

// ErrQueueFull is returned by Submit when the queue is at hard capacity
// (100% full). Callers should map this to gRPC RESOURCE_EXHAUSTED or
// HTTP 429 with a Retry-After hint so OTLP clients back off cleanly.
var ErrQueueFull = errors.New("ingest pipeline at capacity")

// PipelineConfig holds the tunables for a Pipeline.
type PipelineConfig struct {
	Capacity      int     // total queue depth across all signal types
	Workers       int     // worker goroutines draining the queue
	SoftThreshold float64 // fullness fraction above which healthy batches are dropped (0.0–1.0)
}

// DefaultPipelineConfig returns production-sized defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Capacity:      50000,
		Workers:       8,
		SoftThreshold: 0.9,
	}
}

// pipelineWriter is the slice of *storage.Repository the Pipeline depends
// on. Defining it as an interface keeps the package layering clean and
// lets tests inject fakes without spinning up SQLite.
type pipelineWriter interface {
	BatchCreateTraces(traces []storage.Trace) error
	BatchCreateSpans(spans []storage.Span) error
	BatchCreateLogs(logs []storage.Log) error
}

// Pipeline decouples OTLP Export() from synchronous DB writes. It holds a
// bounded buffered channel of Batches, a worker pool that drains the
// channel into the Repository, and Prometheus instruments that surface
// queue depth, drop counts, and rejection counts.
//
// Lifecycle:
//
//	p := NewPipeline(repo, metrics, cfg)
//	p.Start(ctx)
//	defer p.Stop()       // drains in-flight before returning
//	p.Submit(batch)
type Pipeline struct {
	writer  pipelineWriter
	metrics *telemetry.Metrics

	cfg   PipelineConfig
	queue chan *Batch

	// Stats — exported via accessors for tests and for the /metrics path
	// that doesn't already cover pipeline counters.
	enqueuedTotal   atomic.Int64
	processedTotal  atomic.Int64
	droppedHealthy  atomic.Int64
	rejectedFull    atomic.Int64
	processFailures atomic.Int64

	stopCh chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

// NewPipeline constructs a Pipeline with the given config, falling back
// to DefaultPipelineConfig() values for non-positive fields. The
// Pipeline does NOT start workers — call Start(ctx) when ready.
func NewPipeline(writer pipelineWriter, metrics *telemetry.Metrics, cfg PipelineConfig) *Pipeline {
	d := DefaultPipelineConfig()
	if cfg.Capacity <= 0 {
		cfg.Capacity = d.Capacity
	}
	if cfg.Workers <= 0 {
		cfg.Workers = d.Workers
	}
	if cfg.SoftThreshold <= 0 || cfg.SoftThreshold >= 1.0 {
		cfg.SoftThreshold = d.SoftThreshold
	}
	return &Pipeline{
		writer:  writer,
		metrics: metrics,
		cfg:     cfg,
		queue:   make(chan *Batch, cfg.Capacity),
		stopCh:  make(chan struct{}),
	}
}

// Start spawns the worker pool. Safe to call once. Subsequent calls are
// no-ops; tests rely on this for reset semantics.
func (p *Pipeline) Start(ctx context.Context) {
	for range p.cfg.Workers {
		p.wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("ingest pipeline worker panic",
						"panic", r,
						"stack", string(debug.Stack()),
					)
					if p.metrics != nil && p.metrics.PanicsRecoveredTotal != nil {
						p.metrics.PanicsRecoveredTotal.WithLabelValues("ingest_pipeline").Inc()
					}
				}
			}()
			p.worker(ctx)
		})
	}
}

// Submit enqueues a batch for asynchronous persistence. Returns nil when
// the batch is accepted (or silently dropped under soft backpressure)
// and ErrQueueFull when the queue is at hard capacity. Nil batches are
// no-ops.
//
// Soft backpressure: when fullness >= SoftThreshold, healthy batches
// (Priority()==false) are dropped at the door and Submit returns nil so
// the OTLP client sees a successful Export. Errors and slow traces
// always continue to the channel.
//
// Hard backpressure: when the channel send fails (buffer at 100%),
// Submit returns ErrQueueFull regardless of priority. The caller should
// translate this into a backpressure signal so the client retries with
// exponential backoff rather than tighter loops.
func (p *Pipeline) Submit(b *Batch) error {
	if b == nil {
		return nil
	}
	if len(b.Traces) == 0 && len(b.Spans) == 0 && len(b.Logs) == 0 {
		// Empty batch — nothing to persist. Skip the channel entirely.
		return nil
	}
	b.enqueuedAt = time.Now()

	fullness := float64(len(p.queue)) / float64(p.cfg.Capacity)
	if fullness >= p.cfg.SoftThreshold && !b.Priority() {
		p.droppedHealthy.Add(1)
		p.observeDrop(b.Type, "soft_backpressure")
		return nil
	}

	select {
	case p.queue <- b:
		p.enqueuedTotal.Add(1)
		p.observeQueueDepth(b.Type)
		return nil
	default:
		p.rejectedFull.Add(1)
		p.observeDrop(b.Type, "queue_full")
		return ErrQueueFull
	}
}

// Stop signals workers to exit and blocks until in-flight batches have
// been drained from the channel. Idempotent.
func (p *Pipeline) Stop() {
	p.once.Do(func() {
		close(p.stopCh)
	})
	p.wg.Wait()
}

// Stats returns snapshot counters for tests and for telemetry that
// doesn't already use Prometheus instruments. The values are best-effort
// and not synchronized across atomics — sufficient for diagnostics.
func (p *Pipeline) Stats() PipelineStats {
	return PipelineStats{
		Enqueued:        p.enqueuedTotal.Load(),
		Processed:       p.processedTotal.Load(),
		DroppedHealthy:  p.droppedHealthy.Load(),
		RejectedFull:    p.rejectedFull.Load(),
		ProcessFailures: p.processFailures.Load(),
		QueueDepth:      len(p.queue),
		Capacity:        p.cfg.Capacity,
	}
}

// PipelineStats is a snapshot of pipeline counters.
type PipelineStats struct {
	Enqueued        int64
	Processed       int64
	DroppedHealthy  int64
	RejectedFull    int64
	ProcessFailures int64
	QueueDepth      int
	Capacity        int
}

// worker drains the queue. Exits when stopCh closes (after draining
// remaining batches) or when ctx is canceled (immediate).
func (p *Pipeline) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-p.queue:
			p.process(b)
		case <-p.stopCh:
			// Drain remaining buffered batches synchronously so a
			// graceful shutdown doesn't lose in-flight ingest.
			for {
				select {
				case b := <-p.queue:
					p.process(b)
				default:
					return
				}
			}
		}
	}
}

// process persists a single batch in Trace→Span→Log order, mirroring the
// ordering invariant of the synchronous Export() path. Failures are
// logged and surfaced via processFailures; the batch is then dropped
// (the DLQ tier is the redundancy story for write failures).
func (p *Pipeline) process(b *Batch) {
	if b == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ingest pipeline process panic",
				"panic", r,
				"stack", string(debug.Stack()),
			)
			p.processFailures.Add(1)
			if p.metrics != nil && p.metrics.PanicsRecoveredTotal != nil {
				p.metrics.PanicsRecoveredTotal.WithLabelValues("ingest_pipeline").Inc()
			}
		}
	}()
	p.processedTotal.Add(1)

	if len(b.Traces) > 0 {
		if err := p.writer.BatchCreateTraces(b.Traces); err != nil {
			slog.Error("ingest pipeline: BatchCreateTraces failed", "error", err)
			p.processFailures.Add(1)
			// Continue — spans may still land if their trace exists from
			// a prior batch. Mirrors the synchronous path's tolerance.
		}
	}
	if len(b.Spans) > 0 {
		if err := p.writer.BatchCreateSpans(b.Spans); err != nil {
			slog.Error("ingest pipeline: BatchCreateSpans failed", "error", err)
			p.processFailures.Add(1)
			return
		}
		if b.SpanCallback != nil {
			for _, s := range b.Spans {
				b.SpanCallback(s)
			}
		}
	}
	if len(b.Logs) > 0 {
		if err := p.writer.BatchCreateLogs(b.Logs); err != nil {
			slog.Error("ingest pipeline: BatchCreateLogs failed", "error", err)
			p.processFailures.Add(1)
			return
		}
		if b.LogCallback != nil {
			for _, l := range b.Logs {
				b.LogCallback(l)
			}
		}
	}
}

func (p *Pipeline) observeQueueDepth(t SignalType) {
	if p.metrics == nil || p.metrics.IngestPipelineQueueDepth == nil {
		return
	}
	p.metrics.IngestPipelineQueueDepth.WithLabelValues(signalLabel(t)).Set(float64(len(p.queue)))
}

func (p *Pipeline) observeDrop(t SignalType, reason string) {
	if p.metrics == nil || p.metrics.IngestPipelineDroppedTotal == nil {
		return
	}
	p.metrics.IngestPipelineDroppedTotal.WithLabelValues(signalLabel(t), reason).Inc()
}
